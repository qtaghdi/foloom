package docx

import (
	"fmt"
	"strconv"
	"strings"
)

func (p *documentPackage) notesPart(kind string, changes map[string][]byte) (string, *node, error) {
	data := p.partData(documentRelationships, changes)
	if len(data) == 0 {
		return "", nil, nil
	}
	rels, err := parseXML(data)
	if err != nil || rels.name.Space != packageRelNS || rels.name.Local != "Relationships" {
		return "", nil, fmt.Errorf("invalid DOCX document relationships")
	}
	var filename string
	for _, rel := range rels.children {
		if rel.name.Space != packageRelNS || rel.name.Local != "Relationship" ||
			attribute(rel, "", "Type") != officeRelNS+"/"+kind+"s" {
			continue
		}
		id := attribute(rel, "", "Id")
		part := p.relatedPart(id, kind+"s", changes)
		mode, target := attribute(rel, "", "TargetMode"), attribute(rel, "", "Target")
		if filename != "" || id == "" || part == "" || target == "" || strings.ContainsAny(target, "\\?#:") ||
			mode != "" && mode != "Internal" {
			return "", nil, fmt.Errorf("ambiguous DOCX %ss relationship", kind)
		}
		filename = part
	}
	if filename == "" {
		return "", nil, nil
	}
	notes, err := parseXML(p.partData(filename, changes))
	if err != nil || !notes.is(kind+"s") {
		return "", nil, fmt.Errorf("invalid DOCX %ss part", kind)
	}
	return filename, notes, nil
}

func (p *documentPackage) appendNote(kind string, paragraph *node, text string, changes map[string][]byte) (string, error) {
	if !validInlineText(text) || strings.TrimSpace(text) == "" || len(text) > 4000 {
		return "", fmt.Errorf("invalid DOCX %s text", kind)
	}
	filename, notes, err := p.notesPart(kind, changes)
	if err != nil {
		return "", err
	}
	highest := int64(0)
	seen := map[int64]bool{}
	if notes != nil {
		for _, note := range notes.children {
			if !note.is(kind) {
				continue
			}
			id, err := strconv.ParseInt(note.attr("id"), 10, 32)
			if err != nil || seen[id] {
				return "", fmt.Errorf("invalid or duplicate DOCX %s identifier", kind)
			}
			seen[id] = true
			highest = max(highest, id)
		}
	}
	// Reserve dangling references too so adding a note never changes their meaning.
	var reserveReferences func(*node)
	reserveReferences = func(n *node) {
		if n.is(kind+"Reference") || n.is(kind) {
			if id, err := strconv.ParseInt(n.attr("id"), 10, 32); err == nil {
				highest = max(highest, id)
			}
		}
		for _, child := range n.children {
			reserveReferences(child)
		}
	}
	reserveReferences(paragraph.parent)
	if _, _, root, err := p.pageSettings(changes); err == nil && root != nil {
		reserveReferences(root)
	}
	if highest >= 2147483647 || len(seen) >= 500 {
		return "", fmt.Errorf("DOCX %ss limit exceeded", kind)
	}
	raw := p.partData(filename, changes)
	if notes == nil {
		filename, err = p.addWordPart(changes, kind+"s", "rIdFoloom"+strings.ToUpper(kind[:1])+kind[1:]+"s")
		if err != nil {
			return "", err
		}
		raw = []byte(fmt.Sprintf(`<w:%[1]ss xmlns:w="%[2]s"><w:%[1]s w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:%[1]s><w:%[1]s w:type="continuationSeparator" w:id="0"><w:p><w:r><w:continuationSeparator/></w:r></w:p></w:%[1]s></w:%[1]ss>`, kind, wordNS))
	}
	id := strconv.FormatInt(highest+1, 10)
	note := `<dwn:` + kind + ` xmlns:dwn="` + wordNS + `" dwn:id="` + id + `"><dwn:p><dwn:r><dwn:rPr><dwn:vertAlign dwn:val="superscript"/></dwn:rPr><dwn:` + kind + `Ref/></dwn:r><dwn:r>` +
		inlineXML(" "+text) + `</dwn:r></dwn:p></dwn:` + kind + `>`
	raw, err = appendXMLChild(raw, note)
	if err != nil {
		return "", err
	}
	changes[filename] = raw
	return `<dwn:r xmlns:dwn="` + wordNS + `"><dwn:rPr><dwn:vertAlign dwn:val="superscript"/></dwn:rPr><dwn:` + kind + `Reference dwn:id="` + id + `"/></dwn:r>`, nil
}

func footnoteTextTarget(note *node, source []byte) *node {
	if kind := note.attr("type"); kind != "" && kind != "normal" {
		return nil
	}
	if len(note.children) != 1 || !note.children[0].is("p") ||
		strings.Contains(string(source[note.start:note.end]), "<!--") {
		return nil
	}
	paragraph := note.children[0]
	runs := paragraph.children
	if len(runs) > 0 && runs[0].is("pPr") {
		runs = runs[1:]
	}
	if len(runs) != 2 || !runs[0].is("r") || !runs[1].is("r") {
		return nil
	}
	mark := runs[0].child("footnoteRef")
	text := runs[1].child("t")
	if mark == nil || text == nil || len(text.children) != 0 || !plainTextAttributes(text) ||
		!validText(text.text.String()) || len(text.text.String()) > 1024 {
		return nil
	}
	if len(runs[0].children) != 1 && (len(runs[0].children) != 2 || runs[0].child("rPr") == nil) ||
		len(runs[1].children) != 1 && (len(runs[1].children) != 2 || runs[1].child("rPr") == nil) {
		return nil
	}
	for _, name := range []string{"ins", "del", "rPrChange", "pPrChange"} {
		if note.has(name) {
			return nil
		}
	}
	return text
}

func (p *documentPackage) changeFootnote(t target, edit ReferenceEdit, changes map[string][]byte) error {
	if t.kind != "footnote" || edit.URL != "" || edit.Bookmark != "" ||
		!validText(edit.Text) || strings.TrimSpace(edit.Text) == "" || len(edit.Text) > 1024 {
		return fmt.Errorf("invalid DOCX footnote edit")
	}
	id := t.node.child("footnoteReference").attr("id")
	filename, notes, err := p.notesPart("footnote", changes)
	if err != nil || notes == nil {
		return fmt.Errorf("DOCX footnotes part unavailable")
	}
	var text *node
	seen := false
	for _, note := range notes.children {
		if note.is("footnote") && note.attr("id") == id {
			if seen {
				return fmt.Errorf("ambiguous DOCX footnote")
			}
			seen = true
			text = footnoteTextTarget(note, notes.source)
		}
	}
	if text == nil {
		return fmt.Errorf("protected DOCX footnote cannot be changed")
	}
	prefix := ""
	if strings.HasPrefix(text.text.String(), " ") {
		prefix = " "
	}
	raw := append(append([]byte(nil), notes.source[:text.start]...), inlineXML(prefix+strings.TrimSpace(edit.Text))...)
	changes[filename] = append(raw, notes.source[text.end:]...)
	return nil
}

func (p *documentPackage) deleteNote(kind string, t target, edit ReferenceEdit, changes map[string][]byte) ([]replacement, error) {
	if t.kind != kind || edit.Text != "" || edit.URL != "" || edit.Bookmark != "" ||
		!onlyNamespaceAttributes(t.node) || strings.Contains(string(p.document[t.node.start:t.node.end]), "<!--") {
		return nil, fmt.Errorf("invalid DOCX %s removal", kind)
	}
	id, err := strconv.ParseInt(t.node.child(kind+"Reference").attr("id"), 10, 32)
	if err != nil || id < 1 {
		return nil, fmt.Errorf("invalid DOCX %s identifier", kind)
	}
	matches := func(n *node) bool {
		found, err := strconv.ParseInt(n.attr("id"), 10, 32)
		return err == nil && found == id
	}
	body := t.paragraph.parent
	if noteReferenceCounts(body, kind)[id] != 1 {
		return nil, fmt.Errorf("shared DOCX %s cannot be removed", kind)
	}
	filename, root, err := p.notesPart(kind, changes)
	if err != nil || root == nil {
		return nil, fmt.Errorf("DOCX %ss part unavailable", kind)
	}
	var note *node
	for _, child := range root.children {
		if child.is(kind) && matches(child) {
			if note != nil {
				return nil, fmt.Errorf("ambiguous DOCX %s", kind)
			}
			note = child
		}
	}
	if note == nil || !editableNote(kind, note, root.source) ||
		kind == "endnote" && !endnoteDeletionAllowed(t, endnoteBookmarkSpans(body)) {
		return nil, fmt.Errorf("protected DOCX %s cannot be removed", kind)
	}
	raw := append([]byte(nil), root.source[:note.start]...)
	changes[filename] = append(raw, root.source[note.end:]...)
	return []replacement{{start: t.node.start, end: t.node.end}}, nil
}

func noteReferenceCounts(body *node, kind string) map[int64]int {
	counts := map[int64]int{}
	var visit func(*node)
	visit = func(n *node) {
		if n.is(kind + "Reference") {
			if id, err := strconv.ParseInt(n.attr("id"), 10, 32); err == nil {
				counts[id]++
			}
		}
		for _, child := range n.children {
			visit(child)
		}
	}
	visit(body)
	return counts
}

func editableNote(kind string, note *node, source []byte) bool {
	if kind == "endnote" {
		nodes, _ := plainEndnoteText(note, source)
		return len(nodes) != 0
	}
	return footnoteTextTarget(note, source) != nil
}

func (p *documentPackage) projectNotes(kind string, body *node, blocks []Block, targets map[string]target) {
	_, notes, err := p.notesPart(kind, nil)
	if err != nil || notes == nil {
		return
	}
	textByID := map[string]string{}
	editableByID := map[string]bool{}
	for _, note := range notes.children {
		if !note.is(kind) {
			continue
		}
		id := note.attr("id")
		if _, seen := textByID[id]; seen {
			textByID[id] = ""
			delete(editableByID, id)
			continue
		}
		textByID[id] = ""
		if kind := note.attr("type"); id == "" || (kind != "" && kind != "normal") {
			continue
		}
		var lines []string
		for _, child := range note.children {
			if child.is("p") {
				lines = append(lines, strings.TrimSpace(textContent(child)))
			}
		}
		textByID[id] = reviewText(strings.Join(lines, "\n"), 1024)
		editableByID[id] = editableNote(kind, note, notes.source)
	}
	var incoming map[int64]int
	var bookmarks []endnoteBookmarkSpan
	if kind == "endnote" {
		incoming = noteReferenceCounts(body, kind)
		bookmarks = endnoteBookmarkSpans(body)
	}
	for _, block := range blocks {
		if block.Paragraph == nil {
			continue
		}
		for _, run := range block.Paragraph.Runs {
			if run.Object != nil && run.Object.Kind == kind {
				run.Object.Text = textByID[run.Object.noteID]
				t := targets[run.ID]
				run.Object.Editable = t.kind == kind && editableByID[run.Object.noteID]
				if kind == "endnote" && run.Object.Editable {
					id, err := strconv.ParseInt(run.Object.noteID, 10, 32)
					run.Object.DeleteEditable = err == nil && id > 0 && incoming[id] == 1 && endnoteDeletionAllowed(t, bookmarks) &&
						onlyNamespaceAttributes(t.node) && !strings.Contains(string(p.document[t.node.start:t.node.end]), "<!--")
				}
			}
		}
	}
}
