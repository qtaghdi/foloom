package docx

import (
	"fmt"
	"strconv"
	"strings"
)

// DrawingML stores each degree as 60,000 units.
const drawingUnitsPerDegree = 60_000

func basicPictureAngle(transform *node) int {
	rotation := 0
	for _, attr := range transform.attrs {
		if isNamespaceAttribute(attr) {
			continue
		}
		if attr.Name.Space != "" || attr.Name.Local != "rot" {
			return -1
		}
		units, err := strconv.Atoi(attr.Value)
		if err != nil || units <= -360*drawingUnitsPerDegree || units >= 360*drawingUnitsPerDegree ||
			units%(90*drawingUnitsPerDegree) != 0 {
			return -1
		}
		rotation = (units/drawingUnitsPerDegree + 360) % 360
	}
	return rotation
}

func supportedPictureRotation(inline *node, data []byte) (*node, int, bool) {
	if crop, _, _ := supportedPictureCrop(inline, data); crop == nil {
		return nil, 0, false
	}
	graphic := childNS(childNS(inline, drawingNS, "graphic"), drawingNS, "graphicData")
	shape := childNS(childNS(graphic, pictureNS, "pic"), pictureNS, "spPr")
	transform := childNS(shape, drawingNS, "xfrm")
	geometry := childNS(shape, drawingNS, "prstGeom")
	if transform == nil || len(shape.children) != 2 || attribute(geometry, "", "prst") != "rect" ||
		len(transform.children) != 2 || strings.TrimSpace(transform.text.String()) != "" {
		return nil, 0, false
	}
	off, size := transform.children[0], transform.children[1]
	frame := childNS(inline, drawingWordNS, "extent")
	rotation := basicPictureAngle(transform)
	if rotation < 0 || off.name.Space != drawingNS || off.name.Local != "off" ||
		size.name.Space != drawingNS || size.name.Local != "ext" || frame == nil ||
		attribute(off, "", "x") != "0" || attribute(off, "", "y") != "0" ||
		attribute(size, "", "cx") != attribute(frame, "", "cx") ||
		attribute(size, "", "cy") != attribute(frame, "", "cy") {
		return nil, 0, false
	}
	return transform, rotation, true
}

func rotateInlinePicture(data []byte, inline *node, rotation int) (replacement, error) {
	transform, current, ok := supportedPictureRotation(inline, data)
	if !ok || (rotation < 0 || rotation >= 360 || rotation%90 != 0) || rotation == current ||
		strings.Contains(string(data[transform.start:transform.end]), "<!--") {
		return replacement{}, fmt.Errorf("unsupported DOCX picture rotation")
	}
	values := map[string]string{}
	var remove []string
	if rotation == 0 {
		remove = []string{"rot"}
	} else {
		values["rot"] = strconv.Itoa(rotation * drawingUnitsPerDegree)
	}
	open, _ := scopedOpening(transform, data, false)
	open, err := patchOpeningAttributes(open, "", values, remove)
	if err != nil {
		return replacement{}, err
	}
	return replacement{start: transform.start, end: transform.inner, data: []byte(open)}, nil
}
