package docx

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var bookmarkName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,39}$`)

type ReferenceEdit struct {
	Action   string `json:"action" jsonschema:"insert_link, insert_bookmark, set_link_url, or unlink"`
	Text     string `json:"text,omitempty"`
	URL      string `json:"url,omitempty"`
	Bookmark string `json:"bookmark,omitempty"`
}

type BookmarkInfo struct {
	Name        string `json:"name"`
	ParagraphID string `json:"paragraphId"`
}

type LinkInfo struct {
	URL      string `json:"url,omitempty"`
	Bookmark string `json:"bookmark,omitempty"`
}

type bookmarkIndex struct {
	names   map[string]int
	starts  map[string][]*node
	ends    map[string][]*node
	highest int
}

func readBookmarks(root *node) bookmarkIndex {
	index := bookmarkIndex{names: map[string]int{}, starts: map[string][]*node{}, ends: map[string][]*node{}}
	var walk func(*node)
	walk = func(n *node) {
		if n.is("bookmarkStart") || n.is("bookmarkEnd") {
			id := attribute(n, wordNS, "id")
			if number, err := strconv.Atoi(id); err == nil && number > index.highest {
				index.highest = number
			}
			if n.is("bookmarkStart") {
				index.starts[id] = append(index.starts[id], n)
				index.names[strings.ToLower(attribute(n, wordNS, "name"))]++
			} else {
				index.ends[id] = append(index.ends[id], n)
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
	return index
}

func (index bookmarkIndex) safeStart(n *node) bool {
	id, name := attribute(n, wordNS, "id"), attribute(n, wordNS, "name")
	start, end := index.starts[id], index.ends[id]
	return id != "" && bookmarkName.MatchString(name) && index.names[strings.ToLower(name)] == 1 && len(start) == 1 && len(end) == 1 && n.start < end[0].start
}

func safeLink(raw string) bool {
	if len(raw) == 0 || len(raw) > 2048 || strings.ContainsAny(raw, "\\\t\r\n\x00 ") {
		return false
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	u, err := url.Parse(raw)
	return err == nil && u != nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.Opaque == ""
}

func (p *documentPackage) addHyperlink(raw string, changes map[string][]byte) (string, error) {
	rels := p.partData(documentRelationships, changes)
	if len(rels) == 0 {
		rels = []byte(`<Relationships xmlns="` + packageRelNS + `"></Relationships>`)
	}
	root, err := parseXML(rels)
	if err != nil {
		return "", err
	}
	used := map[string]bool{}
	for _, rel := range root.children {
		used[attribute(rel, "", "Id")] = true
	}
	id := "rIdFoloomLink"
	for i := 1; used[id]; i++ {
		id = "rIdFoloomLink" + strconv.Itoa(i)
	}
	xml := `<dwr:Relationship xmlns:dwr="` + packageRelNS + `" Id="` + id + `" Type="` + officeRelNS + `/hyperlink" Target="` + escapeText(raw) + `" TargetMode="External"/>`
	rels, err = appendXMLChild(rels, xml)
	if err != nil {
		return "", err
	}
	changes[documentRelationships] = rels
	return id, nil
}

func (p *documentPackage) referenceEdits(data []byte, t target, edit ReferenceEdit, bookmarks *bookmarkIndex, changes map[string][]byte) ([]replacement, error) {
	if edit.Action == "set_link_url" {
		return p.changeWebLink(data, t, edit, changes)
	}
	if edit.Action == "unlink" {
		return unlinkReference(data, t, edit)
	}
	paragraph := t.paragraph
	if t.kind != "replace_paragraphs" || paragraph == nil || !paragraph.parent.is("body") {
		return nil, fmt.Errorf("DOCX reference requires an editable body paragraph")
	}
	appendRun := func(raw string) []replacement {
		if strings.HasSuffix(string(data[paragraph.start:paragraph.inner]), "/>") {
			open, close := scopedOpening(paragraph, data, false)
			return []replacement{{paragraph.start, paragraph.end, []byte(open + raw + close)}}
		}
		return []replacement{{paragraph.endStart, paragraph.endStart, []byte(raw)}}
	}
	switch edit.Action {
	case "insert_bookmark":
		key := strings.ToLower(edit.Bookmark)
		if edit.Text != "" || edit.URL != "" || !bookmarkName.MatchString(edit.Bookmark) || bookmarks.names[key] != 0 || bookmarks.highest >= 2147483647 {
			return nil, fmt.Errorf("invalid or duplicate DOCX bookmark")
		}
		bookmarks.highest++
		id := strconv.Itoa(bookmarks.highest)
		bookmarks.names[key] = 1
		open := `<dwb:bookmarkStart xmlns:dwb="` + wordNS + `" dwb:id="` + id + `" dwb:name="` + edit.Bookmark + `"/>`
		close := `<dwb:bookmarkEnd xmlns:dwb="` + wordNS + `" dwb:id="` + id + `"/>`
		return appendRun(open + close), nil
	case "insert_link":
		if !validText(edit.Text) || len(edit.Text) > 4096 || strings.TrimSpace(edit.Text) == "" || (edit.URL == "") == (edit.Bookmark == "") {
			return nil, fmt.Errorf("invalid DOCX link text or target")
		}
		property := ""
		if edit.URL != "" {
			if !safeLink(edit.URL) {
				return nil, fmt.Errorf("unsupported DOCX link URL")
			}
			id, err := p.addHyperlink(edit.URL, changes)
			if err != nil {
				return nil, err
			}
			property = ` r:id="` + id + `"`
		} else {
			if !bookmarkName.MatchString(edit.Bookmark) || !bookmarks.hasTarget(edit.Bookmark) {
				return nil, fmt.Errorf("DOCX link destination does not exist")
			}
			property = ` dwb:anchor="` + edit.Bookmark + `"`
		}
		return appendRun(`<dwb:hyperlink xmlns:dwb="` + wordNS + `" xmlns:r="` + officeRelNS + `"` + property + `><dwb:r><dwb:rPr><dwb:color dwb:val="0563C1"/><dwb:u dwb:val="single"/></dwb:rPr><dwb:t xml:space="preserve">` + escapeText(edit.Text) + `</dwb:t></dwb:r></dwb:hyperlink>`), nil
	default:
		return nil, fmt.Errorf("unsupported DOCX reference action")
	}
}

func (p *documentPackage) changeWebLink(data []byte, target target, edit ReferenceEdit, changes map[string][]byte) ([]replacement, error) {
	link := target.node
	if target.kind != "hyperlink" || link == nil || !link.parent.is("p") || edit.Text != "" || edit.Bookmark != "" ||
		attribute(link, officeRelNS, "id") == "" || attribute(link, wordNS, "anchor") != "" || !safeLink(edit.URL) {
		return nil, fmt.Errorf("invalid DOCX web link update")
	}
	if protectedLink(link) {
		return nil, fmt.Errorf("protected DOCX web link cannot be changed")
	}

	if len(link.children) == 0 {
		return nil, fmt.Errorf("DOCX web link has no displayed content")
	}
	relationship, err := p.addHyperlink(edit.URL, changes)
	if err != nil {
		return nil, err
	}
	opening, _ := scopedOpening(link, data, false)
	opening, err = patchOpeningAttributes(opening, officeRelNS, map[string]string{"id": relationship}, nil)
	if err != nil {
		return nil, err
	}
	return []replacement{{start: link.start, end: link.inner, data: []byte(opening)}}, nil
}

func protectedLink(link *node) bool {
	for _, name := range []string{"fldChar", "instrText", "fldSimple", "ins", "del", "moveFrom", "moveTo", "sdt", "rPrChange", "pPrChange"} {
		if link.parent.has(name) {
			return true
		}
	}
	for parent := link.parent; parent != nil; parent = parent.parent {
		if parent.is("ins") || parent.is("del") || parent.is("moveFrom") || parent.is("moveTo") || parent.is("sdt") {
			return true
		}
	}
	return false
}

func unlinkReference(data []byte, target target, edit ReferenceEdit) ([]replacement, error) {
	link := target.node
	if target.kind != "hyperlink" || link == nil || !link.parent.is("p") ||
		edit.Text != "" || edit.URL != "" || edit.Bookmark != "" ||
		len(link.children) == 0 || strings.TrimSpace(link.text.String()) != "" {
		return nil, fmt.Errorf("invalid DOCX link removal")
	}
	if protectedLink(link) {
		return nil, fmt.Errorf("protected DOCX link cannot be removed")
	}
	for _, a := range link.attrs {
		if isNamespaceAttribute(a) || a.Name.Space == officeRelNS && a.Name.Local == "id" {
			continue
		}
		switch a.Name.Local {
		case "anchor", "history", "tooltip", "tgtFrame", "docLocation":
			if a.Name.Space == wordNS {
				continue
			}
		}
		return nil, fmt.Errorf("protected DOCX link metadata")
	}
	var out strings.Builder
	start := link.inner
	for _, run := range link.children {
		if !run.is("r") || strings.HasSuffix(string(data[run.start:run.inner]), "/>") {
			return nil, fmt.Errorf("DOCX link has protected content")
		}
		for _, child := range run.children {
			if !child.is("rPr") && !child.is("t") && !plainLineBreak(child) && !plainTab(child) {
				return nil, fmt.Errorf("DOCX link has protected content")
			}
		}
		if run.has("rPrChange") {
			return nil, fmt.Errorf("DOCX link has tracked formatting")
		}
		// Runs can inherit prefixes defined on the hyperlink being removed.
		open, close := scopedOpening(run, data, false)
		out.Write(data[start:run.start])
		out.WriteString(open)
		out.Write(data[run.inner:run.endStart])
		out.WriteString(close)
		start = run.end
	}
	out.Write(data[start:link.endStart])
	return []replacement{{start: link.start, end: link.end, data: []byte(out.String())}}, nil
}

func (index bookmarkIndex) hasTarget(name string) bool {
	for _, starts := range index.starts {
		if len(starts) == 1 && attribute(starts[0], wordNS, "name") == name && index.safeStart(starts[0]) {
			return true
		}
	}
	return false
}

func (p *documentPackage) projectReferences(root *node, blocks []Block, targets map[string]target) []BookmarkInfo {
	bookmarks := []BookmarkInfo{}
	if !bytes.Contains(p.document, []byte("hyperlink")) && !bytes.Contains(p.document, []byte("bookmarkStart")) {
		return bookmarks
	}
	index := readBookmarks(root)
	names := map[string]bool{}
	blockIndex := 0
	for _, child := range root.child("body").children {
		if !child.is("p") && !child.is("tbl") {
			continue
		}
		block := &blocks[blockIndex]
		blockIndex++
		if block.Paragraph == nil {
			continue
		}
		for _, marker := range child.children {
			if len(bookmarks) >= 500 {
				break
			}
			if marker.is("bookmarkStart") && index.safeStart(marker) {
				name := attribute(marker, wordNS, "name")
				names[name] = true
				bookmarks = append(bookmarks, BookmarkInfo{Name: name, ParagraphID: block.Paragraph.ID})
			}
		}
	}
	var external map[string]string
	readExternal := func() {
		external = make(map[string]string)
		rels, err := parseXML(p.partData(documentRelationships, nil))
		if err != nil {
			return
		}
		for _, rel := range rels.children {
			if rel.name.Space == packageRelNS && rel.name.Local == "Relationship" && attribute(rel, "", "Type") == officeRelNS+"/hyperlink" && attribute(rel, "", "TargetMode") == "External" {
				raw := attribute(rel, "", "Target")
				if safeLink(raw) {
					external[attribute(rel, "", "Id")] = raw
				}
			}
		}
	}
	project := func(paragraph *Paragraph) {
		for i := range paragraph.Runs {
			run := &paragraph.Runs[i]
			t, ok := targets[run.ID]
			if !ok || t.kind != "hyperlink" || strings.TrimSpace(run.Text) == "" {
				continue
			}
			id, anchor := attribute(t.node, officeRelNS, "id"), attribute(t.node, wordNS, "anchor")
			if id != "" && anchor == "" {
				if external == nil {
					readExternal()
				}
				if raw := external[id]; raw != "" {
					run.Link = &LinkInfo{URL: raw}
				}
			} else if id == "" && names[anchor] {
				run.Link = &LinkInfo{Bookmark: anchor}
			}
		}
	}
	for i := range blocks {
		block := &blocks[i]
		if block.Paragraph != nil {
			project(block.Paragraph)
		}
		for j := range block.Rows {
			for k := range block.Rows[j].Cells {
				for l := range block.Rows[j].Cells[k].Paragraphs {
					project(&block.Rows[j].Cells[k].Paragraphs[l])
				}
			}
		}
	}
	return bookmarks
}
