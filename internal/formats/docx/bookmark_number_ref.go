package docx

import "strings"

func (p *documentPackage) projectNumbering(blocks []Block) numberingCatalog {
	listRoot, _ := parseXML(p.partData(p.numberingPath, nil))
	catalog := readNumbering(listRoot)
	applyListLabels(blocks, catalog)
	return catalog
}

func numberedBookmarkText(para *Paragraph, numbering numberingCatalog) string {
	if para.List == nil || para.List.Level != 0 {
		return ""
	}
	level := numbering.levels[para.List.ID][0]
	switch level.kind {
	case "decimal", "upperLetter", "lowerLetter", "upperRoman", "lowerRoman":
	default:
		return ""
	}
	if strings.Count(level.text, "%") != 1 || !strings.Contains(level.text, "%1") ||
		para.ListLabel == "["+level.kind+"]" || !validText(para.ListLabel) {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(para.ListLabel), ".")
}

func (p *documentPackage) bookmarkNumberText(root *node, index *bookmarkIndex, name string) (string, error) {
	blocks, _, _, err := inspectDocument(root, "number-ref", 0, p.styles)
	if err != nil {
		return "", err
	}
	numbering := p.projectNumbering(blocks)
	blockIndex := 0
	for _, child := range root.child("body").children {
		if !child.is("p") && !child.is("tbl") {
			continue
		}
		block := blocks[blockIndex]
		blockIndex++
		if block.Paragraph == nil {
			continue
		}
		for _, marker := range child.children {
			if marker.is("bookmarkStart") && marker.attr("name") == name && bookmarkedPlainText(index, marker) != "" {
				return numberedBookmarkText(block.Paragraph, numbering), nil
			}
		}
	}
	return "", nil
}
