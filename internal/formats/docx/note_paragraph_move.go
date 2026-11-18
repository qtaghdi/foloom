package docx

import (
	"strconv"
	"strings"
)

func plainBodyNoteReference(run *node) *node {
	if !run.is("r") || len(run.children) == 0 || len(run.children) > 2 {
		return nil
	}
	if len(run.children) == 2 && !run.children[0].is("rPr") {
		return nil
	}
	ref := run.children[len(run.children)-1]
	if !(ref.is("footnoteReference") || ref.is("endnoteReference")) ||
		len(ref.children) != 0 || strings.TrimSpace(ref.text.String()) != "" {
		return nil
	}
	for _, attr := range ref.attrs {
		if isNamespaceAttribute(attr) || attr.Name.Space == wordNS && attr.Name.Local == "id" {
			continue
		}
		if attr.Name.Space == wordNS && attr.Name.Local == "customMarkFollows" {
			switch attr.Value {
			case "0", "false", "off":
				continue
			}
		}
		return nil
	}
	id, err := strconv.ParseInt(ref.attr("id"), 10, 32)
	if err != nil || id < 1 {
		return nil
	}
	return ref
}

func markNoteParagraphs(root *node, paragraphs map[*node]*Paragraph, protected map[*node]bool, targets map[string]target) {
	body := root.child("body")
	if !body.has("footnoteReference") && !body.has("endnoteReference") {
		return
	}
	markBodyParagraphMoves(root, paragraphs, protected, func(p *node) bool {
		for _, run := range p.children {
			if plainBodyNoteReference(run) != nil {
				return structuralParagraphContents(p, root.source, "note", nil)
			}
		}
		return false
	}, func(p, before, after *node) {
		paragraph := paragraphs[p]
		paragraph.MoveNoteUp, paragraph.MoveNoteDown = before != nil, after != nil
		targets[paragraph.ID] = target{kind: "note_paragraph", paragraph: p, moveUp: before, moveDown: after}
	})
}

func (p *documentPackage) inspectWithNoteMoves(root *node, sessionID string, revision uint64) ([]Block, map[string]target, bool, error) {
	blocks, targets, unsupported, err := inspectDocument(root, sessionID, revision, p.styles)
	if err != nil {
		return nil, nil, false, err
	}
	// Resolve the document relationship only for reference types present in
	// candidate paragraphs. Movement leaves those package parts intact.
	available := map[string]map[int64]bool{}
	for _, block := range blocks {
		paragraph := block.Paragraph
		if paragraph == nil || !(paragraph.MoveNoteUp || paragraph.MoveNoteDown) {
			continue
		}
		move := targets[paragraph.ID]
		for _, run := range move.paragraph.children {
			ref := plainBodyNoteReference(run)
			if ref == nil {
				continue
			}
			kind := strings.TrimSuffix(ref.name.Local, "Reference")
			if available[kind] == nil {
				available[kind] = p.movableNotes(root.child("body"), kind)
			}
			id, _ := strconv.ParseInt(ref.attr("id"), 10, 32)
			if !available[kind][id] {
				paragraph.MoveNoteUp, paragraph.MoveNoteDown = false, false
				delete(targets, paragraph.ID)
				break
			}
		}
	}
	return blocks, targets, unsupported, nil
}

func (p *documentPackage) movableNotes(body *node, kind string) map[int64]bool {
	available := map[int64]bool{}
	_, notes, err := p.notesPart(kind, nil)
	if err != nil || notes == nil {
		return available
	}
	incoming := noteReferenceCounts(body, kind)
	for _, note := range notes.children {
		if !note.is(kind) {
			continue
		}
		id, err := strconv.ParseInt(note.attr("id"), 10, 32)
		if err == nil && incoming[id] == 1 {
			_, seen := available[id]
			noteType := note.attr("type")
			available[id] = !seen && (noteType == "" || noteType == "normal")
		}
	}
	return available
}
