package docx

import (
	"bytes"
	"fmt"
	"strings"
)

func markBookmarkMoves(root *node, paragraphs map[*node]*Paragraph, protected map[*node]bool, targets map[string]target) {
	if !root.has("bookmarkStart") {
		return
	}
	index := readBookmarks(root)
	body := root.child("body")
	candidates := map[*node]bool{}
	bookmarked := map[*node]bool{}
	for p, snapshot := range paragraphs {
		if protected[p] || snapshot.PageBreakBefore || snapshot.PageBreakAfter ||
			strings.TrimSpace(p.text.String()) != "" {
			continue
		}
		if snapshot.StructureEditable {
			candidates[p] = true
			continue
		}
		if !structuralParagraphContents(p, root.source, true, nil) {
			continue
		}
		hasBookmark, safe := false, true
		for _, node := range p.children {
			id := node.attr("id")
			switch {
			case node.is("bookmarkStart"):
				hasBookmark = true
				safe = safe && index.safeStart(node) && index.ends[id][0].parent == p
			case node.is("bookmarkEnd"):
				safe = safe && len(index.starts[id]) == 1 && index.starts[id][0].parent == p && index.safeStart(index.starts[id][0])
			}
		}
		for _, run := range snapshot.Runs {
			safe = safe && run.Editable
		}
		if hasBookmark && safe {
			candidates[p] = true
			bookmarked[p] = true
		}
	}
	for i, p := range body.children {
		if !bookmarked[p] {
			continue
		}
		var before, after *node
		if i > 0 && candidates[body.children[i-1]] {
			before = body.children[i-1]
		}
		if i+1 < len(body.children) && candidates[body.children[i+1]] {
			after = body.children[i+1]
		}
		paragraph := paragraphs[p]
		paragraph.MoveBookmarkUp, paragraph.MoveBookmarkDown = before != nil, after != nil
		if before != nil || after != nil {
			targets[paragraph.ID] = target{kind: "move_bookmark", paragraph: p, moveUp: before, moveDown: after}
		}
	}
}

func moveBookmarkedParagraph(data []byte, target target, operation string) (replacement, error) {
	if target.kind != "move_bookmark" {
		return replacement{}, fmt.Errorf("DOCX bookmark movement requires a supported body paragraph")
	}
	first, second := target.paragraph, target.moveDown
	if operation == "move_bookmark_up" {
		first, second = target.moveUp, target.paragraph
	}
	if first == nil || second == nil || first.end > second.start {
		return replacement{}, fmt.Errorf("protected DOCX bookmark movement boundary")
	}
	// Both paragraphs retain their original parent and namespace context.
	raw := bytes.Join([][]byte{data[second.start:second.end], data[first.end:second.start], data[first.start:first.end]}, nil)
	return replacement{first.start, second.end, raw}, nil
}
