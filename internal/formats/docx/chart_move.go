package docx

import (
	"fmt"
	"strings"
)

func markChartMoves(root *node, paragraphs map[*node]*Paragraph, protected map[*node]bool, targets map[string]target, styles previewStyles) {
	body := root.child("body")
	if !body.has("drawing") {
		return
	}
	ranged, valid := bodyRangeBlocks(body)
	if !valid {
		return
	}
	safe := func(p *node) bool {
		view := paragraphs[p]
		return view != nil && !protected[p] && !ranged[p] && !view.PageBreakBefore && !view.PageBreakAfter &&
			view.List == nil && view.OutlineLevel == nil && plainVisualParagraphProperties(p)
	}
	isCaptionNeighbor := func(i int) bool {
		return i >= 0 && i < len(body.children) && chartCaptionNeighbor(body.children[i], styles)
	}
	for i, p := range body.children {
		if !safe(p) || !p.has("drawing") {
			continue
		}
		var before, after *node
		if i > 0 {
			before = body.children[i-1]
		}
		if i+1 < len(body.children) {
			after = body.children[i+1]
		}
		// A neighboring caption can belong to either side of the chart.
		// Conservatively leave charts adjacent to fields or caption styles fixed.
		if isCaptionNeighbor(i-1) || isCaptionNeighbor(i+1) {
			continue
		}
		for _, run := range paragraphs[p].Runs {
			t := targets[run.ID]
			if t.kind != "chart" || !dedicatedDrawingParagraph(t, root.source) || !deletableInlineChart(t, root.source) {
				continue
			}
			if safe(before) && paragraphs[before].StructureEditable && !isCaptionNeighbor(i-2) && whitespaceBetween(root.source, before, p) {
				t.moveUp = before
			}
			if safe(after) && paragraphs[after].StructureEditable && !isCaptionNeighbor(i+2) && whitespaceBetween(root.source, p, after) {
				t.moveDown = after
			}
			targets[run.ID] = t
		}
	}
}

func chartCaptionNeighbor(p *node, styles previewStyles) bool {
	if !p.is("p") {
		return false
	}
	style := p.child("pPr").child("pStyle").attr("val")
	if strings.EqualFold(style, "Caption") || p.has("fldSimple") || p.has("fldChar") || p.has("instrText") {
		return true
	}
	for _, ancestor := range styles.chain(style, "paragraph") {
		if strings.EqualFold(ancestor.attr("styleId"), "Caption") || strings.EqualFold(ancestor.child("name").attr("val"), "Caption") {
			return true
		}
	}
	return false
}

func (p *documentPackage) moveInlineChart(t target, operation string) (replacement, error) {
	if t.kind != "chart" || t.moveUp == nil && t.moveDown == nil {
		return replacement{}, fmt.Errorf("protected or unsupported DOCX chart movement")
	}
	if title, _, _ := p.chartTitle(chartRelationshipID(t.run)); title == nil {
		return replacement{}, fmt.Errorf("unsupported or externally linked DOCX chart")
	}
	return movePreservedParagraph(p.document, t, operation)
}
