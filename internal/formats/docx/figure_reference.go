package docx

import (
	"fmt"
	"strconv"
	"strings"
)

const figureBookmarkPrefix = "FoloomFigure"

func isFigureBookmarkPair(start, end *node) bool {
	if !start.is("bookmarkStart") || !end.is("bookmarkEnd") || len(start.children) != 0 || len(end.children) != 0 ||
		strings.TrimSpace(start.text.String()) != "" || strings.TrimSpace(end.text.String()) != "" {
		return false
	}
	id := start.attr("id")
	number, err := strconv.ParseInt(id, 10, 32)
	if err != nil || number < 1 || end.attr("id") != id || start.attr("name") != figureBookmarkPrefix+id {
		return false
	}
	for _, marker := range []*node{start, end} {
		for _, attr := range marker.attrs {
			if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || attr.Name.Local != "id" &&
				(marker != start || attr.Name.Local != "name")) {
				return false
			}
		}
	}
	return true
}

func bookmarkedFigureNumber(index *bookmarkIndex, marker *node, data []byte) *node {
	if marker == nil || !marker.parent.is("p") || !marker.parent.parent.is("body") || !index.safeStart(marker) {
		return nil
	}
	children := marker.parent.children
	for i, child := range children {
		if child == marker && i+2 < len(children) && isFigureBookmarkPair(marker, children[i+2]) {
			return captionSequenceValue(children[i+1], data, supportedFigureSequence)
		}
	}
	return nil
}

func figureNumberBookmark(field *node) *node {
	children := field.parent.children
	for i, child := range children {
		if child == field && i > 0 && i+1 < len(children) && isFigureBookmarkPair(children[i-1], children[i+1]) {
			return children[i-1]
		}
	}
	return nil
}

func displayedBookmarkText(index *bookmarkIndex, marker *node, data []byte) string {
	if number := bookmarkedFigureNumber(index, marker, data); number != nil {
		return number.text.String()
	}
	return bookmarkedPlainText(index, marker)
}

func (p *documentPackage) insertFigureReference(destination, picture target, edit ReferenceEdit, index *bookmarkIndex) ([]replacement, error) {
	if destination.kind != "replace_paragraphs" || destination.paragraph == nil || !destination.paragraph.parent.is("body") ||
		edit.FigureID == "" || edit.NewBookmark != "" || edit.Bookmark != "" || edit.Text != "" || edit.URL != "" {
		return nil, fmt.Errorf("DOCX figure reference requires an editable body paragraph and a current picture")
	}
	if picture.kind != "picture" || picture.run == nil || !picture.paragraph.parent.is("body") {
		return nil, fmt.Errorf("DOCX figure reference requires an inline picture")
	}
	info, relation := inlinePictureInfo(picture.run)
	if info == nil {
		return nil, fmt.Errorf("DOCX figure reference requires an inline picture")
	}
	if _, _, err := imageConfig(p.partData(p.picturePartMap()[relation], nil), maxImageReadBytes); err != nil {
		return nil, err
	}
	next, eligible := captionFollowingPicture(picture)
	if !eligible {
		return nil, fmt.Errorf("DOCX figure reference requires an adjacent numbered caption")
	}
	caption := readFigureCaption(next, p.document)
	if _, err := documentFigureFields(picture.paragraph.parent); err != nil {
		return nil, err
	}
	if caption.number == nil {
		return nil, fmt.Errorf("DOCX figure reference requires a supported numbered caption")
	}
	var name string
	var edits []replacement
	if caption.bookmark != nil {
		if bookmarkedFigureNumber(index, caption.bookmark, p.document) == nil {
			return nil, fmt.Errorf("ambiguous DOCX picture caption bookmark")
		}
		name = caption.bookmark.attr("name")
	} else {
		if index.highest >= 2147483647 {
			return nil, fmt.Errorf("DOCX figure bookmarks exhausted")
		}
		id := strconv.Itoa(index.highest + 1)
		name = figureBookmarkPrefix + id
		if index.names[strings.ToLower(name)] != 0 {
			return nil, fmt.Errorf("DOCX figure bookmark name is already used")
		}
		field := caption.number.parent.parent
		start := `<dwb:bookmarkStart xmlns:dwb="` + wordNS + `" dwb:id="` + id + `" dwb:name="` + name + `"/>`
		end := `<dwb:bookmarkEnd xmlns:dwb="` + wordNS + `" dwb:id="` + id + `"/>`
		edits = append(edits, replacement{field.start, field.start, []byte(start)}, replacement{field.end, field.end, []byte(end)})
	}
	field := `<dwr:fldSimple xmlns:dwr="` + wordNS + `" dwr:instr="REF ` + name +
		` \h" dwr:dirty="true"><dwr:r>` + inlineXML(caption.number.text.String()) + `</dwr:r></dwr:fldSimple>`
	return append(edits, appendParagraphContent(p.document, destination.paragraph, field)), nil
}

func figureReferenceNumberEdits(body *node, data []byte, values []*node, at int) ([]replacement, error) {
	if !body.has("bookmarkStart") {
		return nil, nil
	}
	index := readBookmarks(body.parent)
	renumbered := map[string]string{}
	for i, value := range values {
		if value.start < at {
			continue
		}
		if bookmark := figureNumberBookmark(value.parent.parent); bookmark != nil {
			if !index.safeStart(bookmark) {
				return nil, fmt.Errorf("ambiguous DOCX picture caption bookmark")
			}
			renumbered[strings.ToLower(bookmark.attr("name"))] = strconv.Itoa(i + 2)
		}
	}
	var edits []replacement
	var walk func(*node) error
	walk = func(n *node) error {
		if n.is("fldSimple") {
			instruction := strings.Fields(n.attr("instr"))
			if len(instruction) >= 2 && strings.EqualFold(instruction[0], "REF") {
				if desired := renumbered[strings.ToLower(instruction[1])]; desired != "" {
					kind, _ := bookmarkFieldTarget(n)
					if kind != "REF" || !canRefreshBookmarkReference(n, data) {
						return fmt.Errorf("protected DOCX picture caption reference")
					}
					cache := n.children[0].child("t")
					if cache.text.String() != desired {
						edit, err := replaceTextNode(data, cache, desired)
						if err != nil {
							return err
						}
						edits = append(edits, edit)
					}
				}
			}
		}
		for _, child := range n.children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if len(renumbered) == 0 {
		return nil, nil
	}
	err := walk(body)
	return edits, err
}

// Only a validated bookmark that closes around the figure field travels with its caption.
func figureBookmarkMarkers(root *node) map[*node]bool {
	body := root.child("body")
	if !body.has("bookmarkStart") {
		return nil
	}
	index := readBookmarks(root)
	ignored := map[*node]bool{}
	for _, paragraph := range body.children {
		caption := readFigureCaption(paragraph, root.source)
		if caption.bookmark != nil && index.safeStart(caption.bookmark) {
			ignored[caption.bookmark] = true
			ignored[index.ends[caption.bookmark.attr("id")][0]] = true
		}
	}
	return ignored
}
