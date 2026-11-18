package docx

import (
	"fmt"
	"strings"
)

func plainHyperlinkRun(run *node) bool {
	if !run.is("r") || run.has("rPrChange") {
		return false
	}
	for _, child := range run.children {
		if !child.is("rPr") && !child.is("t") && !plainLineBreak(child) && !plainTab(child) {
			return false
		}
	}
	return true
}

func changeBookmarkLink(data []byte, target target, edit ReferenceEdit, bookmarks *bookmarkIndex) ([]replacement, error) {
	if target.kind != "hyperlink" || !target.linkAnchorEditable || edit.Text != "" || edit.URL != "" ||
		!bookmarkName.MatchString(edit.Bookmark) {
		return nil, fmt.Errorf("invalid DOCX bookmark link update")
	}
	link := target.node
	original := bookmarks.bodyBookmark(link.attr("anchor"))
	updated := bookmarks.bodyBookmark(edit.Bookmark)
	if original == nil || updated == nil {
		return nil, fmt.Errorf("DOCX bookmark link target does not exist")
	}
	if original == updated {
		return nil, nil
	}
	opening, _ := scopedOpening(link, data, false)
	opening, err := patchOpeningAttributes(opening, wordNS, map[string]string{"anchor": updated.attr("name")}, nil)
	if err != nil {
		return nil, err
	}
	// Keep the original link contents and closing tag. A bookmark link has no
	// relationship part to rewrite.
	return []replacement{{start: link.start, end: link.inner, data: []byte(opening)}}, nil
}

func (index bookmarkIndex) bodyBookmark(name string) *node {
	key := strings.ToLower(name)
	if index.names[key] != 1 {
		return nil
	}
	for _, starts := range index.starts {
		if len(starts) != 1 {
			continue
		}
		start := starts[0]
		bookmarked := start.attr("name")
		if strings.EqualFold(bookmarked, name) && start.parent.is("p") && start.parent.parent.is("body") && index.safeStart(start) {
			return start
		}
	}
	return nil
}
