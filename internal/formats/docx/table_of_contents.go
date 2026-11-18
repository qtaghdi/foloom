package docx

import (
	"fmt"
	"strings"
)

const headingTOCInstruction = `TOC \o "1-3" \h \z \u`

func tocObjectKind(field *node) string {
	if !field.is("fldSimple") || len(field.children) != 1 || !field.children[0].is("r") {
		return ""
	}
	for _, child := range field.children[0].children {
		if !child.is("t") && !plainLineBreak(child) {
			return ""
		}
	}
	switch strings.ToUpper(strings.Join(strings.Fields(field.attr("instr")), " ")) {
	case strings.ToUpper(headingTOCInstruction):
		return "toc"
	case strings.ToUpper(figureListInstruction):
		return "figure_list"
	case strings.ToUpper(tableListInstruction):
		return "table_list"
	default:
		return ""
	}
}

func hasTableOfContents(body *node) bool {
	return anyWordField(body, func(_ *node, code string) bool {
		isTOC, label := tocInstructionLabel(code)
		return isTOC && label == ""
	})
}

func anyWordField(root *node, matches func(simple *node, code string) bool) bool {
	var fields []*strings.Builder
	var visit func(*node) bool
	visit = func(n *node) bool {
		switch {
		case n.is("fldSimple"):
			simple := n
			if len(fields) != 0 {
				simple = nil
			}
			if matches(simple, n.attr("instr")) {
				return true
			}
		case n.is("fldChar"):
			switch n.attr("fldCharType") {
			case "begin":
				fields = append(fields, new(strings.Builder))
			case "separate", "end":
				if depth := len(fields); depth != 0 {
					if matches(nil, fields[depth-1].String()) {
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
	return visit(root)
}

func (p *documentPackage) insertTableOfContents(paragraph *node) ([]replacement, error) {
	root := paragraph.parent.parent
	if hasTableOfContents(root.child("body")) {
		return nil, fmt.Errorf("DOCX document already contains a table of contents")
	}
	content, err := p.tocHeadingText(root)
	if err != nil {
		return nil, err
	}
	if content == "" {
		return nil, fmt.Errorf("DOCX table of contents requires heading text")
	}
	return insertTOCCache(paragraph, headingTOCInstruction, content), nil
}

func insertTOCCache(paragraph *node, instruction, content string) []replacement {
	// Cache the visible labels. The native field resolves page numbers and
	// internal links when the document editor updates its fields.
	raw := `<w:p xmlns:w="` + wordNS + `"><w:fldSimple w:dirty="true" w:instr="` + escapeText(instruction) +
		`"><w:r>` + inlineXML(content) + `</w:r></w:fldSimple></w:p>`
	return []replacement{{start: paragraph.start, end: paragraph.start, data: []byte(raw)}}
}
func (p *documentPackage) tocHeadingText(root *node) (string, error) {
	blocks, _, _, err := inspectDocument(root, "toc", 0, p.styles)
	if err != nil {
		return "", err
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
	if !validInlineText(content) {
		return "", fmt.Errorf("DOCX table of contents requires heading text within the plain text limit")
	}
	return content, nil
}

func tocCacheRun(field *node, data []byte) *node {
	if tocObjectKind(field) == "" || field.parent == nil || !field.parent.is("p") ||
		field.parent.parent == nil || !field.parent.parent.is("body") || len(field.parent.children) != 1 ||
		!onOff(field.attr("dirty"), false) || strings.Contains(string(data[field.start:field.end]), "<!--") {
		return nil
	}
	for _, attr := range field.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || attr.Name.Local != "instr" && attr.Name.Local != "dirty") {
			return nil
		}
	}
	run := field.children[0]
	if !onlyNamespaceAttributes(run) || strings.TrimSpace(run.text.String()) != "" {
		return nil
	}
	for _, child := range run.children {
		if child.is("t") && !plainTextAttributes(child) {
			return nil
		}
	}
	return run
}

func (p *documentPackage) updateTableOfContents(t target) ([]replacement, error) {
	if t.kind != "toc" {
		return nil, fmt.Errorf("protected DOCX table of contents")
	}
	run := tocCacheRun(t.node, p.document)
	if run == nil {
		return nil, fmt.Errorf("protected DOCX table of contents")
	}
	content, err := p.tocHeadingText(t.paragraph.parent.parent)
	if err != nil {
		return nil, err
	}
	return replaceTOCCache(run, content), nil
}

func replaceTOCCache(run *node, content string) []replacement {
	if content == textContent(run) {
		return nil
	}
	return []replacement{{start: run.inner, end: run.endStart, data: []byte(inlineXML(content))}}
}
