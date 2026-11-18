package docx

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func trailingNoteSplitRun(paragraph *node) *node {
	children := paragraph.children
	if len(children) != 0 && children[0].is("pPr") {
		children = children[1:]
	}
	if len(children) < 2 {
		return nil
	}
	run := children[0]
	if !run.is("r") || len(run.children) > 2 || len(run.children) == 2 && !run.children[0].is("rPr") {
		return nil
	}
	text := run.child("t")
	if text == nil || !validText(text.text.String()) || utf8.RuneCountInString(text.text.String()) < 2 {
		return nil
	}
	for _, marker := range children[1:] {
		if plainBodyNoteReference(marker) == nil {
			return nil
		}
	}
	return run
}

func splitNoteParagraph(data []byte, t target, op Operation, ids paragraphIDs) (replacement, error) {
	if !t.splitNoteParagraph || !validText(op.Text) || op.ExpectedText != t.text ||
		op.Text == "" || op.Text == t.text || !strings.HasPrefix(t.text, op.Text) {
		return replacement{}, fmt.Errorf("DOCX note paragraph requires a split within the original text")
	}
	n := t.run.child("t")
	left, err := replaceTextNode(data, n, op.Text)
	if err != nil {
		return replacement{}, err
	}
	right, err := replaceTextNode(data, n, strings.TrimPrefix(t.text, op.Text))
	if err != nil {
		return replacement{}, err
	}
	id, err := ids.reserve()
	if err != nil {
		return replacement{}, err
	}
	p := t.paragraph
	open, close := newParagraphOpening(p, data, id)
	var result strings.Builder
	result.Write(data[p.start:left.start])
	result.Write(left.data)
	result.Write(data[left.end:t.run.end])
	result.Write(data[p.endStart:p.end])
	result.WriteString(open)
	result.Write(data[p.inner:right.start])
	result.Write(right.data)
	// The original markers and their whitespace occur only in the new paragraph.
	result.Write(data[right.end:p.endStart])
	result.WriteString(close)
	if result.Len() > maxPartBytes {
		return replacement{}, fmt.Errorf("DOCX note paragraph split exceeds the XML size limit")
	}
	return replacement{start: p.start, end: p.end, data: []byte(result.String())}, nil
}
