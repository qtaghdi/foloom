package docx

import (
	"fmt"
	"strconv"
	"strings"
)

func reviewTextRevision(n *node, data []byte) *ObjectInfo {
	if n == nil || (!n.is("ins") && !n.is("del")) || len(n.children) == 0 ||
		strings.TrimSpace(n.text.String()) != "" || strings.Contains(string(data[n.start:n.end]), "<!--") {
		return nil
	}
	id, err := strconv.ParseInt(n.attr("id"), 10, 32)
	if err != nil || id < 0 {
		return nil
	}
	for _, attr := range n.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS ||
			attr.Name.Local != "id" && attr.Name.Local != "author" && attr.Name.Local != "date") {
			return nil
		}
	}
	kind, textName := "tracked_insert", "t"
	if n.is("del") {
		kind, textName = "tracked_delete", "delText"
	}
	var text strings.Builder
	for _, run := range n.children {
		if !run.is("r") || strings.TrimSpace(run.text.String()) != "" {
			return nil
		}
		for _, attr := range run.attrs {
			if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || !strings.HasPrefix(attr.Name.Local, "rsid")) {
				return nil
			}
		}
		if countChild(run, "rPr") > 1 {
			return nil
		}
		for _, child := range run.children {
			if child.is("rPr") && child == run.children[0] && !child.has("rPrChange") {
				continue
			}
			if !child.is(textName) || len(child.children) != 0 || !plainTextAttributes(child) || !validText(child.text.String()) {
				return nil
			}
			text.WriteString(child.text.String())
		}
	}
	if text.Len() == 0 || text.Len() > maxTextBytes {
		return nil
	}
	return &ObjectInfo{Kind: kind, Text: reviewText(text.String(), 1024), Author: reviewText(n.attr("author"), 160)}
}

func revisionReplacement(data []byte, t target, operation string) (replacement, error) {
	n := t.node
	if t.kind != "tracked_text" || reviewTextRevision(n, data) == nil {
		return replacement{}, fmt.Errorf("protected DOCX tracked revision")
	}
	keepText := n.is("ins") == (operation == "accept_revision")
	edit := replacement{start: n.start, end: n.end}
	if !keepText {
		return edit, nil
	}
	var contents strings.Builder
	for _, run := range n.children {
		open, close := scopedOpening(run, data, false)
		contents.WriteString(open)
		if n.is("ins") {
			contents.Write(data[run.inner:run.endStart])
		} else {
			for _, child := range run.children {
				if child.is("delText") {
					contents.WriteString(inlineXML(child.text.String()))
				} else {
					contents.Write(data[child.start:child.end])
				}
			}
		}
		contents.WriteString(close)
	}
	edit.data = []byte(contents.String())
	return edit, nil
}
