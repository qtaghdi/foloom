package docx

import (
	"fmt"
	"strconv"
	"strings"
)

func (p *documentPackage) footnotesPart(changes map[string][]byte) (string, *node, error) {
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
			attribute(rel, "", "Type") != officeRelNS+"/footnotes" {
			continue
		}
		id := attribute(rel, "", "Id")
		part := p.relatedPart(id, "footnotes", changes)
		mode, target := attribute(rel, "", "TargetMode"), attribute(rel, "", "Target")
		if filename != "" || id == "" || part == "" || target == "" || strings.ContainsAny(target, "\\?#:") ||
			mode != "" && mode != "Internal" {
			return "", nil, fmt.Errorf("ambiguous DOCX footnotes relationship")
		}
		filename = part
	}
	if filename == "" {
		return "", nil, nil
	}
	notes, err := parseXML(p.partData(filename, changes))
	if err != nil || !notes.is("footnotes") {
		return "", nil, fmt.Errorf("invalid DOCX footnotes part")
	}
	return filename, notes, nil
}

func (p *documentPackage) appendFootnote(paragraph *node, text string, changes map[string][]byte) (string, error) {
	if !validInlineText(text) || strings.TrimSpace(text) == "" || len(text) > 4000 {
		return "", fmt.Errorf("invalid DOCX footnote text")
	}
	filename, notes, err := p.footnotesPart(changes)
	if err != nil {
		return "", err
	}
	highest := int64(0)
	seen := map[int64]bool{}
	if notes != nil {
		for _, note := range notes.children {
			if !note.is("footnote") {
				continue
			}
			id, err := strconv.ParseInt(note.attr("id"), 10, 32)
			if err != nil || seen[id] {
				return "", fmt.Errorf("invalid or duplicate DOCX footnote identifier")
			}
			seen[id] = true
			highest = max(highest, id)
		}
	}
	// Reserve dangling references too so adding a note never changes their meaning.
	var reserveReferences func(*node)
	reserveReferences = func(n *node) {
		if n.is("footnoteReference") || n.is("footnote") {
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
		return "", fmt.Errorf("DOCX footnotes limit exceeded")
	}
	raw := p.partData(filename, changes)
	if notes == nil {
		filename, err = p.addWordPart(changes, "footnotes", "rIdFoloomFootnotes")
		if err != nil {
			return "", err
		}
		raw = []byte(`<w:footnotes xmlns:w="` + wordNS + `"><w:footnote w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:footnote><w:footnote w:type="continuationSeparator" w:id="0"><w:p><w:r><w:continuationSeparator/></w:r></w:p></w:footnote></w:footnotes>`)
	}
	id := strconv.FormatInt(highest+1, 10)
	note := `<dwn:footnote xmlns:dwn="` + wordNS + `" dwn:id="` + id + `"><dwn:p><dwn:r><dwn:rPr><dwn:vertAlign dwn:val="superscript"/></dwn:rPr><dwn:footnoteRef/></dwn:r><dwn:r>` +
		inlineXML(" "+text) + `</dwn:r></dwn:p></dwn:footnote>`
	raw, err = appendXMLChild(raw, note)
	if err != nil {
		return "", err
	}
	changes[filename] = raw
	return `<dwn:r xmlns:dwn="` + wordNS + `"><dwn:rPr><dwn:vertAlign dwn:val="superscript"/></dwn:rPr><dwn:footnoteReference dwn:id="` + id + `"/></dwn:r>`, nil
}

func (p *documentPackage) projectFootnotes(blocks []Block) {
	_, notes, err := p.footnotesPart(nil)
	if err != nil || notes == nil {
		return
	}
	textByID := map[string]string{}
	for _, note := range notes.children {
		if !note.is("footnote") {
			continue
		}
		id := note.attr("id")
		if _, seen := textByID[id]; seen {
			textByID[id] = ""
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
	}
	for _, block := range blocks {
		if block.Paragraph == nil {
			continue
		}
		for _, run := range block.Paragraph.Runs {
			if run.Object != nil && run.Object.Kind == "footnote" {
				run.Object.Text = textByID[run.Object.footnoteID]
			}
		}
	}
}
