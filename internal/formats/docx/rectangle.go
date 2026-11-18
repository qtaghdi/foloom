package docx

import (
	"fmt"
	"strconv"
)

type RectanglePreview struct {
	WidthEMU  int    `json:"widthEmu"`
	HeightEMU int    `json:"heightEmu"`
	Fill      string `json:"fill"`
	Stroke    string `json:"stroke"`
}

func solidShapeColor(parent *node) string {
	color := singlePayload(parent)
	if color == nil || color.name.Space != drawingNS || color.name.Local != "srgbClr" || len(color.children) != 0 {
		return ""
	}
	rgb := attribute(color, "", "val")
	if len(rgb) != 6 {
		return ""
	}
	for _, digit := range rgb {
		if !(digit >= '0' && digit <= '9' || digit >= 'a' && digit <= 'f' || digit >= 'A' && digit <= 'F') {
			return ""
		}
	}
	return rgb
}

func previewInlineRectangle(inline, graphic *node) *RectanglePreview {
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
	if fill == "" || stroke == "" {
		return nil
	}
	return &RectanglePreview{WidthEMU: *width, HeightEMU: *height, Fill: fill, Stroke: stroke}
}

func inlineRectangle(id uint32) string {
	return fmt.Sprintf(`<dws:r xmlns:dws="%s" xmlns:wp="%s" xmlns:a="%s" xmlns:wps="%s">
<dws:drawing><wp:inline><wp:extent cx="1143000" cy="571500"/><wp:docPr id="%d" name="Rectangle %d"/><wp:cNvGraphicFramePr/>
<a:graphic><a:graphicData uri="%s"><wps:wsp><wps:cNvSpPr/>
<wps:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="1143000" cy="571500"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
<a:solidFill><a:srgbClr val="D7EAE4"/></a:solidFill><a:ln w="12700"><a:solidFill><a:srgbClr val="0F766E"/></a:solidFill></a:ln></wps:spPr>
<wps:bodyPr/></wps:wsp></a:graphicData></a:graphic></wp:inline></dws:drawing></dws:r>`,
		wordNS, drawingWordNS, drawingNS, shapeWordNS, id, id, shapeWordNS)
}
