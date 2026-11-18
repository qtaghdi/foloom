package docx

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	drawingWordNS      = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	drawingNS          = "http://schemas.openxmlformats.org/drawingml/2006/main"
	pictureNS          = "http://schemas.openxmlformats.org/drawingml/2006/picture"
	maxImageWriteBytes = 512 << 10
	maxImageReadBytes  = 4 << 20
	maxImagePixels     = 12_000_000
	maxImageExtent     = 18_288_000
)

type ImageEdit struct {
	Rotation *int       `json:"rotation,omitempty" jsonschema:"Degrees clockwise. Supported values are 0, 90, 180, and 270"`
	Crop     *ImageCrop `json:"crop,omitempty"`
	Action   string     `json:"action" jsonschema:"insert, replace, set_alt, delete, resize, crop, or rotate"`
	Base64   string     `json:"base64,omitempty" jsonschema:"PNG or JPEG data without a data URI prefix, up to 512 KiB decoded"`
	Alt      string     `json:"alt,omitempty"`
	WidthEMU int        `json:"widthEmu,omitempty"`
}

func imageConfig(raw []byte, byteLimit int) (image.Config, string, error) {
	if len(raw) == 0 || len(raw) > byteLimit {
		return image.Config{}, "", fmt.Errorf("DOCX image exceeds the byte limit")
	}
	config, kind, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (kind != "png" && kind != "jpeg") || config.Width < 1 || config.Height < 1 ||
		config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return image.Config{}, "", fmt.Errorf("DOCX image format or pixel dimensions are unsupported")
	}
	return config, kind, nil
}

func (p *documentPackage) availableDrawingID() (uint32, error) {
	var highest uint64
	var walk func(*node)
	walk = func(n *node) {
		if n.name.Space == drawingWordNS && n.name.Local == "docPr" {
			id, err := strconv.ParseUint(attribute(n, "", "id"), 10, 32)
			if err == nil && id > highest {
				highest = id
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	// Restrict the parse to XML parts that contain a possible document property.
	for _, part := range p.parts {
		if !strings.HasSuffix(strings.ToLower(part.header.Name), ".xml") || !bytes.Contains(part.data, []byte("docPr")) {
			continue
		}
		root, err := parseXML(part.data)
		if err != nil {
			return 0, err
		}
		walk(root)
	}
	if highest >= 1<<32-1 {
		return 0, fmt.Errorf("DOCX drawing IDs exhausted")
	}
	return uint32(highest + 1), nil
}

func (p *documentPackage) imageInsertion(data []byte, target target, edit ImageEdit, drawingID uint32, changes map[string][]byte) (replacement, error) {
	paragraph := target.paragraph
	if edit.Action != "insert" || edit.Rotation != nil || edit.Crop != nil || target.kind != "replace_paragraphs" || paragraph == nil ||
		len(edit.Alt) > 1024 || !utf8.ValidString(edit.Alt) || !validInlineText(edit.Alt) {
		return replacement{}, fmt.Errorf("invalid DOCX picture insertion")
	}
	raw, config, kind, err := decodePicture(edit.Base64)
	if err != nil {
		return replacement{}, err
	}
	width := edit.WidthEMU
	if width == 0 {
		width = min(config.Width*9525, 5_486_400)
	}
	if width < 9525 || width > maxImageExtent {
		return replacement{}, fmt.Errorf("invalid DOCX picture width")
	}
	height := int(int64(width) * int64(config.Height) / int64(config.Width))
	if height < 1 || height > maxImageExtent {
		return replacement{}, fmt.Errorf("invalid DOCX picture height")
	}
	extension := "." + kind
	filename := p.availablePart(changes, "word/media/foloom-image", extension)
	id := strconv.FormatUint(uint64(drawingID), 10)
	rel, err := p.addRelatedPart(changes, filename, "image", "image/"+kind, "rIdFoloomImage"+id)
	if err != nil {
		return replacement{}, err
	}
	changes[filename] = raw
	drawing := inlinePicture(id, rel, edit.Alt, width, height)
	return appendDrawingRun(data, paragraph, drawing), nil
}

func appendDrawingRun(data []byte, paragraph *node, drawing string) replacement {
	if strings.HasSuffix(string(data[paragraph.start:paragraph.inner]), "/>") {
		open, close := scopedOpening(paragraph, data, false)
		return replacement{paragraph.start, paragraph.end, []byte(open + drawing + close)}
	}
	return replacement{paragraph.endStart, paragraph.endStart, []byte(drawing)}
}

func decodePicture(encoded string) ([]byte, image.Config, string, error) {
	if len(encoded) > base64.StdEncoding.EncodedLen(maxImageWriteBytes) {
		return nil, image.Config{}, "", fmt.Errorf("DOCX picture exceeds the encoded size limit")
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return nil, image.Config{}, "", fmt.Errorf("invalid DOCX picture data")
	}
	config, kind, err := imageConfig(raw, maxImageWriteBytes)
	if err != nil {
		return nil, image.Config{}, "", err
	}
	// Reject truncated uploads before allocating a package relationship.
	if _, _, err = image.Decode(bytes.NewReader(raw)); err != nil {
		return nil, image.Config{}, "", fmt.Errorf("invalid DOCX picture payload")
	}
	return raw, config, kind, nil
}

func (p *documentPackage) pictureUpdateReplacement(t target, edit ImageEdit, changes map[string][]byte) (replacement, error) {
	if t.kind != "picture" || t.run == nil || edit.Action != "replace" && edit.Base64 != "" || edit.Action != "resize" && edit.WidthEMU != 0 ||
		edit.Action != "crop" && edit.Crop != nil || edit.Action != "rotate" && edit.Rotation != nil ||
		len(edit.Alt) > 1024 || !utf8.ValidString(edit.Alt) || !validInlineText(edit.Alt) {
		return replacement{}, fmt.Errorf("invalid DOCX picture update")
	}
	for parent := t.run.parent; parent != nil; parent = parent.parent {
		if parent.is("ins") || parent.is("del") || parent.is("moveFrom") || parent.is("moveTo") {
			return replacement{}, fmt.Errorf("tracked DOCX pictures are protected")
		}
	}
	info, relationship := inlinePictureInfo(t.run)
	inline := childNS(t.run.child("drawing"), drawingWordNS, "inline")
	if info == nil || inline == nil {
		return replacement{}, fmt.Errorf("DOCX picture description requires a supported inline picture")
	}
	var props *node
	for _, child := range inline.children {
		if child.name.Space == drawingWordNS && child.name.Local == "docPr" {
			if props != nil {
				return replacement{}, fmt.Errorf("ambiguous DOCX picture properties")
			}
			props = child
		}
	}
	if props == nil {
		return replacement{}, fmt.Errorf("DOCX picture properties are unavailable")
	}
	part := p.picturePartMap()[relationship]
	if _, _, err := imageConfig(p.partData(part, nil), maxImageReadBytes); err != nil {
		return replacement{}, err
	}
	if edit.Action == "rotate" {
		if edit.Alt != "" || edit.Rotation == nil {
			return replacement{}, fmt.Errorf("invalid DOCX picture rotation")
		}
		return rotateInlinePicture(p.document, inline, *edit.Rotation)
	}
	if edit.Action == "crop" {
		if edit.Alt != "" {
			return replacement{}, fmt.Errorf("unexpected DOCX picture description")
		}
		return cropInlinePicture(p.document, inline, edit.Crop)
	}
	if edit.Action == "replace" {
		graphic := childNS(childNS(inline, drawingNS, "graphic"), drawingNS, "graphicData")
		picture := childNS(graphic, pictureNS, "pic")
		fill := childNS(picture, pictureNS, "blipFill")
		blip := childNS(fill, drawingNS, "blip")
		if edit.Alt != "" || childNS(fill, drawingNS, "srcRect") != nil ||
			blip == nil || strings.Contains(string(p.document[blip.start:blip.end]), "<!--") {
			return replacement{}, fmt.Errorf("protected DOCX picture cannot be replaced")
		}
		data, _, kind, err := decodePicture(edit.Base64)
		if err != nil {
			return replacement{}, err
		}
		filename := p.availablePart(changes, "word/media/foloom-image", "."+kind)
		// Existing drawings can share this relationship, so replace only this blip.
		relation, err := p.addRelatedPart(changes, filename, "image", "image/"+kind, "rIdFoloomReplacement")
		if err != nil {
			return replacement{}, err
		}
		raw, err := patchScalarProperty(blip, p.document, "blip", officeRelNS, map[string]string{"embed": relation}, nil)
		if err != nil {
			return replacement{}, err
		}
		changes[filename] = data
		return replacement{blip.start, blip.end, []byte(raw)}, nil
	}
	if edit.Action == "resize" {
		if edit.Alt != "" {
			return replacement{}, fmt.Errorf("unexpected DOCX picture description")
		}
		return resizeInlinePicture(p.document, inline, info, edit.WidthEMU)
	}
	if edit.Action == "delete" {
		if edit.Alt != "" || !t.run.parent.is("p") ||
			strings.Contains(string(p.document[t.run.start:t.run.end]), "<!--") {
			return replacement{}, fmt.Errorf("protected DOCX picture cannot be deleted")
		}
		// Keep package relationships and media because another drawing can share them.
		return replacement{start: t.run.start, end: t.run.end}, nil
	}
	raw, err := patchScalarProperty(props, p.document, "docPr", "", map[string]string{"descr": edit.Alt}, nil)
	if err != nil {
		return replacement{}, err
	}
	return replacement{props.start, props.end, []byte(raw)}, nil
}

func resizeInlinePicture(data []byte, inline *node, info *PictureInfo, width int) (replacement, error) {
	if width < 9525 || width > maxImageExtent {
		return replacement{}, fmt.Errorf("invalid DOCX picture width")
	}
	height := int(int64(width) * int64(info.HeightEMU) / int64(info.WidthEMU))
	if height < 1 || height > maxImageExtent {
		return replacement{}, fmt.Errorf("invalid DOCX picture height")
	}
	extent := childNS(inline, drawingWordNS, "extent")
	graphic := childNS(childNS(inline, drawingNS, "graphic"), drawingNS, "graphicData")
	picture := childNS(graphic, pictureNS, "pic")
	transform := childNS(childNS(picture, pictureNS, "spPr"), drawingNS, "xfrm")
	innerExtent := childNS(transform, drawingNS, "ext")
	if extent == nil || innerExtent == nil || extent.end > innerExtent.start ||
		attribute(innerExtent, "", "cx") != strconv.Itoa(info.WidthEMU) || attribute(innerExtent, "", "cy") != strconv.Itoa(info.HeightEMU) {
		return replacement{}, fmt.Errorf("DOCX picture resize requires matching inline and transform sizes")
	}
	dimensions := map[string]string{"cx": strconv.Itoa(width), "cy": strconv.Itoa(height)}
	outer, err := patchScalarProperty(extent, data, "extent", "", dimensions, nil)
	if err != nil {
		return replacement{}, err
	}
	inner, err := patchScalarProperty(innerExtent, data, "ext", "", dimensions, nil)
	if err != nil {
		return replacement{}, err
	}
	raw := outer + string(data[extent.end:innerExtent.start]) + inner
	return replacement{start: extent.start, end: innerExtent.end, data: []byte(raw)}, nil
}

func inlinePicture(id, relationship, alt string, width, height int) string {
	cx, cy := strconv.Itoa(width), strconv.Itoa(height)
	return `<di:r xmlns:di="` + wordNS + `" xmlns:wp="` + drawingWordNS + `" xmlns:a="` + drawingNS + `" xmlns:pic="` + pictureNS + `" xmlns:r="` + officeRelNS + `"><di:drawing><wp:inline><wp:extent cx="` + cx + `" cy="` + cy + `"/><wp:docPr id="` + id + `" name="Picture ` + id + `" descr="` + escapeText(alt) + `"/><wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr><a:graphic><a:graphicData uri="` + pictureNS + `"><pic:pic><pic:nvPicPr><pic:cNvPr id="0" name="Picture ` + id + `"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="` + relationship + `"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + cx + `" cy="` + cy + `"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></di:drawing></di:r>`
}

type PictureInfo struct {
	Rotation         int        `json:"rotation,omitempty"`
	RotationEditable bool       `json:"rotationEditable,omitempty"`
	Crop             *ImageCrop `json:"crop,omitempty"`
	CropEditable     bool       `json:"cropEditable,omitempty"`
	WidthEMU         int        `json:"widthEmu"`
	HeightEMU        int        `json:"heightEmu"`
	Alt              string     `json:"alt"`
	Preview          bool       `json:"preview"`
}

type pictureAsset struct {
	data []byte
	mime string
}

func childNS(parent *node, namespace, local string) *node {
	if parent != nil {
		for _, child := range parent.children {
			if child.name.Space == namespace && child.name.Local == local {
				return child
			}
		}
	}
	return nil
}

func inlinePictureInfo(run *node) (*PictureInfo, string) {
	if sectionChildCount(run, "drawing") != 1 || len(run.children) > 2 || len(run.children) == 2 && run.child("rPr") == nil {
		return nil, ""
	}
	inline := childNS(run.child("drawing"), drawingWordNS, "inline")
	extent := childNS(inline, drawingWordNS, "extent")
	width, height := previewNumber(attribute(extent, "", "cx"), 1, maxImageExtent), previewNumber(attribute(extent, "", "cy"), 1, maxImageExtent)
	if width == nil || height == nil {
		return nil, ""
	}
	graphic := childNS(childNS(inline, drawingNS, "graphic"), drawingNS, "graphicData")
	pic := childNS(graphic, pictureNS, "pic")
	blip := childNS(childNS(pic, pictureNS, "blipFill"), drawingNS, "blip")
	relationship := attribute(blip, officeRelNS, "embed")
	if relationship == "" || attribute(blip, officeRelNS, "link") != "" {
		return nil, ""
	}
	alt := attribute(childNS(inline, drawingWordNS, "docPr"), "", "descr")
	if len(alt) > 1024 {
		alt = alt[:1024]
		for !utf8.ValidString(alt) {
			alt = alt[:len(alt)-1]
		}
	}
	return &PictureInfo{WidthEMU: *width, HeightEMU: *height, Alt: alt}, relationship
}

func (p *documentPackage) picturePartMap() map[string]string {
	out := make(map[string]string)
	root, err := parseXML(p.partData(documentRelationships, nil))
	if err != nil {
		return out
	}
	for _, rel := range root.children {
		if rel.name.Space != packageRelNS || rel.name.Local != "Relationship" ||
			attribute(rel, "", "Type") != officeRelNS+"/image" || attribute(rel, "", "TargetMode") == "External" {
			continue
		}
		target := attribute(rel, "", "Target")
		if strings.HasPrefix(target, "/") {
			target = strings.TrimPrefix(path.Clean(target), "/")
		} else {
			target = path.Clean(path.Join("word", target))
		}
		out[attribute(rel, "", "Id")] = target
	}
	return out
}

func (s *Session) addPicturePreviews(blocks []Block, targets map[string]target) {
	s.pictureAssets = make(map[string]pictureAsset)
	assets := make(map[string]pictureAsset)
	var relations map[string]string
	project := func(paragraph *Paragraph) {
		for index := range paragraph.Runs {
			run := &paragraph.Runs[index]
			source, ok := targets[run.ID]
			if !ok || source.kind != "picture" {
				continue
			}
			info, relationship := inlinePictureInfo(source.run)
			if info == nil {
				continue
			}
			run.Picture = info
			if relations == nil {
				relations = s.pkg.picturePartMap()
			}
			part := relations[relationship]
			asset, cached := assets[part]
			if !cached {
				raw := s.pkg.partData(part, nil)
				if _, kind, err := imageConfig(raw, maxImageReadBytes); err == nil {
					asset = pictureAsset{raw, "image/" + kind}
				}
				assets[part] = asset
			}
			if asset.data != nil {
				info.Preview = true
				inline := childNS(source.run.child("drawing"), drawingWordNS, "inline")
				_, info.Rotation, info.RotationEditable = supportedPictureRotation(inline, s.pkg.document)
				if crop, _, _ := supportedPictureCrop(inline, s.pkg.document); crop != nil {
					info.CropEditable = true
					if *crop != (ImageCrop{}) {
						info.Crop = crop
					}
				}
				s.pictureAssets[run.ID] = asset
			}
		}
	}
	for index := range blocks {
		block := &blocks[index]
		if block.Paragraph != nil {
			project(block.Paragraph)
		}
		for i := range block.Rows {
			for j := range block.Rows[i].Cells {
				for k := range block.Rows[i].Cells[j].Paragraphs {
					project(&block.Rows[i].Cells[j].Paragraphs[k])
				}
			}
		}
	}
}
