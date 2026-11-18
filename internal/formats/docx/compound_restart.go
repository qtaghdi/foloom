package docx

import (
	"fmt"
	"strconv"
	"strings"
)

type CompoundRestartInfo struct {
	ParentNumber int  `json:"parentNumber"`
	Terminal     bool `json:"terminal,omitempty"`
}

func directRestartParagraph(p *Paragraph, targets map[string]target, ranged map[*node]bool) bool {
	if p == nil || p.List == nil || !p.StructureEditable {
		return false
	}
	t := targets[p.ID]
	if t.kind != "replace_paragraphs" || t.paragraph == nil || !t.paragraph.parent.is("body") || ranged[t.paragraph] {
		return false
	}
	props := t.paragraph.child("pPr")
	ref := props.child("numPr")
	return countChild(props, "numPr") == 1 && countChild(ref, "ilvl") == 1 && countChild(ref, "numId") == 1 &&
		listInt(ref.child("ilvl").attr("val"), -1) == p.List.Level && listInt(ref.child("numId").attr("val"), -1) == p.List.ID
}

func projectCompoundRestarts(body *node, blocks []Block, targets map[string]target, catalog numberingCatalog) {
	if len(catalog.compoundSources) == 0 || body.has("permStart") || body.has("permEnd") {
		return
	}
	ranged, valid := bodyRangeBlocks(body)
	if !valid {
		return
	}
	// A following use of the same instance in a table or after a text gap
	// still belongs to the same parent until its next level-zero item.
	nextLevel := make([]int, len(blocks))
	seen := map[int]int{}
	for i := len(blocks) - 1; i >= 0; i-- {
		if p := blocks[i].Paragraph; p != nil && p.List != nil {
			nextLevel[i] = seen[p.List.ID]
			seen[p.List.ID] = p.List.Level + 1
		}
		for row := len(blocks[i].Rows) - 1; row >= 0; row-- {
			cells := blocks[i].Rows[row].Cells
			for cell := len(cells) - 1; cell >= 0; cell-- {
				paragraphs := cells[cell].Paragraphs
				for index := len(paragraphs) - 1; index >= 0; index-- {
					if list := paragraphs[index].List; list != nil {
						seen[list.ID] = list.Level + 1
					}
				}
			}
		}
	}
	for i := 0; i < len(blocks); i++ {
		parent := blocks[i].Paragraph
		if !directRestartParagraph(parent, targets, ranged) || parent.List.Level != 0 || !catalog.compoundSources[parent.List.ID] {
			continue
		}
		number := listInt(strings.TrimSuffix(parent.ListLabel, "."), 0)
		if number < 1 || number > 9999 {
			continue
		}
		end := i + 1
		for end < len(blocks) {
			child := blocks[end].Paragraph
			if !directRestartParagraph(child, targets, ranged) || child.List.ID != parent.List.ID || child.List.Level != 1 {
				break
			}
			end++
		}
		if end == i+1 || nextLevel[end-1] > 1 {
			continue
		}
		if end < len(blocks) && blocks[end].Paragraph != nil {
			if next := blocks[end].Paragraph.List; next != nil && next.Level > 0 {
				continue
			}
		}
		for child := i + 1; child < end; child++ {
			blocks[child].Paragraph.CompoundRestart = &CompoundRestartInfo{ParentNumber: number, Terminal: child == end-1}
		}
		i = end - 1
	}
}

func (p *documentPackage) restartCompoundSublevel(root *node, input ApplyInput, op Operation, parts map[string][]byte) ([]replacement, error) {
	if op.ListStart == nil || *op.ListStart < 1 || *op.ListStart > 9999 {
		return nil, fmt.Errorf("invalid DOCX compound sublevel start")
	}
	blocks, targets, _, err := p.inspectWithNoteParagraphs(root, input.SessionID, input.BaseRevision)
	if err != nil {
		return nil, err
	}
	catalog := p.projectNumbering(blocks)
	projectCompoundRestarts(root.child("body"), blocks, targets, catalog)
	first, last := -1, -1
	for i, block := range blocks {
		if block.ID == op.TargetID {
			first = i
		}
		if block.ID == op.EndTargetID {
			last = i
		}
	}
	if first < 0 || last < first || blocks[first].Paragraph == nil || blocks[last].Paragraph == nil {
		return nil, fmt.Errorf("invalid DOCX compound sublevel range")
	}
	begin, end := blocks[first].Paragraph, blocks[last].Paragraph
	if begin.CompoundRestart == nil || end.CompoundRestart == nil || !end.CompoundRestart.Terminal {
		return nil, fmt.Errorf("DOCX compound restart requires the terminal children of one parent")
	}
	source, parent := begin.List.ID, begin.CompoundRestart.ParentNumber
	for _, block := range blocks[first : last+1] {
		item := block.Paragraph
		if item == nil || item.CompoundRestart == nil || item.List.ID != source ||
			item.CompoundRestart.ParentNumber != parent {
			return nil, fmt.Errorf("DOCX compound restart cannot cross list groups")
		}
	}
	const key = "compoundRestart"
	definition := &ListDefinition{Key: key, SourceID: source, Level: 1, Start: *op.ListStart, parentNumber: parent}
	ids, _, err := p.prepareNumbering([]Operation{{Type: "define_list", ListDefinition: definition}}, parts)
	if err != nil {
		return nil, err
	}
	var edits []replacement
	for _, block := range blocks[first : last+1] {
		ref := targets[block.ID].paragraph.child("pPr").child("numPr").child("numId")
		raw, err := patchProperty(ref, p.document, "numId", map[string]string{"val": strconv.Itoa(ids[key])}, nil)
		if err != nil {
			return nil, err
		}
		edits = append(edits, replacement{ref.start, ref.end, []byte(raw)})
	}
	return edits, nil
}
