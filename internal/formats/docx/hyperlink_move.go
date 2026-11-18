package docx

import "strings"

func markHyperlinkParagraphs(root *node, paragraphs map[*node]*Paragraph, protected map[*node]bool, targets map[string]target) {
	if !root.child("body").has("hyperlink") {
		return
	}
	markBodyParagraphMoves(root, paragraphs, protected, func(p *node) bool {
		return p.child("hyperlink") != nil && structuralParagraphContents(p, root.source, "hyperlink", nil)
	}, func(p, before, after *node) {
		if before == nil && after == nil {
			return
		}
		paragraph := paragraphs[p]
		paragraph.MoveHyperlinkUp, paragraph.MoveHyperlinkDown = before != nil, after != nil
		targets[paragraph.ID] = target{kind: "hyperlink_paragraph", paragraph: p, moveUp: before, moveDown: after}
	})
}

func markBodyParagraphMoves(root *node, paragraphs map[*node]*Paragraph, protected map[*node]bool, matches func(*node) bool, mark func(p, before, after *node)) {
	body := root.child("body")
	ranged, valid := bodyRangeBlocks(body)
	if !valid {
		return
	}
	safe := func(p *node) bool {
		paragraph := paragraphs[p]
		return paragraph != nil && !protected[p] && !ranged[p] && !paragraph.PageBreakBefore && !paragraph.PageBreakAfter &&
			strings.TrimSpace(p.text.String()) == ""
	}
	for i, p := range body.children {
		if !safe(p) || !matches(p) {
			continue
		}
		var before, after *node
		if i > 0 {
			previous := body.children[i-1]
			if safe(previous) && paragraphs[previous].StructureEditable && whitespaceBetween(root.source, previous, p) {
				before = previous
			}
		}
		if i+1 < len(body.children) {
			next := body.children[i+1]
			if safe(next) && paragraphs[next].StructureEditable && whitespaceBetween(root.source, p, next) {
				after = next
			}
		}
		mark(p, before, after)
	}
}

func whitespaceBetween(data []byte, first, second *node) bool {
	return strings.TrimSpace(string(data[first.end:second.start])) == ""
}

func allowPlainHyperlink(link *node, validRun func(*node) bool) bool {
	if !link.is("hyperlink") || len(link.children) == 0 || strings.TrimSpace(link.text.String()) != "" {
		return false
	}
	relationship := attribute(link, officeRelNS, "id")
	anchor := attribute(link, wordNS, "anchor")
	if (relationship == "") == (anchor == "") {
		return false
	}
	for _, a := range link.attrs {
		if !isNamespaceAttribute(a) && !(a.Name.Space == officeRelNS && a.Name.Local == "id") &&
			!(a.Name.Space == wordNS && strings.Contains(" anchor history tooltip tgtFrame ", " "+a.Name.Local+" ")) {
			return false
		}
	}
	for _, run := range link.children {
		if !validRun(run) {
			return false
		}
	}
	return true
}

// A range can begin and end in other blocks, leaving no visible marker on a
// paragraph it covers. Keep such blocks stationary even if both look ordinary.
func bodyRangeBlocks(body *node) (map[*node]bool, bool) {
	type rangeID struct{ kind, id string }
	active := map[rangeID]bool{}
	ranged := map[*node]bool{}
	valid := true
	var visit func(*node, *node)
	visit = func(n, block *node) {
		if n.name.Space == wordNS {
			kind, start := strings.CutSuffix(n.name.Local, "Start")
			if !start {
				kind, _ = strings.CutSuffix(n.name.Local, "End")
			}
			if (kind == "bookmark" || strings.HasSuffix(kind, "Range")) && kind != n.name.Local {
				id := rangeID{kind, n.attr("id")}
				ranged[block] = true
				if id.id == "" || active[id] == start {
					valid = false
				}
				if start {
					active[id] = true
				} else {
					delete(active, id)
				}
			}
		}
		for _, child := range n.children {
			visit(child, block)
		}
	}
	for _, block := range body.children {
		if len(active) != 0 {
			ranged[block] = true
		}
		visit(block, block)
	}
	return ranged, valid && len(active) == 0
}
