package docx

import (
	"fmt"
	"strconv"
	"strings"
	"time"
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
	date := ""
	if parsed, err := time.Parse(time.RFC3339Nano, n.attr("date")); err == nil {
		date = parsed.UTC().Format(time.RFC3339)
	}
	return &ObjectInfo{Kind: kind, Text: reviewText(text.String(), 1024), Author: reviewText(n.attr("author"), 160), Date: date}
}

func revisionReplacement(data []byte, t target, operation string) (replacement, error) {
	if t.kind == "tracked_format" {
		return formatRevisionReplacement(data, t, operation)
	}
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

func trackableRun(run *node, texts []*node, data []byte) bool {
	if len(texts) != 1 || !plainTextAttributes(texts[0]) ||
		(strings.TrimSpace(texts[0].text.String()) == "" || len(texts[0].text.String()) > 1024 ||
			!validText(texts[0].text.String())) ||
		(len(run.children) != 1 && (len(run.children) != 2 || !run.children[0].is("rPr"))) ||
		strings.Contains(string(data[run.start:run.end]), "<!--") {
		return false
	}
	for _, attr := range run.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || !strings.HasPrefix(attr.Name.Local, "rsid")) {
			return false
		}
	}
	return true
}

// Reserve Word IDs in existing stories before adding review annotations.
func (p *documentPackage) revisionIDs(root *node) map[int64]bool {
	used := map[int64]bool{}
	var visit func(*node)
	visit = func(n *node) {
		if id, err := strconv.ParseInt(n.attr("id"), 10, 32); err == nil {
			used[id] = true
		}
		for _, child := range n.children {
			visit(child)
		}
	}
	visit(root)
	for _, part := range p.parts {
		name := part.header.Name
		if name == "word/document.xml" || !strings.HasPrefix(name, "word/") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		other, err := parseXML(part.data)
		if err == nil && other.name.Space == wordNS {
			visit(other)
		}
	}
	return used
}

func trackedTextEdit(data []byte, t target, op Operation, author, date string, used map[int64]bool, nextID *int64) (replacement, error) {
	if !t.trackable || t.kind != "replace_run_text" || t.text != op.ExpectedText ||
		t.text == op.Text || len(op.Text) > 1024 || !validText(op.Text) {
		return replacement{}, fmt.Errorf("invalid or stale DOCX tracked text")
	}
	run := t.run
	open, close := scopedOpening(run, data, false)
	props := ""
	if node := run.child("rPr"); node != nil {
		props = string(data[node.start:node.end])
	}
	wrap := func(kind, value string) string {
		for used[*nextID] {
			(*nextID)++
		}
		id := *nextID
		used[id] = true
		return fmt.Sprintf(`<dwr:%s xmlns:dwr="%s" dwr:id="%d" dwr:author="%s" dwr:date="%s">%s%s%s%s</dwr:%s>`,
			kind, wordNS, id, author, date, open, props, value, close, kind)
	}
	value := wrap("del", `<dwr:delText xmlns:dwr="`+wordNS+`" xml:space="preserve">`+escapeText(t.text)+`</dwr:delText>`)
	if op.Text != "" {
		value += wrap("ins", inlineXML(op.Text))
	}
	return replacement{start: run.start, end: run.end, data: []byte(value)}, nil
}
