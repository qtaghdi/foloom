package docx

import (
	"fmt"
	"strings"
)

const headingTOCInstruction = `TOC \o "1-3" \h \z \u`

func supportedTOCField(field *node) bool {
	if !field.is("fldSimple") || !strings.EqualFold(strings.Join(strings.Fields(field.attr("instr")), " "), headingTOCInstruction) ||
		len(field.children) != 1 || !field.children[0].is("r") {
		return false
	}
	for _, child := range field.children[0].children {
		if !child.is("t") && !plainLineBreak(child) {
			return false
		}
	}
	return true
}

func hasTableOfContents(body *node) bool {
	isTOC := func(code string) bool {
		fields := strings.Fields(code)
		return len(fields) != 0 && strings.EqualFold(fields[0], "TOC")
	}
	var fields []*strings.Builder
	var visit func(*node) bool
	visit = func(n *node) bool {
		switch {
		case n.is("fldSimple") && isTOC(n.attr("instr")):
			return true
		case n.is("fldChar"):
			switch n.attr("fldCharType") {
			case "begin":
				fields = append(fields, new(strings.Builder))
			case "separate", "end":
				if depth := len(fields); depth != 0 {
					if isTOC(fields[depth-1].String()) {
						return true
					}
					if n.attr("fldCharType") == "end" {
						fields = fields[:depth-1]
					}
				}
			}
		case n.is("instrText") && len(fields) != 0:
			fields[len(fields)-1].WriteString(n.text.String())
		}
		for _, child := range n.children {
			if visit(child) {
				return true
			}
		}
		return false
	}
	return visit(body)
}

func (p *documentPackage) insertTableOfContents(paragraph *node) ([]replacement, error) {
	root := paragraph.parent.parent
	if hasTableOfContents(root.child("body")) {
		return nil, fmt.Errorf("DOCX document already contains a table of contents")
	}
	blocks, _, _, err := inspectDocument(root, "toc", 0, p.styles)
	if err != nil {
		return nil, err
	}
	var headings []string
	for _, block := range blocks {
		item := block.Paragraph
		if item == nil || item.OutlineLevel == nil || *item.OutlineLevel < 0 || *item.OutlineLevel > 2 {
			continue
		}
		var heading strings.Builder
		for _, run := range item.Runs {
			heading.WriteString(run.Text)
		}
		text := strings.Join(strings.Fields(heading.String()), " ")
		if text != "" {
			headings = append(headings, strings.Repeat("  ", *item.OutlineLevel)+text)
		}
	}
	content := strings.Join(headings, "\n")
	if content == "" || !validInlineText(content) {
		return nil, fmt.Errorf("DOCX table of contents requires heading text within the plain text limit")
	}
	// Cache the visible heading labels. The native field resolves page numbers
	// and internal links when the document editor updates its fields.
	raw := `<w:p xmlns:w="` + wordNS + `"><w:fldSimple w:dirty="true" w:instr="` + escapeText(headingTOCInstruction) +
		`"><w:r>` + inlineXML(content) + `</w:r></w:fldSimple></w:p>`
	return []replacement{{start: paragraph.start, end: paragraph.start, data: []byte(raw)}}, nil
}
