package docx

import (
	"fmt"
	"strconv"
	"strings"
)

type RectanglePreview struct {
	WidthEMU  int    `json:"widthEmu"`
	HeightEMU int    `json:"heightEmu"`
	Fill      string `json:"fill"`
	Stroke    string `json:"stroke"`
}

type RectangleColorEdit struct {
	Fill   string `json:"fill" jsonschema:"Six hexadecimal RGB digits for the solid fill, without a hash prefix"`
	Stroke string `json:"stroke" jsonschema:"Six hexadecimal RGB digits for the solid outline, without a hash prefix"`
}

type RectangleSizeEdit struct {
	WidthEMU  int `json:"widthEmu" jsonschema:"Rectangle width in EMUs, from 12700 through 18288000"`
	HeightEMU int `json:"heightEmu" jsonschema:"Rectangle height in EMUs, from 12700 through 18288000"`
}

type inlineRectangleInfo struct {
	RectanglePreview
	fillNode, strokeNode     *node
	frameSize, transformSize *node
}

func validRectangleColor(rgb string) bool {
	if len(rgb) != 6 {
		return false
	}
	for _, digit := range rgb {
		if !(digit >= '0' && digit <= '9' || digit >= 'a' && digit <= 'f' || digit >= 'A' && digit <= 'F') {
			return false
		}
	}
	return true
}

func solidShapeColor(parent *node) *node {
	color := singlePayload(parent)
	if color == nil || color.name.Space != drawingNS || color.name.Local != "srgbClr" || len(color.children) != 0 ||
		!validRectangleColor(attribute(color, "", "val")) {
		return nil
	}
	return color
}

func inspectInlineRectangle(inline, graphic *node) *inlineRectangleInfo {
	if inline == nil || inline.name.Local != "inline" {
		return nil
	}
	shape := singlePayload(graphic)
	if shape == nil || shape.name.Space != shapeWordNS || shape.name.Local != "wsp" || len(shape.children) != 3 {
		return nil
	}
	nonvisual := childNS(shape, shapeWordNS, "cNvSpPr")
	body := childNS(shape, shapeWordNS, "bodyPr")
	properties := childNS(shape, shapeWordNS, "spPr")
	if nonvisual == nil || len(nonvisual.children) != 0 || !onlyNamespaceAttributes(nonvisual) || body == nil || properties == nil || len(properties.children) != 4 || !onlyNamespaceAttributes(properties) {
		return nil
	}
	transform := childNS(properties, drawingNS, "xfrm")
	geometry := childNS(properties, drawingNS, "prstGeom")
	line := childNS(properties, drawingNS, "ln")
	if transform == nil || len(transform.children) != 2 || !onlyNamespaceAttributes(transform) ||
		attribute(geometry, "", "prst") != "rect" || attribute(line, "", "w") != "12700" ||
		attribute(line, "", "cmpd") != "" {
		return nil
	}
	adjustments := singlePayload(geometry)
	offset := childNS(transform, drawingNS, "off")
	if adjustments == nil || adjustments.name.Space != drawingNS || adjustments.name.Local != "avLst" ||
		len(adjustments.children) != 0 || attribute(offset, "", "x") != "0" || attribute(offset, "", "y") != "0" {
		return nil
	}
	inner := childNS(transform, drawingNS, "ext")
	outer := childNS(inline, drawingWordNS, "extent")
	width := previewNumber(attribute(outer, "", "cx"), 1, maxImageExtent)
	height := previewNumber(attribute(outer, "", "cy"), 1, maxImageExtent)
	if width == nil || height == nil ||
		attribute(inner, "", "cx") != strconv.Itoa(*width) || attribute(inner, "", "cy") != strconv.Itoa(*height) {
		return nil
	}
	fill := solidShapeColor(childNS(properties, drawingNS, "solidFill"))
	lineFill := singlePayload(line)
	if lineFill == nil || lineFill.name.Space != drawingNS || lineFill.name.Local != "solidFill" {
		return nil
	}
	stroke := solidShapeColor(lineFill)
	if fill == nil || stroke == nil {
		return nil
	}
	return &inlineRectangleInfo{
		RectanglePreview: RectanglePreview{WidthEMU: *width, HeightEMU: *height, Fill: attribute(fill, "", "val"), Stroke: attribute(stroke, "", "val")},
		fillNode:         fill, strokeNode: stroke,
		frameSize: outer, transformSize: inner,
	}
}

func rectangleForRun(run *node) *inlineRectangleInfo {
	inline := childNS(run.child("drawing"), drawingWordNS, "inline")
	graphic := childNS(childNS(inline, drawingNS, "graphic"), drawingNS, "graphicData")
	return inspectInlineRectangle(inline, graphic)
}

func (info *inlineRectangleInfo) canResize(data []byte) bool {
	if info == nil {
		return false
	}
	frames := 0
	for _, child := range info.frameSize.parent.children {
		if child.name.Space != drawingWordNS {
			continue
		}
		if child.name.Local == "extent" {
			frames++
		}
		if child.name.Local == "effectExtent" {
			for _, attr := range child.attrs {
				if !isNamespaceAttribute(attr) && attr.Value != "0" {
					return false
				}
			}
		}
	}
	if frames != 1 {
		return false
	}
	for _, size := range []*node{info.frameSize, info.transformSize} {
		if len(size.children) != 0 || strings.TrimSpace(string(data[size.inner:size.endStart])) != "" {
			return false
		}
	}
	return true
}

func resizeInlineRectangle(data []byte, t target, edit RectangleSizeEdit) ([]replacement, error) {
	info := rectangleForRun(t.run)
	if !info.canResize(data) || edit.WidthEMU < 12700 || edit.WidthEMU > maxImageExtent ||
		edit.HeightEMU < 12700 || edit.HeightEMU > maxImageExtent ||
		edit.WidthEMU == info.WidthEMU && edit.HeightEMU == info.HeightEMU {
		return nil, fmt.Errorf("invalid or unsupported DOCX rectangle size")
	}
	dimensions := map[string]string{"cx": strconv.Itoa(edit.WidthEMU), "cy": strconv.Itoa(edit.HeightEMU)}
	replacements := make([]replacement, 0, 2)
	for _, size := range []*node{info.frameSize, info.transformSize} {
		raw, err := patchScalarProperty(size, data, size.name.Local, "", dimensions, nil)
		if err != nil {
			return nil, err
		}
		replacements = append(replacements, replacement{size.start, size.end, []byte(raw)})
	}
	return replacements, nil
}

func editInlineRectangleColors(data []byte, t target, edit RectangleColorEdit) ([]replacement, error) {
	if !validRectangleColor(edit.Fill) || !validRectangleColor(edit.Stroke) {
		return nil, fmt.Errorf("DOCX rectangle colors require six hexadecimal digits")
	}
	info := rectangleForRun(t.run)
	if info == nil {
		return nil, fmt.Errorf("unsupported DOCX rectangle")
	}
	var replacements []replacement
	for _, color := range []struct {
		node     *node
		old, rgb string
	}{{info.fillNode, info.Fill, edit.Fill}, {info.strokeNode, info.Stroke, edit.Stroke}} {
		if strings.EqualFold(color.old, color.rgb) {
			continue
		}
		raw, err := patchScalarProperty(color.node, data, "srgbClr", "", map[string]string{"val": strings.ToUpper(color.rgb)}, nil)
		if err != nil {
			return nil, err
		}
		replacements = append(replacements, replacement{color.node.start, color.node.end, []byte(raw)})
	}
	if len(replacements) == 0 {
		return nil, fmt.Errorf("DOCX rectangle colors are unchanged")
	}
	return replacements, nil
}

func inlineRectangle(id uint32) string {
	return inlineWordShape(id, "Rectangle", 1143000, 571500,
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
<a:solidFill><a:srgbClr val="D7EAE4"/></a:solidFill><a:ln w="12700"><a:solidFill><a:srgbClr val="0F766E"/></a:solidFill></a:ln>`)
}

func inlineWordShape(id uint32, name string, width, height int, geometry string) string {
	return fmt.Sprintf(`<dws:r xmlns:dws="%s" xmlns:wp="%s" xmlns:a="%s" xmlns:wps="%s">
<dws:drawing><wp:inline><wp:extent cx="%d" cy="%d"/><wp:docPr id="%d" name="%s %d"/><wp:cNvGraphicFramePr/>
<a:graphic><a:graphicData uri="%s"><wps:wsp><wps:cNvSpPr/>
<wps:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm>%s</wps:spPr>
<wps:bodyPr/></wps:wsp></a:graphicData></a:graphic></wp:inline></dws:drawing></dws:r>`,
		wordNS, drawingWordNS, drawingNS, shapeWordNS, width, height, id, name, id, shapeWordNS, width, height, geometry)
}
