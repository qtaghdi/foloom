package docx

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

func (p *documentPackage) contentControlIDs() (map[int64]bool, error) {
	used := map[int64]bool{}
	var visit func(*node)
	visit = func(n *node) {
		if n.is("sdtPr") && n.parent != nil && n.parent.is("sdt") {
			for _, property := range n.children {
				if property.is("id") {
					if id, err := strconv.ParseInt(property.attr("val"), 10, 32); err == nil {
						used[id] = true
					}
				}
			}
		}
		for _, child := range n.children {
			visit(child)
		}
	}
	// Headers, footers, and notes can also contain saved content controls.
	for _, item := range p.parts {
		if !strings.HasSuffix(strings.ToLower(item.header.Name), ".xml") || !bytes.Contains(item.data, []byte("sdtPr")) {
			continue
		}
		root, err := parseXML(item.data)
		if err != nil {
			return nil, err
		}
		visit(root)
	}
	return used, nil
}

func inlinePlainControl(id int64, text string) string {
	return fmt.Sprintf(`<dws:sdt xmlns:dws="%s"><dws:sdtPr><dws:id dws:val="%d"/><dws:text/></dws:sdtPr><dws:sdtContent><dws:r><dws:t xml:space="preserve">%s</dws:t></dws:r></dws:sdtContent></dws:sdt>`,
		wordNS, id, escapeText(text))
}

func plainControlText(control *node, data []byte) ([]*node, string) {
	if !control.is("sdt") || strings.TrimSpace(control.text.String()) != "" ||
		strings.Contains(string(data[control.start:control.end]), "<!--") {
		return nil, ""
	}
	props, content := control.child("sdtPr"), control.child("sdtContent")
	count := 2
	if control.child("sdtEndPr") != nil {
		count++
	}
	if props == nil || content == nil || len(control.children) != count ||
		strings.TrimSpace(content.text.String()) != "" || countChild(props, "text") != 1 || countChild(props, "lock") > 1 {
		return nil, ""
	}
	for _, item := range props.children {
		switch {
		case item.is("rPr") || item.is("alias") || item.is("tag") || item.is("id") || item.is("placeholder") || item.is("text"):
		case item.is("lock") && (item.attr("val") == "unlocked" || item.attr("val") == "sdtLocked"):
		case item.name.Space == "http://schemas.microsoft.com/office/word/2012/wordml" &&
			(item.name.Local == "appearance" || item.name.Local == "color"):
		default:
			return nil, ""
		}
	}
	return plainTextRunNodes(content.children)
}

func plainTextRunNodes(runs []*node) ([]*node, string) {
	var texts []*node
	var value strings.Builder
	for _, run := range runs {
		if !run.is("r") || strings.TrimSpace(run.text.String()) != "" || run.has("rPrChange") {
			return nil, ""
		}
		if len(run.children) == 0 || (len(run.children) == 1 && run.child("rPr") != nil) {
			continue
		}
		text := run.child("t")
		if text == nil || len(text.children) != 0 || !plainTextAttributes(text) ||
			len(run.children) != 1 && (len(run.children) != 2 || run.child("rPr") == nil) ||
			!validText(text.text.String()) {
			return nil, ""
		}
		if value.Len()+text.text.Len() > 1024 {
			return nil, ""
		}
		texts = append(texts, text)
		value.WriteString(text.text.String())
	}
	return texts, value.String()
}

// Retain the common prefix and suffix in their original runs. Changed text
// inherits the run at the start of the edit or the last run when appending.
func plainRunTextEdits(data []byte, nodes []*node, oldText, newText string) ([]replacement, error) {
	before, after := []rune(oldText), []rune(newText)
	first := 0
	for first < len(before) && first < len(after) && before[first] == after[first] {
		first++
	}
	oldEnd, newEnd := len(before), len(after)
	for oldEnd > first && newEnd > first && before[oldEnd-1] == after[newEnd-1] {
		oldEnd--
		newEnd--
	}
	var edits []replacement
	offset := 0
	inserted := false
	for i, n := range nodes {
		old := []rune(n.text.String())
		end := offset + len(old)
		prefixEnd := min(end, max(offset, first))
		suffixStart := max(offset, min(end, oldEnd))
		value := string(before[offset:prefixEnd])
		if !inserted && (first < end || i == len(nodes)-1) {
			value += string(after[first:newEnd])
			inserted = true
		}
		value += string(before[suffixStart:end])
		offset = end
		if value == n.text.String() {
			continue
		}
		edit, err := replaceTextNode(data, n, value)
		if err != nil {
			return nil, err
		}
		edits = append(edits, edit)
	}
	return edits, nil
}
