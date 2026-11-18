package docx

import (
	"fmt"
	"strconv"
	"strings"
)

type ChartFrame struct {
	WidthEMU  int `json:"widthEmu"`
	HeightEMU int `json:"heightEmu"`
}

func inlineChartFrame(run *node, data []byte) (*ChartFrame, *node) {
	inline := singlePayload(run.child("drawing"))
	if inline == nil || inline.name.Space != drawingWordNS || inline.name.Local != "inline" {
		return nil, nil
	}
	var extent *node
	for _, child := range inline.children {
		if child.name.Space != drawingWordNS {
			continue
		}
		switch child.name.Local {
		case "extent":
			if extent != nil {
				return nil, nil
			}
			extent = child
		case "effectExtent":
			// Effect margins can change the position and visible size independently.
			for _, attr := range child.attrs {
				if !isNamespaceAttribute(attr) && attr.Value != "0" {
					return nil, nil
				}
			}
		}
	}
	if extent == nil || len(extent.children) != 0 || strings.TrimSpace(string(data[extent.inner:extent.endStart])) != "" {
		return nil, nil
	}
	for _, attr := range extent.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != "" || attr.Name.Local != "cx" && attr.Name.Local != "cy") {
			return nil, nil
		}
	}
	width := previewNumber(attribute(extent, "", "cx"), 1, maxImageExtent)
	height := previewNumber(attribute(extent, "", "cy"), 1, maxImageExtent)
	if width == nil || height == nil {
		return nil, nil
	}
	return &ChartFrame{WidthEMU: *width, HeightEMU: *height}, extent
}

func (p *documentPackage) resizeInlineChart(t target, counts map[string]int, width int) (replacement, error) {
	relationship := chartRelationshipID(t.run)
	if !p.uniqueChart(relationship, counts) {
		return replacement{}, fmt.Errorf("unsupported or shared DOCX chart")
	}
	title, _, _ := p.chartTitle(relationship)
	frame, extent := inlineChartFrame(t.run, p.document)
	if title == nil || frame == nil || width < 9525 || width > maxImageExtent || width == frame.WidthEMU {
		return replacement{}, fmt.Errorf("invalid or unsupported DOCX chart resize")
	}
	height := int(int64(width) * int64(frame.HeightEMU) / int64(frame.WidthEMU))
	if height < 1 || height > maxImageExtent {
		return replacement{}, fmt.Errorf("invalid DOCX chart height")
	}
	raw, err := patchScalarProperty(extent, p.document, "extent", "",
		map[string]string{"cx": strconv.Itoa(width), "cy": strconv.Itoa(height)}, nil)
	if err != nil {
		return replacement{}, err
	}
	return replacement{start: extent.start, end: extent.end, data: []byte(raw)}, nil
}
