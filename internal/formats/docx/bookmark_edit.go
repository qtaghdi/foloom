package docx

import (
	"fmt"
	"strings"
)

func (p *documentPackage) editBookmark(data []byte, t target, edit ReferenceEdit, index *bookmarkIndex) ([]replacement, error) {
	deleting := edit.Action == "delete_bookmark"
	if t.kind != "bookmark_paragraph" || edit.Text != "" || edit.URL != "" || !bookmarkName.MatchString(edit.Bookmark) ||
		deleting && edit.NewBookmark != "" || !deleting && (!bookmarkName.MatchString(edit.NewBookmark) || edit.NewBookmark == edit.Bookmark) {
		return nil, fmt.Errorf("invalid DOCX bookmark edit")
	}
	if !deleting && !strings.EqualFold(edit.Bookmark, edit.NewBookmark) && index.names[strings.ToLower(edit.NewBookmark)] != 0 {
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
	var edits []replacement
	display := ""
	if deleting {
		display = bookmarkedPlainText(index, start)
		end := index.ends[start.attr("id")][0]
		edits = []replacement{{start: start.start, end: start.end}, {start: end.start, end: end.end}}
	} else {
		raw, err := patchScalarProperty(start, data, "bookmarkStart", wordNS, map[string]string{"name": edit.NewBookmark}, nil)
		if err != nil {
			return nil, err
		}
		edits = []replacement{{start.start, start.end, []byte(raw)}}
	}
	var walk func(*node, bool) error
	walk = func(n *node, main bool) error {
		switch {
		case n.is("fldChar") || n.is("instrText") || n.is("delInstrText"):
			return fmt.Errorf("DOCX bookmark edit requires a document without opaque fields")
		case n.is("fldSimple"):
			bookmark := bookmarkFieldName(n)
			if bookmark == "" && pageFieldInstruction(n) == "" && !supportedTOCField(n) {
				return fmt.Errorf("DOCX bookmark edit requires a document without opaque fields")
			}
			if bookmark != "" && strings.EqualFold(bookmark, edit.Bookmark) {
				if !main {
					return fmt.Errorf("protected DOCX bookmark field outside body")
				}
				result, err := patchBookmarkField(data, n, deleting, edit.NewBookmark, display)
				if err != nil {
					return err
				}
				edits = append(edits, result)
			}
		case !main && n.is("bookmarkStart") &&
			(strings.EqualFold(n.attr("name"), edit.Bookmark) || !deleting && strings.EqualFold(n.attr("name"), edit.NewBookmark)):
			return fmt.Errorf("DOCX bookmark name exists in another story")
		case n.is("hyperlink") && strings.EqualFold(n.attr("anchor"), edit.Bookmark):
			if attribute(n, officeRelNS, "id") == "" {
				if !main || n.attr("docLocation") != "" || protectedLink(n) {
					return fmt.Errorf("protected DOCX bookmark reference")
				}
				if deleting {
					unlinked, err := unlinkReference(data, target{kind: "hyperlink", node: n}, ReferenceEdit{Action: "unlink"})
					if err != nil {
						return err
					}
					edits = append(edits, unlinked...)
					return nil
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
