package docx

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const maxTableRows = 100
const maxTableColumns = 50
const maxTableCells = 3000
const maxTableRowHeight = 31680

type TableEdit struct {
	Caption     string `json:"caption,omitempty" jsonschema:"Plain caption text for insert_caption, up to 1024 UTF-8 bytes"`
	Height      *int   `json:"height,omitempty" jsonschema:"Minimum row height in twips, from 20 to 31680"`
	Action      string `json:"action" jsonschema:"insert, delete, add_row, delete_row, move_row, add_column, delete_column, move_column, set_row_height, insert_caption, or insert_body_paragraph_before"`
	Rows        int    `json:"rows,omitempty"`
	Columns     int    `json:"columns,omitempty"`
	Index       *int   `json:"index,omitempty" jsonschema:"Zero-based row or column index"`
	Destination *int   `json:"destination,omitempty" jsonschema:"Final zero-based index for a move"`
}

type tableGrid struct {
	grid  *node
	rows  []*node
	cells [][]*node
}

func simpleTable(table *node) (tableGrid, bool) {
	var out tableGrid
	if table == nil || !table.is("tbl") || tableChildCount(table, "tblGrid") != 1 || tableChildCount(table, "tblPr") > 1 {
		return out, false
	}
	for _, protected := range []string{
		"gridSpan", "vMerge", "hMerge", "ins", "del", "moveFrom", "moveTo", "sdt", "gridBefore", "gridAfter",
		"tblPrChange", "trPrChange", "tcPrChange", "tblGridChange", "bookmarkStart", "bookmarkEnd",
		"commentRangeStart", "commentRangeEnd", "permStart", "permEnd", "sectPr",
		"customXmlInsRangeStart", "customXmlInsRangeEnd", "customXmlDelRangeStart", "customXmlDelRangeEnd",
		"moveFromRangeStart", "moveFromRangeEnd", "moveToRangeStart", "moveToRangeEnd",
	} {
		if table.has(protected) {
			return out, false
		}
	}
	out.grid = table.child("tblGrid")
	columns := len(out.grid.children)
	if columns < 1 || columns > maxTableColumns {
		return out, false
	}
	for _, column := range out.grid.children {
		if !column.is("gridCol") {
			return out, false
		}
	}
	for _, row := range table.children {
		if row.is("tblGrid") || row.is("tblPr") {
			continue
		}
		if !row.is("tr") || tableChildCount(row, "trPr") > 1 || strings.TrimSpace(row.text.String()) != "" {
			return out, false
		}
		var cells []*node
		for _, cell := range row.children {
			if cell.is("trPr") || cell.is("tblPrEx") {
				continue
			}
			if !cell.is("tc") || tableChildCount(cell, "tcPr") > 1 || strings.TrimSpace(cell.text.String()) != "" {
				return out, false
			}
			for _, content := range cell.children {
				if !content.is("p") && !content.is("tcPr") {
					return out, false
				}
			}
			cells = append(cells, cell)
		}
		if len(cells) != columns {
			return out, false
		}
		out.rows = append(out.rows, row)
		out.cells = append(out.cells, cells)
	}
	return out, len(out.rows) > 0 && len(out.rows) <= maxTableRows && len(out.rows)*columns <= maxTableCells
}

func tableChildCount(parent *node, name string) int {
	count := 0
	for _, child := range parent.children {
		if child.is(name) {
			count++
		}
	}
	return count
}

const newTableCell = `<dwt:tc xmlns:dwt="` + wordNS + `"><dwt:p/></dwt:tc>`
const newGridColumn = `<dwt:gridCol xmlns:dwt="` + wordNS + `" dwt:w="1440"/>`

func newTableRow(columns int) string {
	return `<dwt:tr xmlns:dwt="` + wordNS + `">` + strings.Repeat(newTableCell, columns) + `</dwt:tr>`
}

func leadingBodyTable(n *node) bool {
	return n != nil && n.is("tbl") && n.parent != nil && n.parent.is("body") &&
		len(n.parent.children) != 0 && n.parent.children[0] == n &&
		tableChildCount(n, "tblGrid") == 1 && n.child("tblGrid").child("gridCol") != nil && n.child("tr") != nil
}

func tableReplacement(data []byte, target target, op TableEdit) (replacement, error) {
	if op.Caption != "" {
		return replacement{}, fmt.Errorf("unexpected DOCX table caption")
	}
	if op.Action != "set_row_height" && op.Height != nil {
		return replacement{}, fmt.Errorf("unexpected DOCX row height")
	}
	if op.Action == "insert_body_paragraph_before" {
		table := target.node
		if !leadingBodyTable(table) || op.Rows != 0 || op.Columns != 0 || op.Index != nil || op.Destination != nil {
			return replacement{}, fmt.Errorf("DOCX body paragraph insertion requires the leading table")
		}
		// Inserting outside the first table does not interpret or reconstruct its XML.
		raw := `<dwt:p xmlns:dwt="` + wordNS + `"/>`
		return replacement{table.start, table.start, []byte(raw)}, nil
	}

	if op.Action == "insert" {
		p := target.paragraph
		if target.kind != "replace_paragraphs" || p == nil || !p.parent.is("body") ||
			op.Rows < 1 || op.Rows > maxTableRows || op.Columns < 1 || op.Columns > maxTableColumns ||
			op.Rows*op.Columns > maxTableCells || op.Index != nil || op.Destination != nil {
			return replacement{}, fmt.Errorf("DOCX table insertion requires a body paragraph and supported dimensions")
		}
		raw := `<dwt:tbl xmlns:dwt="` + wordNS + `"><dwt:tblPr><dwt:tblW dwt:w="0" dwt:type="auto"/></dwt:tblPr><dwt:tblGrid>` +
			strings.Repeat(newGridColumn, op.Columns) + `</dwt:tblGrid>` + strings.Repeat(newTableRow(op.Columns), op.Rows) + `</dwt:tbl>`
		return replacement{p.end, p.end, []byte(raw)}, nil
	}
	if target.kind != "update_table" || op.Rows != 0 || op.Columns != 0 {
		return replacement{}, fmt.Errorf("DOCX table update requires a table ID")
	}
	table := target.node
	grid, allowed := simpleTable(table)
	if !allowed {
		return replacement{}, fmt.Errorf("complex DOCX table structure is protected")
	}
	if op.Action == "set_row_height" {
		if op.Index == nil || *op.Index < 0 || *op.Index >= len(grid.rows) || op.Destination != nil ||
			op.Height == nil || *op.Height < 20 || *op.Height > maxTableRowHeight {
			return replacement{}, fmt.Errorf("DOCX row height requires a valid row and a height from 20 to 31680 twips")
		}
		return tableRowHeightReplacement(data, grid.rows[*op.Index], *op.Height)
	}
	if op.Action == "delete" {
		if op.Index != nil || op.Destination != nil {
			return replacement{}, fmt.Errorf("unexpected DOCX table delete fields")
		}
		return replacement{table.start, table.end, nil}, nil
	}
	action, dimension, ok := strings.Cut(op.Action, "_")
	if !ok || (dimension != "row" && dimension != "column") || op.Index == nil ||
		(action == "move") != (op.Destination != nil) {
		return replacement{}, fmt.Errorf("invalid DOCX table axis operation")
	}
	columns := len(grid.grid.children)
	count, limit, other := len(grid.rows), maxTableRows, columns
	if dimension == "column" {
		count, limit, other = columns, maxTableColumns, len(grid.rows)
	}
	index := *op.Index
	if index < 0 || index > count || (action != "add" && index == count) ||
		(action == "add" && (count == limit || (count+1)*other > maxTableCells)) ||
		(action == "delete" && count == 1) ||
		(action == "move" && (*op.Destination < 0 || *op.Destination >= count || *op.Destination == index)) {
		return replacement{}, fmt.Errorf("DOCX table index or size is out of range")
	}
	if action != "add" && action != "delete" && action != "move" {
		return replacement{}, fmt.Errorf("unsupported DOCX table axis operation")
	}

	var changes []replacement
	changeAxis := func(items []*node, fresh string) {
		switch action {
		case "add":
			at := items[len(items)-1].end
			if index < len(items) {
				at = items[index].start
			}
			changes = append(changes, replacement{at, at, []byte(fresh)})
		case "delete":
			item := items[index]
			changes = append(changes, replacement{item.start, item.end, nil})
		case "move":
			to := *op.Destination
			first, last := min(index, to), max(index, to)
			for position := first; position <= last; position++ {
				source := index
				if position != to {
					if index < to {
						source = position + 1
					} else {
						source = position - 1
					}
				}
				old, next := items[position], items[source]
				changes = append(changes, replacement{old.start, old.end, data[next.start:next.end]})
			}
		}
	}
	if dimension == "row" {
		changeAxis(grid.rows, newTableRow(columns))
	} else {
		changeAxis(grid.grid.children, newGridColumn)
		for _, cells := range grid.cells {
			changeAxis(cells, newTableCell)
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].start < changes[j].start })
	var raw []byte
	offset := table.start
	for _, change := range changes {
		raw = append(raw, data[offset:change.start]...)
		raw = append(raw, change.data...)
		offset = change.end
	}
	raw = append(raw, data[offset:table.end]...)
	return replacement{table.start, table.end, raw}, nil
}

func tableRowHeightReplacement(data []byte, row *node, height int) (replacement, error) {
	props := row.child("trPr")
	var existing *node
	if props != nil {
		if tableChildCount(props, "trHeight") > 1 {
			return replacement{}, fmt.Errorf("ambiguous DOCX row height")
		}
		existing = props.child("trHeight")
	}
	raw, err := patchProperty(existing, data, "trHeight", map[string]string{"val": strconv.Itoa(height), "hRule": "atLeast"}, nil)
	if err != nil {
		return replacement{}, err
	}
	if existing != nil {
		return replacement{existing.start, existing.end, []byte(raw)}, nil
	}
	if props == nil {
		raw = `<dwt:trPr xmlns:dwt="` + wordNS + `">` + raw + `</dwt:trPr>`
		return replacement{row.inner, row.inner, []byte(raw)}, nil
	}
	opening, closing := scopedOpening(props, data, false)
	// Preserve every other row property and the bytes between them.
	if props.inner <= props.endStart {
		raw += string(data[props.inner:props.endStart])
	}
	return replacement{props.start, props.end, []byte(opening + raw + closing)}, nil
}

func tableRowHeight(row *node) int {
	props := row.child("trPr")
	if props == nil {
		return 0
	}
	height := props.child("trHeight")
	if height == nil || attribute(height, wordNS, "hRule") == "auto" {
		return 0
	}
	value, err := strconv.Atoi(attribute(height, wordNS, "val"))
	if err != nil || value < 1 || value > maxTableRowHeight {
		return 0
	}
	return value
}
