package docx

import (
	"fmt"
	"strconv"
	"strings"
)

const cropWhole = 100_000

type ImageCrop struct {
	Left   int `json:"left" jsonschema:"Left inset in thousandths of a percent"`
	Top    int `json:"top" jsonschema:"Top inset in thousandths of a percent"`
	Right  int `json:"right" jsonschema:"Right inset in thousandths of a percent"`
	Bottom int `json:"bottom" jsonschema:"Bottom inset in thousandths of a percent"`
}

func (crop ImageCrop) valid() bool {
	for _, value := range []int{crop.Left, crop.Top, crop.Right, crop.Bottom} {
		if value < 0 || value >= cropWhole {
			return false
		}
	}
	return crop.Left+crop.Right <= cropWhole-1000 && crop.Top+crop.Bottom <= cropWhole-1000
}

func supportedPictureCrop(inline *node, data []byte) (*ImageCrop, *node, *node) {
	graphic := childNS(childNS(inline, drawingNS, "graphic"), drawingNS, "graphicData")
	picture := childNS(graphic, pictureNS, "pic")
	fill := childNS(picture, pictureNS, "blipFill")
	blip := childNS(fill, drawingNS, "blip")
	stretch := childNS(fill, drawingNS, "stretch")
	stretchRect := childNS(stretch, drawingNS, "fillRect")
	source := childNS(fill, drawingNS, "srcRect")
	shape := childNS(picture, pictureNS, "spPr")
	transform := childNS(shape, drawingNS, "xfrm")
	geometry := childNS(shape, drawingNS, "prstGeom")
	if blip == nil || len(blip.children) != 0 || stretchRect == nil || len(stretch.children) != 1 ||
		len(stretchRect.children) != 0 || !onlyNamespaceAttributes(stretchRect) || transform == nil || !onlyNamespaceAttributes(transform) ||
		len(shape.children) != 2 || attribute(geometry, "", "prst") != "rect" {
		return nil, nil, nil
	}
	if source == nil {
		if len(fill.children) != 2 || fill.children[0] != blip || fill.children[1] != stretch {
			return nil, nil, nil
		}
		return &ImageCrop{}, nil, stretch
	}
	if len(fill.children) != 3 || fill.children[0] != blip || fill.children[1] != source || fill.children[2] != stretch ||
		len(source.children) != 0 || strings.Contains(string(data[source.start:source.end]), "<!--") ||
		strings.TrimSpace(source.text.String()) != "" {
		return nil, nil, nil
	}
	crop := ImageCrop{}
	fields := map[string]*int{"l": &crop.Left, "t": &crop.Top, "r": &crop.Right, "b": &crop.Bottom}
	for _, attr := range source.attrs {
		if isNamespaceAttribute(attr) {
			continue
		}
		field := fields[attr.Name.Local]
		value, err := strconv.Atoi(attr.Value)
		if attr.Name.Space != "" || field == nil || err != nil {
			return nil, nil, nil
		}
		*field = value
	}
	if !crop.valid() {
		return nil, nil, nil
	}
	return &crop, source, stretch
}

func cropInlinePicture(data []byte, inline *node, crop *ImageCrop) (replacement, error) {
	if crop == nil || !crop.valid() {
		return replacement{}, fmt.Errorf("invalid DOCX picture crop")
	}
	available, source, stretch := supportedPictureCrop(inline, data)
	if available == nil {
		return replacement{}, fmt.Errorf("protected DOCX picture fill")
	}
	if *crop == (ImageCrop{}) {
		if source == nil {
			return replacement{}, fmt.Errorf("DOCX picture crop already clear")
		}
		return replacement{start: source.start, end: source.end}, nil
	}
	values := map[string]string{"l": strconv.Itoa(crop.Left), "t": strconv.Itoa(crop.Top),
		"r": strconv.Itoa(crop.Right), "b": strconv.Itoa(crop.Bottom)}
	if source == nil {
		raw := `<dwic:srcRect xmlns:dwic="` + drawingNS + `" l="` + values["l"] + `" t="` + values["t"] +
			`" r="` + values["r"] + `" b="` + values["b"] + `"/>`
		return replacement{stretch.start, stretch.start, []byte(raw)}, nil
	}
	raw, err := patchScalarProperty(source, data, "srcRect", "", values, nil)
	if err != nil {
		return replacement{}, err
	}
	return replacement{source.start, source.end, []byte(raw)}, nil
}
