package docx

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	figureListInstruction = `TOC \c "Figure" \h \z`
	tableListInstruction  = `TOC \c "Table" \h \z`
)

var wordFieldTokens = regexp.MustCompile(`"[^"]*"|[^\s"]+`)

func tocInstructionLabel(code string) (bool, string) {
	if words := strings.Fields(code); len(words) == 0 || !strings.EqualFold(words[0], "TOC") {
		return false, ""
	}
	// Keep quoted switch arguments intact so a backslash inside an argument
	// does not turn a heading table of contents into a caption list.
	tokens := wordFieldTokens.FindAllString(code, -1)
	for i, token := range tokens[:len(tokens)-1] {
		if strings.EqualFold(token, `\c`) {
			return true, strings.Trim(tokens[i+1], `"`)
		}
	}
	return true, ""
}

func insertCaptionList(paragraph *node, sequence, instruction string) ([]replacement, error) {
	body := paragraph.parent
	if anyWordField(body, func(_ *node, code string) bool {
		isTOC, label := tocInstructionLabel(code)
		return isTOC && strings.EqualFold(label, sequence)
	}) {
		return nil, fmt.Errorf("DOCX document already contains a %s caption list", sequence)
	}
	content, err := captionListText(body, sequence)
	if err != nil {
		return nil, err
	}
	if content == "" {
		return nil, fmt.Errorf("DOCX caption list requires a %s caption", sequence)
	}
	return insertTOCCache(paragraph, instruction, content), nil
}

func isCaptionSequence(code, label string) bool {
	words := strings.Fields(code)
	return len(words) >= 2 && strings.EqualFold(words[0], "SEQ") && strings.EqualFold(strings.Trim(words[1], `"`), label)
}

func captionListText(body *node, sequence string) (string, error) {
	if anyWordField(body, func(simple *node, code string) bool {
		return isCaptionSequence(code, sequence) &&
			(simple == nil || !simple.parent.is("p") || simple.parent.parent != body)
	}) {
		return "", fmt.Errorf("protected DOCX %s caption sequence", sequence)
	}
	var lines strings.Builder
	for _, paragraph := range body.children {
		if !paragraph.is("p") {
			continue
		}
		for _, child := range paragraph.children {
			if !child.is("fldSimple") || !isCaptionSequence(child.attr("instr"), sequence) {
				continue
			}
			label, err := captionLabel(paragraph, child)
			if err != nil {
				return "", err
			}
			if lines.Len() != 0 {
				lines.WriteByte('\n')
			}
			lines.WriteString(label)
			if lines.Len() > maxTextBytes {
				return "", fmt.Errorf("DOCX caption list exceeds the text size limit")
			}
		}
	}
	content := lines.String()
	if !validInlineText(content) {
		return "", fmt.Errorf("DOCX caption list exceeds the plain text limit")
	}
	return content, nil
}

func captionLabel(paragraph, field *node) (string, error) {
	protected := fmt.Errorf("protected DOCX caption paragraph")
	words := strings.Fields(field.attr("instr"))
	if len(words) != 2 && !(len(words) == 4 && words[2] == `\*` && strings.EqualFold(words[3], "ARABIC")) {
		return "", protected
	}
	for _, attr := range field.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || attr.Name.Local != "instr" && attr.Name.Local != "dirty") {
			return "", protected
		}
	}
	texts, result := plainTextRunNodes(field.children)
	number, err := strconv.ParseInt(result, 10, 32)
	if len(texts) != 1 || err != nil || number < 1 {
		return "", protected
	}

	var label strings.Builder
	for _, child := range paragraph.children {
		switch {
		case child == field:
			label.WriteString(result)
		case child.is("pPr"):
			if child.has("sectPr") || child.has("pPrChange") {
				return "", protected
			}
		case child.is("bookmarkStart"), child.is("bookmarkEnd"):
			// Word may add hidden bookmarks to link a caption after updating its list.
		default:
			nodes, value := plainTextRunNodes([]*node{child})
			if nodes == nil && (!child.is("r") || len(child.children) != 0) {
				return "", protected
			}
			label.WriteString(value)
		}
	}
	return strings.Join(strings.Fields(label.String()), " "), nil
}

func (p *documentPackage) updateCaptionList(t target) ([]replacement, error) {
	var sequence string
	switch t.kind {
	case "figure_list":
		sequence = "Figure"
	case "table_list":
		sequence = "Table"
	default:
		return nil, fmt.Errorf("protected DOCX caption list")
	}
	run := tocCacheRun(t.node, p.document)
	if run == nil {
		return nil, fmt.Errorf("protected DOCX caption list")
	}
	content, err := captionListText(t.paragraph.parent, sequence)
	if err != nil {
		return nil, err
	}
	return replaceTOCCache(run, content), nil
}
