package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDOCXFootnotesPersistAndPreserveOriginalParts(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>source</w:t></w:r></w:p><w:p><w:r><w:t>next source</w:t></w:r></w:p>`))
	original := fileBytes(t, filename)
	originalParts := s.pkg.parts
	v := mustSnapshot(t, s)
	a := ReferenceEdit{Action: "insert_footnote", Text: "출처 & 근거"}
	b := ReferenceEdit{Action: "insert_footnote", Text: " "}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: v.Blocks[0].ID, Reference: &a},
		{Type: "update_reference", TargetID: v.Blocks[1].ID, Reference: &b},
	}}
	if _, err := s.Apply(ctx, input, "human"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("invalid footnote committed part of a batch")
	}
	b.Text = "Second note"
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := mustSnapshot(t, reopened)
	for i, expected := range []string{a.Text, b.Text} {
		paragraph := snapshot.Blocks[i].Paragraph
		if len(paragraph.Runs) != 2 || paragraph.Runs[1].Object == nil ||
			paragraph.Runs[1].Object.Kind != "footnote" || paragraph.Runs[1].Object.Text != expected ||
			paragraph.Runs[1].Editable {
			t.Fatalf("footnote %d not available after reopen: %+v", i+1, paragraph.Runs)
		}
	}
	name, notes, err := reopened.pkg.footnotesPart(nil)
	if err != nil || name == "" || len(notes.children) != 4 ||
		notes.children[0].attr("type") != "separator" ||
		notes.children[1].attr("type") != "continuationSeparator" ||
		notes.children[2].attr("id") != "1" || notes.children[3].attr("id") != "2" ||
		!notes.children[2].has("footnoteRef") {
		t.Fatal("native footnote parts and identifiers not preserved", err)
	}
	if !strings.Contains(string(reopened.pkg.partData("[Content_Types].xml", nil)), "application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml") {
		t.Fatal("footnote content type missing")
	}
	for _, part := range originalParts {
		if part.header.Name != "word/document.xml" && part.header.Name != "[Content_Types].xml" &&
			!bytes.Equal(part.data, reopened.pkg.partData(part.header.Name, nil)) {
			t.Fatalf("unrelated part %s changed", part.header.Name)
		}
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original package")
	}
}
