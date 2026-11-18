package docx

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
)

func plainParagraphAlignment(props *node) bool {
	if props == nil {
		return true
	}
	if !onlyNamespaceAttributes(props) || strings.TrimSpace(props.text.String()) != "" || countChild(props, "jc") > 1 {
		return false
	}
	alignment := props.child("jc")
	if alignment == nil {
		return true
	}
	if paragraphAlignmentValue(alignment.attr("val")) == "" || len(alignment.children) != 0 ||
		strings.TrimSpace(alignment.text.String()) != "" {
		return false
	}
	for _, attr := range alignment.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || attr.Name.Local != "val") {
			return false
		}
	}
	return true
}

func alignmentRevision(paragraph *node, data []byte) (change, previous *node) {
	props := paragraph.child("pPr")
	change = props.child("pPrChange")
	if change == nil || countChild(props, "pPrChange") != 1 || props.children[len(props.children)-1] != change ||
		!validFormatRevisionMetadata(change) || !structuralParagraphContents(paragraph, data, "", change) ||
		!plainParagraphAlignment(props) || props.child("jc") == nil {
		return nil, nil
	}
	previous = change.children[0]
	if !previous.is("pPr") || !plainParagraphAlignment(previous) ||
		paragraphAlignmentValue(props.child("jc").attr("val")) == paragraphAlignmentValue(previous.child("jc").attr("val")) {
		return nil, nil
	}
	// Restrict review to an alignment-only change. Identical sibling XML can
	// remain in place when accepting or rejecting.
	siblings := func(node *node) [][]byte {
		var result [][]byte
		for _, child := range node.children {
			if child != change && !child.is("jc") {
				result = append(result, data[child.start:child.end])
			}
		}
		return result
	}
	current, prior := siblings(props), siblings(previous)
	slices.SortFunc(current, bytes.Compare)
	slices.SortFunc(prior, bytes.Compare)
	if len(current) != len(prior) {
		return nil, nil
	}
	for i := range current {
		if !bytes.Equal(current[i], prior[i]) {
			return nil, nil
		}
	}
	return change, previous
}

func alignmentRevisionReplacement(data []byte, t target, operation string) ([]replacement, error) {
	change, previous := alignmentRevision(t.paragraph, data)
	if change == nil {
		return nil, fmt.Errorf("protected DOCX paragraph alignment revision")
	}
	edits := []replacement{{start: change.start, end: change.end}}
	if operation == "reject_revision" {
		current, prior := change.parent.child("jc"), previous.child("jc")
		var original []byte
		if prior != nil {
			open, close := scopedOpening(prior, data, false)
			original = []byte(open + close)
		}
		edits = append(edits, replacement{start: current.start, end: current.end, data: original})
	}
	return edits, nil
}

func trackedParagraphAlignment(data []byte, t target, op Operation, author, date string, used map[int64]bool, nextID *int64) (replacement, error) {
	paragraph := t.paragraph
	if !t.alignmentTrackable || t.kind != "replace_paragraphs" || op.ExpectedText != "" || op.Text != "" {
		return replacement{}, fmt.Errorf("invalid DOCX tracked paragraph alignment target or fields")
	}
	switch op.Alignment {
	case "start", "center", "end", "justify":
	default:
		return replacement{}, fmt.Errorf("unsupported DOCX tracked paragraph alignment")
	}
	props := paragraph.child("pPr")
	if paragraphAlignmentValue(props.child("jc").attr("val")) == op.Alignment {
		return replacement{}, fmt.Errorf("DOCX paragraph alignment did not change")
	}
	current, err := paragraphProperties(props, data, &ParagraphFormat{Align: op.Alignment})
	if err != nil {
		return replacement{}, err
	}
	previous := `<dwf:pPr xmlns:dwf="` + wordNS + `"/>`
	start, end := paragraph.inner, paragraph.inner
	if props != nil {
		open, close := scopedOpening(props, data, false)
		content := ""
		if props.inner < props.endStart {
			content = string(data[props.inner:props.endStart])
		}
		previous = open + content + close
		start, end = props.start, props.end
	}
	revision := formatRevisionMarkup("pPrChange", previous, author, date, used, nextID)
	raw, err := appendXMLChild([]byte(current), revision)
	if err != nil {
		return replacement{}, err
	}
	if strings.HasSuffix(string(data[paragraph.start:paragraph.inner]), "/>") {
		return appendParagraphContent(data, paragraph, string(raw)), nil
	}
	return replacement{start: start, end: end, data: raw}, nil
}

func paragraphAlignmentRevisionPreview(paragraph *node, data []byte, reviewable bool) *ObjectInfo {
	props := paragraph.child("pPr")
	change := props.child("pPrChange")
	if change == nil {
		return nil
	}
	verified, previous := alignmentRevision(paragraph, data)
	label := "복합 문단 서식 변경"
	if verified != nil {
		names := map[string]string{"": "스타일 상속", "start": "시작", "center": "가운데", "end": "끝", "justify": "양쪽"}
		before := names[paragraphAlignmentValue(previous.child("jc").attr("val"))]
		after := names[paragraphAlignmentValue(props.child("jc").attr("val"))]
		label = "정렬 변경 " + before + " → " + after
	}
	preview := formatRevisionPreview(change, textContent(paragraph), reviewable && verified != nil)
	preview.Label = label
	return preview
}
