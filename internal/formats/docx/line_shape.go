package docx

import (
	"fmt"
	"strconv"
)

type LineSizeEdit struct {
	WidthEMU  int `json:"widthEmu" jsonschema:"Horizontal span in EMUs, from 12700 through 18288000"`
	HeightEMU int `json:"heightEmu" jsonschema:"Vertical span in EMUs, from 12700 through 18288000"`
}

type LinePreview struct {
	LineSizeEdit
	Stroke string `json:"stroke"`
}

func (size LineSizeEdit) valid() bool {
	return size.WidthEMU >= 12700 && size.WidthEMU <= maxImageExtent &&
		size.HeightEMU >= 12700 && size.HeightEMU <= maxImageExtent
}

func inlineLine(id uint32, size LineSizeEdit) string {
	return inlineWordShape(id, "Line", size.WidthEMU, size.HeightEMU,
		`<a:prstGeom prst="line"><a:avLst/></a:prstGeom>
<a:noFill/><a:ln w="12700"><a:solidFill><a:srgbClr val="0F766E"/></a:solidFill></a:ln>`)
}

func inspectInlineLine(inline, graphic *node) *LinePreview {
	if inline == nil || inline.name.Space != drawingWordNS || inline.name.Local != "inline" {
		return nil
	}
	shape := singlePayload(graphic)
	if shape == nil || shape.name.Space != shapeWordNS || shape.name.Local != "wsp" || len(shape.children) != 3 {
		return nil
	}
	nonvisual := childNS(shape, shapeWordNS, "cNvSpPr")
	body := childNS(shape, shapeWordNS, "bodyPr")
	properties := childNS(shape, shapeWordNS, "spPr")
	if nonvisual == nil || len(nonvisual.children) != 0 || !onlyNamespaceAttributes(nonvisual) ||
		body == nil || len(body.children) != 0 || properties == nil || len(properties.children) != 4 || !onlyNamespaceAttributes(properties) {
		return nil
	}
	transform := childNS(properties, drawingNS, "xfrm")
	geometry := childNS(properties, drawingNS, "prstGeom")
	emptyFill := childNS(properties, drawingNS, "noFill")
	line := childNS(properties, drawingNS, "ln")
	if transform == nil || len(transform.children) != 2 || !onlyNamespaceAttributes(transform) ||
		attribute(geometry, "", "prst") != "line" || emptyFill == nil || len(emptyFill.children) != 0 ||
		attribute(line, "", "w") != "12700" {
		return nil
	}
	for _, attr := range line.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != "" || attr.Name.Local != "w") {
			return nil
		}
	}
	adjustments := singlePayload(geometry)
	offset := childNS(transform, drawingNS, "off")
	if adjustments == nil || adjustments.name.Space != drawingNS || adjustments.name.Local != "avLst" ||
		len(adjustments.children) != 0 || attribute(offset, "", "x") != "0" || attribute(offset, "", "y") != "0" {
		return nil
	}
	inner := childNS(transform, drawingNS, "ext")
	outer := childNS(inline, drawingWordNS, "extent")
	width := previewNumber(attribute(outer, "", "cx"), 12700, maxImageExtent)
	height := previewNumber(attribute(outer, "", "cy"), 12700, maxImageExtent)
	if width == nil || height == nil ||
		attribute(inner, "", "cx") != strconv.Itoa(*width) || attribute(inner, "", "cy") != strconv.Itoa(*height) {
		return nil
	}
	fill := singlePayload(line)
	if fill == nil || fill.name.Space != drawingNS || fill.name.Local != "solidFill" {
		return nil
	}
	stroke := solidShapeColor(fill)
	if stroke == nil {
		return nil
	}
	return &LinePreview{LineSizeEdit: LineSizeEdit{WidthEMU: *width, HeightEMU: *height}, Stroke: attribute(stroke, "", "val")}
}

func inlineLineSizeNodes(run *node) (*LinePreview, *node, *node) {
	inline := childNS(run.child("drawing"), drawingWordNS, "inline")
	graphic := childNS(childNS(inline, drawingNS, "graphic"), drawingNS, "graphicData")
	preview := inspectInlineLine(inline, graphic)
	if preview == nil {
		return nil, nil, nil
	}
	shape := singlePayload(graphic)
	transform := childNS(childNS(shape, shapeWordNS, "spPr"), drawingNS, "xfrm")
	return preview, childNS(inline, drawingWordNS, "extent"), childNS(transform, drawingNS, "ext")
}

func resizeInlineLine(data []byte, t target, edit LineSizeEdit) ([]replacement, error) {
	preview, outer, inner := inlineLineSizeNodes(t.run)
	if preview == nil || !canResizeShape(data, outer, inner) || !edit.valid() || edit == preview.LineSizeEdit {
		return nil, fmt.Errorf("invalid or unsupported DOCX line size")
	}
	return resizeShape(data, outer, inner, edit.WidthEMU, edit.HeightEMU)
}
