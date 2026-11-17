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
	beforeEdit := fileBytes(t, filename)
	bodyBefore := bytes.Clone(reopened.pkg.document)
	footnote := snapshot.Blocks[0].Paragraph.Runs[1]
	if !footnote.Object.Editable {
		t.Fatal("simple footnote was not marked editable")
	}
	updated, err := reopened.Apply(ctx, ApplyInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: footnote.ID, Reference: &ReferenceEdit{Action: "set_footnote_text", Text: "새 각주 & 교체"}},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bodyBefore, reopened.pkg.document) || updated.Blocks[0].Paragraph.Runs[1].Object.Text != "새 각주 & 교체" ||
		updated.Blocks[1].Paragraph.Runs[1].Object.Text != b.Text {
		t.Fatal("editing the footnote changed its referent or another note")
	}
	reopened, err = Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	if note := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[1].Object; note.Text != "새 각주 & 교체" {
		t.Fatal("edited footnote text was not persisted")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(beforeEdit, fileBytes(t, filename)) {
		t.Fatal("undo changed the original footnote")
	}
	restored := mustSnapshot(t, reopened)
	deleted, err := reopened.Apply(ctx, ApplyInput{SessionID: restored.ID, BaseRevision: restored.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: restored.Blocks[0].Paragraph.Runs[1].ID, Reference: &ReferenceEdit{Action: "delete_footnote"}},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted.Blocks[0].Paragraph.Runs) != 1 || deleted.Blocks[1].Paragraph.Runs[1].Object.Text != b.Text {
		t.Fatal("deletion changed another note or failed to remove its reference")
	}
	_, remaining, err := reopened.pkg.footnotesPart(nil)
	if err != nil || len(remaining.children) != 3 || remaining.children[2].attr("id") != "2" {
		t.Fatal("deletion changed remaining notes or separators", err)
	}
	reopened, err = Open(ctx, filename)
	if err != nil || len(mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs) != 1 {
		t.Fatal("reference not removed after reopen", err)
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(beforeEdit, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the deleted footnote")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original package")
	}
}

func TestDOCXFootnoteRemovalRejectsSharedReference(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>source</w:t></w:r></w:p>`))
	v := mustSnapshot(t, s)
	if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: v.Blocks[0].ID, Reference: &ReferenceEdit{Action: "insert_footnote", Text: "shared"}},
	}}, "human"); err != nil {
		t.Fatal(err)
	}
	rewritePackage(t, filename, func(name string, raw []byte) []byte {
		if name == "word/document.xml" {
			return bytes.Replace(raw, []byte(`</w:p>`), []byte(`</w:p><w:p><w:r><w:footnoteReference w:id="01"/></w:r></w:p>`), 1)
		}
		return raw
	}, nil)
	s, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	original := fileBytes(t, filename)
	v = mustSnapshot(t, s)
	if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: v.Blocks[0].Paragraph.Runs[1].ID, Reference: &ReferenceEdit{Action: "delete_footnote"}},
	}}, "human"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("shared footnote deletion changed the package")
	}
}
