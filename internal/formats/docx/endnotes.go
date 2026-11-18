package docx

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func plainEndnoteText(note *node, source []byte) ([]*node, string) {
	kind := note.attr("type")
	if kind != "" && kind != "normal" || len(note.children) != 1 || !note.children[0].is("p") ||
		strings.Contains(string(source[note.start:note.end]), "<!--") {
		return nil, ""
	}
	for _, name := range []string{"ins", "del", "pPrChange", "rPrChange"} {
		if note.has(name) {
			return nil, ""
		}
	}
	p := note.children[0]
	runs := p.children
	if len(runs) > 0 && runs[0].is("pPr") {
		runs = runs[1:]
	}
	if len(runs) < 2 || !runs[0].is("r") || strings.TrimSpace(runs[0].text.String()) != "" ||
		countChild(runs[0], "endnoteRef") != 1 ||
		len(runs[0].children) != 1 && (len(runs[0].children) != 2 || runs[0].child("rPr") == nil) {
		return nil, ""
	}
	return plainTextRunNodes(runs[1:])
}

func (p *documentPackage) changeEndnote(t target, edit ReferenceEdit, changes map[string][]byte) error {
	next := strings.TrimSpace(edit.Text)
	if t.kind != "endnote" || edit.URL != "" || edit.Bookmark != "" || next == "" ||
		!validText(edit.Text) || len(edit.Text) > 1024 {
		return fmt.Errorf("invalid DOCX endnote edit")
	}
	id, err := strconv.ParseInt(t.node.child("endnoteReference").attr("id"), 10, 32)
	if err != nil || id <= 0 {
		return fmt.Errorf("invalid DOCX endnote identifier")
	}
	filename, root, err := p.notesPart("endnote", changes)
	if err != nil || root == nil {
		return fmt.Errorf("DOCX endnotes part unavailable")
	}
	var note *node
	for _, child := range root.children {
		other, err := strconv.ParseInt(child.attr("id"), 10, 32)
		if !child.is("endnote") || err != nil || other != id {
			continue
		}
		if note != nil {
			return fmt.Errorf("ambiguous DOCX endnote")
		}
		note = child
	}
	if note == nil {
		return fmt.Errorf("DOCX endnote is missing")
	}
	nodes, old := plainEndnoteText(note, root.source)
	if len(nodes) == 0 {
		return fmt.Errorf("protected DOCX endnote cannot be changed")
	}
	left := len(old) - len(strings.TrimLeftFunc(old, unicode.IsSpace))
	right := len(strings.TrimRightFunc(old, unicode.IsSpace))
	next = old[:left] + next + old[max(left, right):]
	if next == old {
		return nil
	}
	if len(next) > 1024 {
		return fmt.Errorf("DOCX endnote exceeds plain text limit")
	}
	edits, err := plainRunTextEdits(root.source, nodes, old, next)
	if err != nil {
		return err
	}
	var raw bytes.Buffer
	offset := 0
	for _, edit := range edits {
		raw.Write(root.source[offset:edit.start])
		raw.Write(edit.data)
		offset = edit.end
	}
	raw.Write(root.source[offset:])
	changes[filename] = raw.Bytes()
	return nil
}
