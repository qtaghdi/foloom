package docx

import (
	"fmt"
	"sort"
	"strings"
)

func bookmarkedPlainText(index *bookmarkIndex, start *node) string {
	if !index.safeStart(start) || !start.parent.is("p") || !start.parent.parent.is("body") {
		return ""
	}
	end := index.ends[start.attr("id")][0]
	// The write plan and bookmark index may come from separate XML parses.
	if end.parent == nil || end.parent.start != start.parent.start {
		return ""
	}
	runs := start.parent.children
	first := sort.Search(len(runs), func(i int) bool { return runs[i].start > start.start })
	last := sort.Search(len(runs), func(i int) bool { return runs[i].start >= end.start })
	_, text := plainTextRunNodes(runs[first:last])
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return text
}

func bookmarkFieldName(field *node) string {
	if !field.is("fldSimple") || len(field.children) != 1 || strings.TrimSpace(field.text.String()) != "" {
		return ""
	}
	for _, attr := range field.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || attr.Name.Local != "instr" && attr.Name.Local != "dirty") {
			return ""
		}
	}
	instruction := strings.Fields(field.attr("instr"))
	if len(instruction) != 3 || !strings.EqualFold(instruction[0], "REF") ||
		!bookmarkName.MatchString(instruction[1]) || !strings.EqualFold(instruction[2], `\h`) {
		return ""
	}
	nodes, _ := plainTextRunNodes(field.children)
	if len(nodes) != 1 {
		return ""
	}
	return instruction[1]
}

func bookmarkFieldXML(index *bookmarkIndex, name string) (string, error) {
	if !bookmarkName.MatchString(name) || index.names[strings.ToLower(name)] != 1 {
		return "", fmt.Errorf("invalid or ambiguous DOCX bookmark")
	}
	for _, starts := range index.starts {
		if len(starts) != 1 || starts[0].attr("name") != name {
			continue
		}
		text := bookmarkedPlainText(index, starts[0])
		if text == "" {
			break
		}
		return `<dwr:fldSimple xmlns:dwr="` + wordNS + `" dwr:instr="REF ` + name + ` \h" dwr:dirty="true"><dwr:r>` +
			inlineXML(text) + `</dwr:r></dwr:fldSimple>`, nil
	}
	return "", fmt.Errorf("DOCX bookmark requires a plain text range")
}

func patchBookmarkField(data []byte, field *node, deleting bool, newName, display string) (replacement, error) {
	if !field.parent.is("p") || !field.parent.parent.is("body") ||
		strings.Contains(string(data[field.start:field.end]), "<!--") {
		return replacement{}, fmt.Errorf("protected DOCX bookmark field")
	}
	if deleting {
		run := field.children[0]
		open, close := scopedOpening(run, data, false)
		content := string(data[run.inner:run.endStart])
		text := run.child("t")
		if display != "" && text.text.String() != display {
			edit, err := replaceTextNode(data, text, display)
			if err != nil {
				return replacement{}, err
			}
			content = string(data[run.inner:edit.start]) + string(edit.data) + string(data[edit.end:run.endStart])
		}
		return replacement{start: field.start, end: field.end, data: []byte(open + content + close)}, nil
	}
	instruction := strings.Fields(field.attr("instr"))
	instruction[1] = newName
	open, _ := scopedOpening(field, data, false)
	open, err := patchOpeningAttributes(open, wordNS, map[string]string{"instr": strings.Join(instruction, " ")}, nil)
	if err != nil {
		return replacement{}, err
	}
	return replacement{start: field.start, end: field.inner, data: []byte(open)}, nil
}
