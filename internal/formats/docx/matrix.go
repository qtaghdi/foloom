package docx

import (
	"fmt"
	"strings"
)

const maxMatrixCellBytes = 256
const maxMatrixDimension = 4

type MathMatrixCellEdit struct {
	Row          int    `json:"row" jsonschema:"One-based row number, 1 through 4"`
	Column       int    `json:"column" jsonschema:"One-based column number, 1 through 4"`
	ExpectedText string `json:"expectedText"`
	Text         string `json:"text"`
}

type MathMatrixAxisEdit struct {
	Axis   string   `json:"axis" jsonschema:"row or column"`
	Mode   string   `json:"mode" jsonschema:"insert or delete"`
	Index  int      `json:"index" jsonschema:"One-based insertion or deletion position"`
	Values []string `json:"values" jsonschema:"Inserted cell values or exact expected values for a deletion"`
}

func validMatrixCellText(text string) bool {
	return len(text) <= maxMatrixCellBytes && validMathText(text)
}

func plainMatrixElement(n *node, name string, children int) bool {
	return n != nil && n.name.Space == mathNS && n.name.Local == name && onlyNamespaceAttributes(n) &&
		len(n.children) == children && strings.TrimSpace(n.text.String()) == ""
}

// A text-only matrix with default formatting has an unambiguous cell address.
func plainMatrixCells(math *node) ([][]*node, bool) {
	if !plainMatrixElement(math, "oMath", 1) {
		return nil, false
	}
	matrix := math.children[0]
	rows := len(matrix.children)
	if rows < 2 || rows > maxMatrixDimension || !plainMatrixElement(matrix, "m", rows) {
		return nil, false
	}
	columns := len(matrix.children[0].children)
	if columns < 2 || columns > maxMatrixDimension {
		return nil, false
	}
	cells := make([][]*node, rows)
	for i, row := range matrix.children {
		if !plainMatrixElement(row, "mr", columns) {
			return nil, false
		}
		cells[i] = make([]*node, columns)
		for j, cell := range row.children {
			if !plainMatrixElement(cell, "e", 1) || !plainMatrixElement(cell.children[0], "r", 1) {
				return cells, false
			}
			text := childNS(cell.children[0], mathNS, "t")
			if text == nil || len(text.children) != 0 || !validMatrixCellText(text.text.String()) {
				return cells, false
			}
			for _, attr := range text.attrs {
				if !isNamespaceAttribute(attr) && (attr.Name.Space != "http://www.w3.org/XML/1998/namespace" ||
					attr.Name.Local != "space" || attr.Value != "default" && attr.Value != "preserve") {
					return cells, false
				}
			}
			cells[i][j] = text
		}
	}
	return cells, true
}

func matrixPreview(math *node) [][]string {
	nodes, ok := plainMatrixCells(math)
	if !ok {
		return nil
	}
	values := make([][]string, len(nodes))
	for i, row := range nodes {
		values[i] = make([]string, len(row))
		for j, text := range row {
			values[i][j] = text.text.String()
		}
	}
	return values
}

func matrixContents(cells [][]string) (string, error) {
	if len(cells) < 2 || len(cells) > maxMatrixDimension || len(cells[0]) < 2 || len(cells[0]) > maxMatrixDimension {
		return "", fmt.Errorf("DOCX matrix must contain two through four rows and columns")
	}
	var contents strings.Builder
	contents.WriteString("<dwm:m>")
	for _, row := range cells {
		if len(row) != len(cells[0]) {
			return "", fmt.Errorf("DOCX matrix rows must have the same number of columns")
		}
		contents.WriteString("<dwm:mr>")
		for _, text := range row {
			if !validMatrixCellText(text) {
				return "", fmt.Errorf("DOCX matrix cells require plain text up to %d bytes", maxMatrixCellBytes)
			}
			contents.WriteString("<dwm:e>" + mathTextRun(text) + "</dwm:e>")
		}
		contents.WriteString("</dwm:mr>")
	}
	contents.WriteString("</dwm:m>")
	return contents.String(), nil
}

func matrixCellReplacement(data []byte, target target, edit *MathMatrixCellEdit) (replacement, error) {
	if target.kind != "math_object" || edit == nil || edit.Row < 1 || edit.Row > maxMatrixDimension ||
		edit.Column < 1 || edit.Column > maxMatrixDimension || !validMatrixCellText(edit.Text) ||
		strings.Contains(string(data[target.node.start:target.node.end]), "<!--") {
		return replacement{}, fmt.Errorf("protected or invalid DOCX matrix cell")
	}
	cells, ok := plainMatrixCells(target.node)
	if !ok || edit.Row > len(cells) || edit.Column > len(cells[0]) {
		return replacement{}, fmt.Errorf("protected DOCX matrix structure")
	}
	text := cells[edit.Row-1][edit.Column-1]
	if text.text.String() != edit.ExpectedText || edit.Text == edit.ExpectedText {
		return replacement{}, fmt.Errorf("DOCX matrix cell is stale or unchanged")
	}
	return replaceTextNode(data, text, edit.Text)
}

func matrixAxisReplacement(data []byte, target target, edit *MathMatrixAxisEdit) (replacement, error) {
	if target.kind != "math_object" || edit == nil || edit.Index < 1 || strings.Contains(string(data[target.node.start:target.node.end]), "<!--") {
		return replacement{}, fmt.Errorf("protected or invalid DOCX matrix axis")
	}
	cells, ok := plainMatrixCells(target.node)
	if !ok || edit.Axis != "row" && edit.Axis != "column" || edit.Mode != "insert" && edit.Mode != "delete" {
		return replacement{}, fmt.Errorf("invalid DOCX matrix structure operation")
	}
	rows, columns := len(cells), len(cells[0])
	size, count := rows, columns
	if edit.Axis == "column" {
		size, count = columns, rows
	}
	insert := edit.Mode == "insert"
	if len(edit.Values) != count || insert && (size >= maxMatrixDimension || edit.Index > size+1) || !insert && (size <= 2 || edit.Index > size) {
		return replacement{}, fmt.Errorf("DOCX matrix axis size or position is outside supported limits")
	}
	for _, value := range edit.Values {
		if !validMatrixCellText(value) {
			return replacement{}, fmt.Errorf("invalid DOCX matrix axis value")
		}
	}
	matrix := target.node.children[0]
	var edits []replacement
	cellXML := func(value string) string {
		return `<dwm:e xmlns:dwm="` + mathNS + `">` + mathTextRun(value) + `</dwm:e>`
	}
	position := func(nodes []*node) int {
		if edit.Index > len(nodes) {
			return nodes[len(nodes)-1].end
		}
		return nodes[edit.Index-1].start
	}
	if edit.Axis == "row" {
		if insert {
			var raw strings.Builder
			raw.WriteString(`<dwm:mr xmlns:dwm="` + mathNS + `">`)
			for _, value := range edit.Values {
				raw.WriteString(cellXML(value))
			}
			raw.WriteString(`</dwm:mr>`)
			at := position(matrix.children)
			edits = append(edits, replacement{at, at, []byte(raw.String())})
		} else {
			for j, text := range cells[edit.Index-1] {
				if text.text.String() != edit.Values[j] {
					return replacement{}, fmt.Errorf("stale DOCX matrix row")
				}
			}
			row := matrix.children[edit.Index-1]
			edits = append(edits, replacement{row.start, row.end, nil})
		}
	} else {
		for i, row := range matrix.children {
			if insert {
				at := position(row.children)
				edits = append(edits, replacement{at, at, []byte(cellXML(edit.Values[i]))})
			} else {
				if cells[i][edit.Index-1].text.String() != edit.Values[i] {
					return replacement{}, fmt.Errorf("stale DOCX matrix column")
				}
				cell := row.children[edit.Index-1]
				edits = append(edits, replacement{cell.start, cell.end, nil})
			}
		}
	}
	var raw strings.Builder
	at := matrix.start
	for _, e := range edits {
		raw.Write(data[at:e.start])
		raw.Write(e.data)
		at = e.end
	}
	raw.Write(data[at:matrix.end])
	return replacement{matrix.start, matrix.end, []byte(raw.String())}, nil
}
