package docx

import (
	"fmt"
	"strings"
)

const tableSequence = `SEQ Table \* ARABIC`

func isTableSequence(instruction string) bool {
	return isCaptionSequence(instruction, "Table")
}

func supportedTableSequence(instruction string) bool {
	fields := strings.Fields(instruction)
	return len(fields) == 2 || len(fields) == 4 && fields[2] == `\*` && strings.EqualFold(fields[3], "ARABIC")
}

func documentTableCaptionFields(body *node) ([]*node, error) {
	values, err := documentCaptionFields(body, "table", isTableSequence, supportedTableSequence)
	if err != nil {
		return nil, err
	}
	var lastParagraph *node
	for _, value := range values {
		field := value.parent.parent
		paragraph := field.parent
		if lastParagraph == paragraph {
			return nil, fmt.Errorf("multiple DOCX table sequences in one caption")
		}
		if _, err := captionLabel(paragraph, field); err != nil {
			return nil, err
		}
		lastParagraph = paragraph
	}
	return values, nil
}

func existingTableCaption(paragraph *node) bool {
	if paragraph == nil || !paragraph.is("p") {
		return false
	}
	if anyWordField(paragraph, func(_ *node, code string) bool { return isTableSequence(code) }) {
		return true
	}
	return strings.EqualFold(paragraph.child("pPr").child("pStyle").attr("val"), "Caption") &&
		!anyWordField(paragraph, func(_ *node, code string) bool { return isCaptionSequence(code, "Figure") })
}

func tableCaptionBoundary(blocks []*node, index int) bool {
	if index > 0 && existingTableCaption(blocks[index-1]) {
		return false
	}
	// A caption immediately preceding the next table belongs to that table.
	// Otherwise protect an imported caption below the current table.
	return index+1 == len(blocks) || !existingTableCaption(blocks[index+1]) ||
		index+2 < len(blocks) && blocks[index+2].is("tbl")
}

func (p *documentPackage) insertTableCaption(t target, op TableEdit) ([]replacement, error) {
	if !t.tableCaptionInsertable || t.kind != "update_table" || op.Rows != 0 || op.Columns != 0 ||
		op.Index != nil || op.Destination != nil || op.Height != nil ||
		len(op.Caption) > 1024 || !validText(op.Caption) || strings.TrimSpace(op.Caption) == "" {
		return nil, fmt.Errorf("DOCX table caption requires plain text and an uncaptioned body table")
	}
	values, err := documentTableCaptionFields(t.node.parent)
	if err != nil {
		return nil, err
	}
	return numberedCaptionEdits(p.document, values, t.node.start, tableSequence, "표 ", op.Caption, true)
}
