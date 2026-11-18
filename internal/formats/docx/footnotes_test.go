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
	name, notes, err := reopened.pkg.notesPart("footnote", nil)
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
	_, remaining, err := reopened.pkg.notesPart("footnote", nil)
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

func TestDOCXEndnotePersistsIndependentlyOfFootnote(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>first</w:t></w:r></w:p><w:p><w:r><w:t>second</w:t></w:r></w:p>`))
	original := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: v.Blocks[0].ID, Reference: &ReferenceEdit{Action: "insert_footnote", Text: "각주"}},
		{Type: "update_reference", TargetID: v.Blocks[1].ID, Reference: &ReferenceEdit{Action: "insert_endnote", Text: "미주 & source"}},
	}}
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := mustSnapshot(t, reopened)
	for i, kind := range []string{"footnote", "endnote"} {
		note := snapshot.Blocks[i].Paragraph.Runs[1].Object
		if note == nil || note.Kind != kind || note.noteID != "1" || note.Text != input.Operations[i].Reference.Text {
			t.Fatalf("%s preview after reopen: %+v", kind, note)
		}
		name, part, err := reopened.pkg.notesPart(kind, nil)
		if err != nil || name != "word/"+kind+"s.xml" || len(part.children) != 3 ||
			part.children[2].attr("id") != "1" || !part.children[2].has(kind+"Ref") {
			t.Fatalf("native %s part missing or malformed: %v", kind, err)
		}
	}
	run := snapshot.Blocks[1].Paragraph.Runs[1]
	if !run.Object.Editable {
		t.Fatal("plain endnote not available for editing")
	}
	beforeEndnoteEdit := fileBytes(t, filename)
	bodyBefore := bytes.Clone(reopened.pkg.document)
	footnoteBefore := bytes.Clone(reopened.pkg.partData("word/footnotes.xml", nil))
	updated, err := reopened.Apply(ctx, ApplyInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: run.ID, Reference: &ReferenceEdit{Action: "set_endnote_text", Text: "새 미주 & 근거"}},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Blocks[1].Paragraph.Runs[1].Object.Text != "새 미주 & 근거" ||
		!bytes.Equal(bodyBefore, reopened.pkg.document) || !bytes.Equal(footnoteBefore, reopened.pkg.partData("word/footnotes.xml", nil)) {
		t.Fatal("editing the endnote modified the body or footnote")
	}
	reopened, err = Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	if mustSnapshot(t, reopened).Blocks[1].Paragraph.Runs[1].Object.Text != "새 미주 & 근거" {
		t.Fatal("the edited endnote did not persist")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(beforeEndnoteEdit, fileBytes(t, filename)) {
		t.Fatal("undo changed the original endnote")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore original package bytes")
	}
}

func TestEndnoteDeletionPreservesOtherNoteParts(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>first</w:t></w:r></w:p><w:p><w:r><w:t>second</w:t></w:r></w:p>`))
	v := mustSnapshot(t, s)
	added, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: v.Blocks[0].ID, Reference: &ReferenceEdit{Action: "insert_footnote", Text: "keep footnote"}},
		{Type: "update_reference", TargetID: v.Blocks[1].ID, Reference: &ReferenceEdit{Action: "insert_endnote", Text: "remove endnote"}},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	run := added.Blocks[1].Paragraph.Runs[1]
	if !run.Object.DeleteEditable {
		t.Fatal("single non-bookmarked endnote cannot be removed")
	}
	before := fileBytes(t, filename)
	footnote := bytes.Clone(s.pkg.partData("word/footnotes.xml", nil))
	relations := bytes.Clone(s.pkg.partData(documentRelationships, nil))
	if _, err := s.Apply(ctx, ApplyInput{SessionID: added.ID, BaseRevision: added.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: run.ID, Reference: &ReferenceEdit{Action: "delete_endnote"}},
	}}, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	_, notes, err := reopened.pkg.notesPart("endnote", nil)
	if err != nil || len(notes.children) != 2 || !bytes.Equal(footnote, reopened.pkg.partData("word/footnotes.xml", nil)) ||
		!bytes.Equal(relations, reopened.pkg.partData(documentRelationships, nil)) ||
		len(mustSnapshot(t, reopened).Blocks[1].Paragraph.Runs) != 1 {
		t.Fatal("deleting the endnote changed other notes or failed to remove its marker")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore original endnote bytes")
	}
}

func TestEndnoteDeletionProtectsSpanningBookmark(t *testing.T) {
	root, err := parseXML([]byte(bodyXML(`<w:p><w:bookmarkStart w:id="5" w:name="Span"/></w:p>` +
		`<w:p><w:r><w:endnoteReference w:id="2"/></w:r></w:p><w:p><w:bookmarkEnd w:id="5"/></w:p>`)))
	if err != nil {
		t.Fatal(err)
	}
	body := root.child("body")
	target := target{node: body.children[1].child("r")}
	if endnoteDeletionAllowed(target, endnoteBookmarkSpans(body)) {
		t.Fatal("endnote inside a cross-paragraph bookmark was marked removable")
	}
}
