package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestTableGridMovesExactCellContentAndRejectsOverlaps(t *testing.T) {
	s, path := withDocument(t, bodyXML(`<w:p><w:r><w:t>body</w:t></w:r></w:p><w:tbl><w:tblPr><w:tblStyle w:val="Custom"/></w:tblPr><w:tblGrid><w:gridCol w:w="1234"/><w:gridCol w:w="2345"/></w:tblGrid><w:tr><w:trPr><w:tblHeader/></w:trPr><w:tc><w:tcPr><w:shd w:fill="FFEECC"/></w:tcPr><w:p><w:r><w:t>A</w:t></w:r></w:p><w:p><w:r><w:t>more</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc></w:tr><w:tr><w:tc><w:p><w:r><w:t>C</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>D</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`))
	original := fileBytes(t, path)
	ctx := context.Background()
	initial := mustSnapshot(t, s)
	from, to := 0, 1
	op := Operation{Type: "update_table", TargetID: initial.Blocks[1].ID, Table: &TableEdit{Action: "move_column", Index: &from, Destination: &to}}
	run := initial.Blocks[1].Rows[0].Cells[0].Paragraphs[0].Runs[0]
	input := ApplyInput{SessionID: initial.ID, BaseRevision: initial.Revision, Operations: []Operation{op, {Type: "replace_cell_text", TargetID: run.ID, ExpectedText: "A", Text: "changed"}}}
	if _, err := s.Apply(ctx, input, "human"); err == nil || !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("overlapping operation changed the file")
	}
	input.Operations = input.Operations[:1]
	v, err := s.Apply(ctx, input, "human")
	if err != nil || !v.Blocks[1].TableEditable || v.Blocks[1].Rows[0].Cells[0].Paragraphs[0].Runs[0].Text != "B" ||
		v.Blocks[1].Rows[0].Cells[1].Paragraphs[1].Runs[0].Text != "more" {
		t.Fatalf("column move did not preserve cell content: %v %+v", err, v.Blocks)
	}
	for _, expected := range []string{`<w:gridCol w:w="2345"/><w:gridCol w:w="1234"/>`, `<w:tblStyle w:val="Custom"/>`, `<w:tblHeader/>`, `<w:shd w:fill="FFEECC"/>`} {
		if !strings.Contains(string(s.pkg.document), expected) {
			t.Fatalf("lost %s", expected)
		}
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("undo changed original bytes")
	}
}

func TestTableOperationsRequireValidGridAndKeepNonemptyDimensions(t *testing.T) {
	ctx := context.Background()
	s, _ := withDocument(t, bodyXML(`<w:p><w:r><w:t>body</w:t></w:r></w:p><w:tbl><w:tblGrid><w:gridCol/></w:tblGrid><w:tr><w:tc><w:tcPr><w:vMerge/></w:tcPr><w:p/></w:tc></w:tr></w:tbl>`))
	apply := func(block int, edit TableEdit) (Snapshot, error) {
		v := mustSnapshot(t, s)
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "update_table", TargetID: v.Blocks[block].ID, Table: &edit}}}, "human")
	}
	zero, one := 0, 1
	if _, err := apply(1, TableEdit{Action: "delete"}); err == nil {
		t.Fatal("protected merged table can be deleted")
	}
	v, err := apply(0, TableEdit{Action: "insert", Rows: 1, Columns: 1})
	if err != nil || !v.Blocks[1].TableEditable {
		t.Fatalf("insert: %v %+v", err, v.Blocks)
	}
	if _, err := apply(1, TableEdit{Action: "delete_column", Index: &zero}); err == nil {
		t.Fatal("last column removed")
	}
	v, err = apply(1, TableEdit{Action: "add_column", Index: &one})
	if err != nil || len(v.Blocks[1].Rows[0].Cells) != 2 {
		t.Fatalf("add column: %v", err)
	}
	v, err = apply(1, TableEdit{Action: "add_row", Index: &zero})
	if err != nil || len(v.Blocks[1].Rows) != 2 || len(v.Blocks[1].Rows[0].Cells) != 2 {
		t.Fatalf("add row: %v", err)
	}
	v, err = apply(1, TableEdit{Action: "delete"})
	if err != nil || len(v.Blocks) != 2 || v.Blocks[1].TableEditable {
		t.Fatalf("delete: %v", err)
	}
}
