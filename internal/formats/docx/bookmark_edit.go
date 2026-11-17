package docx

import (
	"fmt"
	"strings"
)

func (p *documentPackage) renameBookmark(data []byte, t target, edit ReferenceEdit, index *bookmarkIndex) ([]replacement, error) {
	if t.kind != "bookmark_paragraph" || edit.Text != "" || edit.URL != "" ||
		!bookmarkName.MatchString(edit.Bookmark) || !bookmarkName.MatchString(edit.NewBookmark) || edit.NewBookmark == edit.Bookmark {
		return nil, fmt.Errorf("invalid DOCX bookmark rename")
	}
	if !strings.EqualFold(edit.Bookmark, edit.NewBookmark) && index.names[strings.ToLower(edit.NewBookmark)] != 0 {
		return nil, fmt.Errorf("duplicate DOCX bookmark name")
	}
	var start *node
	for _, marker := range t.paragraph.children {
		if marker.is("bookmarkStart") && marker.attr("name") == edit.Bookmark && index.safeStart(marker) {
			start = marker
			break
		}
	}
	if start == nil {
		return nil, fmt.Errorf("DOCX bookmark target has changed")
	}
	raw, err := patchScalarProperty(start, data, "bookmarkStart", wordNS, map[string]string{"name": edit.NewBookmark}, nil)
	if err != nil {
		return nil, err
	}
	edits := []replacement{{start.start, start.end, []byte(raw)}}
	var walk func(*node, bool) error
	walk = func(n *node, main bool) error {
		switch {
		case n.is("fldChar") || n.is("instrText") || n.is("delInstrText") ||
			n.is("fldSimple") && !strings.EqualFold(strings.TrimSpace(n.attr("instr")), "PAGE"):
			return fmt.Errorf("DOCX bookmark rename requires a document without opaque fields")
		case !main && n.is("bookmarkStart") &&
			(strings.EqualFold(n.attr("name"), edit.Bookmark) || strings.EqualFold(n.attr("name"), edit.NewBookmark)):
			return fmt.Errorf("DOCX bookmark name exists in another story")
		case n.is("hyperlink") && strings.EqualFold(n.attr("anchor"), edit.Bookmark):
			if attribute(n, officeRelNS, "id") == "" {
				if !main || n.attr("docLocation") != "" || protectedLink(n) {
					return fmt.Errorf("protected DOCX bookmark reference")
				}
				open, close := scopedOpening(n, data, false)
				open, err := patchOpeningAttributes(open, wordNS, map[string]string{"anchor": edit.NewBookmark}, nil)
				if err != nil {
					return err
				}
				content := ""
				if n.inner < n.endStart {
					content = string(data[n.inner:n.endStart])
				}
				edits = append(edits, replacement{n.start, n.end, []byte(open + content + close)})
			}
		}
		for _, child := range n.children {
			if err := walk(child, main); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(t.paragraph.parent.parent, true); err != nil {
		return nil, err
	}
	for _, part := range p.parts {
		name := part.header.Name
		if name == "word/document.xml" || !strings.HasSuffix(strings.ToLower(name), ".xml") {
			continue
		}
		root, err := parseXML(part.data)
		if err != nil {
			return nil, err
		}
		if err := walk(root, false); err != nil {
			return nil, err
		}
	}
	return edits, nil
}
