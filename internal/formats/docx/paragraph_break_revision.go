package docx

import (
	"fmt"
	"strconv"
	"strings"
)

func paragraphBreakMarker(p *node) *node {
	props := p.child("pPr").child("rPr")
	if inserted := props.child("ins"); inserted != nil {
		return inserted
	}
	return props.child("del")
}

func markParagraphBreakRevisions(root *node, paragraphs map[*node]*Paragraph, protected map[*node]bool, targets map[string]target) {
	body := root.child("body")
	if !body.has("ins") && !body.has("del") {
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
		case marker.is("del") && (preview.List != nil || rightPreview.List != nil):
			reason = "목록이 있는 문단 나눔은 보호됩니다."
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
		if marker.is("del") {
			change.Kind, change.Label = "tracked_delete", "문단 나눔 삭제"
		}
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
	if len(marker.children) != 0 || strings.TrimSpace(marker.text.String()) != "" || countChild(marker.parent, "ins")+countChild(marker.parent, "del") != 1 ||
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
		if p.children[0] != props || !onlyNamespaceAttributes(props) || countChild(props, "rPr") > 1 {
			return "", false
		}
		ignoreAlignment := marker.is("del")
		if ignoreAlignment && countChild(props, "jc") > 1 {
			return "", false
		}
		var out strings.Builder
		for _, child := range props.children {
			if child.is("jc") && ignoreAlignment {
				if !plainBreakAlignment(child) {
					return "", false
				}
				continue
			}
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

func plainBreakAlignment(justification *node) bool {
	if paragraphAlignmentValue(justification.attr("val")) == "" || len(justification.children) != 0 ||
		strings.TrimSpace(justification.text.String()) != "" {
		return false
	}
	for _, a := range justification.attrs {
		if !isNamespaceAttribute(a) && (a.Name.Space != wordNS || a.Name.Local != "val") {
			return false
		}
	}
	return true
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
	deleted := t.node.is("del")
	join := deleted && operation == "accept_revision" || !deleted && operation == "reject_revision"
	if !join {
		return replacement{start: remove.start, end: remove.end}, nil
	}

	var result strings.Builder
	if deleted {
		// The following paragraph's mark survives, so keep its ID and properties.
		open, close := scopedOpening(right, data, false)
		result.WriteString(open)
		content := right.inner
		if props := right.child("pPr"); props != nil {
			result.Write(data[content:props.end])
			content = props.end
		}
		appendReviewedParagraphRuns(&result, left, data)
		if content < right.endStart {
			result.Write(data[content:right.endStart])
		}
		result.WriteString(close)
		result.Write(data[left.end:right.start])
	} else {
		result.Write(data[left.start:remove.start])
		result.Write(data[remove.end:left.endStart])
		appendReviewedParagraphRuns(&result, right, data)
		result.Write(data[left.endStart:right.start])
	}
	if result.Len() > maxPartBytes {
		return replacement{}, fmt.Errorf("DOCX paragraph revision exceeds XML size limit")
	}
	return replacement{start: left.start, end: right.end, data: []byte(result.String())}, nil
}

func appendReviewedParagraphRuns(result *strings.Builder, paragraph *node, data []byte) {
	// A transplanted run can inherit prefixes defined on a removed paragraph.
	for _, run := range paragraph.children {
		if run.is("r") {
			open, close := scopedOpening(run, data, false)
			result.WriteString(open)
			if run.inner < run.endStart {
				result.Write(data[run.inner:run.endStart])
			}
			result.WriteString(close)
		}
	}
}
