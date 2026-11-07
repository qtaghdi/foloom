package docx

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
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
	Action   string `json:"action" jsonschema:"insert"`
	Base64   string `json:"base64" jsonschema:"PNG or JPEG data without a data URI prefix, up to 512 KiB decoded"`
	Alt      string `json:"alt,omitempty"`
	WidthEMU int    `json:"widthEmu,omitempty"`
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
	if edit.Action != "insert" || target.kind != "replace_paragraphs" || paragraph == nil ||
		len(edit.Alt) > 1024 || !utf8.ValidString(edit.Alt) || !validInlineText(edit.Alt) ||
		len(edit.Base64) > base64.StdEncoding.EncodedLen(maxImageWriteBytes) {
		return replacement{}, fmt.Errorf("invalid DOCX picture insertion")
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(edit.Base64)
	if err != nil {
		return replacement{}, fmt.Errorf("invalid DOCX picture data")
	}
	config, kind, err := imageConfig(raw, maxImageWriteBytes)
	if err != nil {
		return replacement{}, err
	}
	// Reject truncated uploads before allocating a package relationship.
	if _, _, err = image.Decode(bytes.NewReader(raw)); err != nil {
		return replacement{}, fmt.Errorf("invalid DOCX picture payload")
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
	if strings.HasSuffix(string(data[paragraph.start:paragraph.inner]), "/>") {
		open, close := scopedOpening(paragraph, data, false)
		return replacement{paragraph.start, paragraph.end, []byte(open + drawing + close)}, nil
	}
	return replacement{paragraph.endStart, paragraph.endStart, []byte(drawing)}, nil
}

func inlinePicture(id, relationship, alt string, width, height int) string {
	cx, cy := strconv.Itoa(width), strconv.Itoa(height)
	return `<di:r xmlns:di="` + wordNS + `" xmlns:wp="` + drawingWordNS + `" xmlns:a="` + drawingNS + `" xmlns:pic="` + pictureNS + `" xmlns:r="` + officeRelNS + `"><di:drawing><wp:inline><wp:extent cx="` + cx + `" cy="` + cy + `"/><wp:docPr id="` + id + `" name="Picture ` + id + `" descr="` + escapeText(alt) + `"/><wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr><a:graphic><a:graphicData uri="` + pictureNS + `"><pic:pic><pic:nvPicPr><pic:cNvPr id="0" name="Picture ` + id + `"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="` + relationship + `"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + cx + `" cy="` + cy + `"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></di:drawing></di:r>`
}
