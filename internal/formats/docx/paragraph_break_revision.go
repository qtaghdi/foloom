package docx

import (
	"fmt"
	"strconv"
	"strings"
)

func paragraphBreakMarker(p *node) *node {
	return p.child("pPr").child("rPr").child("ins")
}

func markParagraphBreakRevisions(root *node, paragraphs map[*node]*Paragraph, protected map[*node]bool, targets map[string]target) {
	body := root.child("body")
	if !body.has("ins") {
		return
	}
	var rangeBlocks map[*node]bool
	var validRanges, permissionRanges bool
	var ids map[int64]int
	for i, left := range body.children {
		preview := paragraphs[left]
		if preview == nil {
			continue
		}
		marker := paragraphBreakMarker(left)
		if marker == nil {
			continue
		}
		if ids == nil {
			ids = countBodyRevisionIDs(root)
			rangeBlocks, validRanges = bodyRangeBlocks(body)
			permissionRanges = body.has("permStart") || body.has("permEnd")
		}
		var right *node
		if i+1 < len(body.children) {
			right = body.children[i+1]
		}
		rightPreview := paragraphs[right]
		text := textContent(left)
		if rightPreview != nil {
			text += "\n" + textContent(right)
		}
		reason := ""
		id, err := strconv.ParseInt(marker.attr("id"), 10, 32)
		switch {
		case err != nil || id < 0 || ids[id] != 1 || !plainParagraphBreakMarker(marker):
			reason = "문단 나눔 변경 정보를 고유하게 확인할 수 없습니다."
		case permissionRanges ||
			rightPreview == nil || protected[left] || protected[right] || !validRanges || rangeBlocks[left] || rangeBlocks[right] ||
			preview.PageBreakBefore || preview.PageBreakAfter || rightPreview.PageBreakBefore || rightPreview.PageBreakAfter:
			reason = "참조 범위나 쪽·구역 경계가 있는 문단은 보호됩니다."
		case !whitespaceBetween(root.source, left, right) ||
			!structuralParagraphContents(left, root.source, "", marker) ||
			!structuralParagraphContents(right, root.source, "", nil) ||
			!compatibleBreakParagraphs(left, right, marker, root.source):
			reason = "앞뒤 문단에 복합 내용이 있거나 문단 서식이 서로 다릅니다."
		}
		if reason != "" {
			text = reason + "\n" + text
		}
		change := formatRevisionPreview(marker, text, reason == "")
		change.Kind, change.Label = "tracked_insert", "문단 나눔 삽입"
		preview.BreakRevision = change
		if change.Editable {
			targets[preview.ID] = target{kind: "tracked_paragraph_break", node: marker, paragraph: left, moveDown: right}
		}
	}
}

func countBodyRevisionIDs(root *node) map[int64]int {
	ids := map[int64]int{}
	var visit func(*node)
	visit = func(n *node) {
		if n.name.Space == wordNS {
			switch n.name.Local {
			case "ins", "del", "moveFrom", "moveTo", "pPrChange", "rPrChange", "tblPrChange", "tblGridChange", "trPrChange", "tcPrChange", "sectPrChange":
				if id, err := strconv.ParseInt(n.attr("id"), 10, 32); err == nil {
					ids[id]++
				}
			}
		}
		for _, child := range n.children {
			visit(child)
		}
	}
	visit(root)
	return ids
}

func plainParagraphBreakMarker(marker *node) bool {
	if len(marker.children) != 0 || strings.TrimSpace(marker.text.String()) != "" || countChild(marker.parent, "ins") != 1 ||
		countChild(marker.parent.parent, "rPr") != 1 {
		return false
	}
	for _, attr := range marker.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || attr.Name.Local != "id" && attr.Name.Local != "author" && attr.Name.Local != "date") {
			return false
		}
	}
	return true
}

func compatibleBreakParagraphs(left, right, marker *node, data []byte) bool {
	propsXML := func(p, skip *node) (string, bool) {
		props := p.child("pPr")
		if props == nil {
			return "", true
		}
		if !onlyNamespaceAttributes(props) {
			return "", false
		}
		var out strings.Builder
		for _, child := range props.children {
			if !child.is("rPr") {
				out.Write(data[child.start:child.end])
				continue
			}
			if !onlyNamespaceAttributes(child) {
				return "", false
			}
			var contents strings.Builder
			for _, property := range child.children {
				if property != skip {
					contents.Write(data[property.start:property.end])
				}
			}
			if contents.Len() != 0 {
				out.WriteString("<rPr>" + contents.String() + "</rPr>")
			}
		}
		return out.String(), true
	}
	leftProps, leftOK := propsXML(left, marker)
	rightProps, rightOK := propsXML(right, nil)
	return leftOK && rightOK && leftProps == rightProps
}

func paragraphBreakRevisionReplacement(data []byte, t target, operation string) (replacement, error) {
	if t.kind != "tracked_paragraph_break" {
		return replacement{}, fmt.Errorf("protected DOCX paragraph break revision")
	}
	left, right := t.paragraph, t.moveDown
	remove := t.node
	if len(remove.parent.children) == 1 {
		remove = remove.parent
		if len(remove.parent.children) == 1 {
			remove = remove.parent
		}
	}
	if operation == "accept_revision" {
		return replacement{start: remove.start, end: remove.end}, nil
	}

	var result strings.Builder
	result.Write(data[left.start:remove.start])
	result.Write(data[remove.end:left.endStart])
	// Right runs can declare prefixes that were inherited from the removed paragraph.
	for _, run := range right.children {
		if run.is("r") {
			open, close := scopedOpening(run, data, false)
			result.WriteString(open)
			result.Write(data[run.inner:run.endStart])
			result.WriteString(close)
		}
	}
	result.Write(data[left.endStart:right.start])
	if result.Len() > maxPartBytes {
		return replacement{}, fmt.Errorf("DOCX paragraph revision exceeds XML size limit")
	}
	return replacement{start: left.start, end: right.end, data: []byte(result.String())}, nil
}
