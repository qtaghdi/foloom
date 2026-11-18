package docx

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPrependBodyParagraphPreservesProtectedLeadingTable(t *testing.T) {
	ctx := context.Background()
	table := `<w:tbl><w:tblGrid><w:gridCol w:w="1800"/><w:gridCol w:w="1800"/></w:tblGrid><w:tr><w:tc><w:tcPr><w:gridSpan w:val="2"/></w:tcPr><w:p><w:r><w:t>병합된 셀</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`
	s, filename := withDocument(t, bodyXML(table+`<w:sectPr/>`))
	before := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	if len(v.Blocks) != 1 || v.Blocks[0].TableEditable || !v.Blocks[0].PrependParagraph {
		t.Fatalf("unexpected leading table availability: %+v", v.Blocks)
	}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_table", TargetID: v.Blocks[0].ID, Table: &TableEdit{Action: "insert_body_paragraph_before"}},
	}}
	v, err := s.Apply(ctx, input, "human")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Blocks) != 2 || !v.Blocks[0].Paragraph.StructureEditable || v.Blocks[1].PrependParagraph || v.Blocks[1].TableEditable {
		t.Fatalf("expected an editable paragraph without unprotecting the table: %+v", v.Blocks)
	}
	if _, err := s.Apply(ctx, input, "human"); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale request was not refused: %v", err)
	}
	if _, err := s.Apply(ctx, batch(v, "replace_run_text", v.Blocks[0].Paragraph.Runs[0], "새 본문"), "human"); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, s)
	if v.Blocks[0].Paragraph.Runs[0].Text != "새 본문" || !strings.Contains(string(s.pkg.document), table) {
		t.Fatal("typed body text or original table XML did not survive reopen")
	}
	historyRestore(t, s, "undo")
	historyRestore(t, s, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original document bytes")
	}
}

func TestPrependBodyParagraphRejectsNonleadingTable(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>AB</w:t></w:r></w:p><w:tbl><w:tblGrid><w:gridCol w:w="1800"/></w:tblGrid><w:tr><w:tc><w:p/></w:tc></w:tr></w:tbl>`))
	v := mustSnapshot(t, s)
	before := fileBytes(t, filename)
	if !v.Blocks[1].TableEditable || v.Blocks[1].PrependParagraph {
		t.Fatal("only the leading table should offer paragraph insertion")
	}
	_, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_table", TargetID: v.Blocks[1].ID, Table: &TableEdit{Action: "insert_body_paragraph_before"}},
	}}, "human")
	if err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatalf("invalid insertion changed the original package: %v", err)
	}
}
