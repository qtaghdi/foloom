package docx

import (
	"fmt"
	"strings"
)

func deletableInlineChart(t target, data []byte) bool {
	if t.kind != "chart" || strings.Contains(string(data[t.run.start:t.run.end]), "<!--") ||
		strings.TrimSpace(t.run.text.String()) != "" {
		return false
	}
	for _, attr := range t.run.attrs {
		if !isNamespaceAttribute(attr) && !(attr.Name.Space == wordNS && strings.HasPrefix(attr.Name.Local, "rsid")) {
			return false
		}
	}
	for _, name := range []string{"bookmarkStart", "bookmarkEnd", "commentRangeStart", "commentRangeEnd", "commentReference"} {
		if t.paragraph.has(name) {
			return false
		}
	}
	if props := t.run.child("rPr"); props != nil {
		if t.run.children[0] != props || strings.TrimSpace(props.text.String()) != "" || !onlyNamespaceAttributes(props) {
			return false
		}
		for _, child := range props.children {
			if child.name.Space != wordNS || len(child.children) != 0 || strings.TrimSpace(child.text.String()) != "" {
				return false
			}
		}
	}
	inline := singlePayload(t.run.child("drawing"))
	if inline == nil || inline.name.Space != drawingWordNS || inline.name.Local != "inline" ||
		chartRelationshipID(t.run) == "" {
		return false
	}
	seen := map[string]bool{}
	for _, child := range inline.children {
		key := child.name.Local
		if seen[key] {
			return false
		}
		seen[key] = true
		if child.name.Space == drawingNS && key == "graphic" {
			content := singlePayload(child)
			if content == nil || content.name.Space != drawingNS || content.name.Local != "graphicData" {
				return false
			}
			chart := singlePayload(content)
			if chart == nil || len(chart.children) != 0 || strings.TrimSpace(chart.text.String()) != "" {
				return false
			}
			continue
		}
		if child.name.Space != drawingWordNS || key != "extent" && key != "effectExtent" && key != "docPr" && key != "cNvGraphicFramePr" {
			return false
		}
	}
	frame, _ := inlineChartFrame(t.run, data)
	return frame != nil && seen["docPr"]
}

func (p *documentPackage) deleteInlineChart(t target) (replacement, error) {
	if !deletableInlineChart(t, p.document) {
		return replacement{}, fmt.Errorf("protected or unsupported DOCX chart placement")
	}
	if title, _, _ := p.chartTitle(chartRelationshipID(t.run)); title == nil {
		return replacement{}, fmt.Errorf("unsupported or externally linked DOCX chart")
	}
	// Other placements may share the chart or workbook. Remove only this run
	// and keep the original relationship graph and all chart parts intact.
	return replacement{start: t.run.start, end: t.run.end}, nil
}
