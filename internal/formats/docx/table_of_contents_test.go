package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestInsertTableOfContentsPersistsAndUndoes(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>Contents here</w:t></w:r></w:p><w:p><w:pPr><w:outlineLvl w:val="0"/></w:pPr><w:r><w:t>개요 &amp; 목적</w:t></w:r></w:p><w:p><w:pPr><w:outlineLvl w:val="1"/></w:pPr><w:r><w:t>절차</w:t></w:r></w:p><w:p><w:pPr><w:outlineLvl w:val="3"/></w:pPr><w:r><w:t>Excluded</w:t></w:r></w:p>`))
	before := fileBytes(t, filename)
	insert := func(s *Session, index int) (Snapshot, error) {
		v := mustSnapshot(t, s)
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{
			Type: "update_reference", TargetID: v.Blocks[index].ID, Reference: &ReferenceEdit{Action: "insert_toc"},
		}}}, "human")
	}
	if _, err := insert(s, 0); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v := mustSnapshot(t, reopened)
	toc := v.Blocks[0].Paragraph.Runs[0].Object
	if v.Blocks[0].Paragraph.Runs[0].Editable || toc == nil || toc.Kind != "toc" || !toc.Editable || toc.Text != "개요 & 목적\n  절차" ||
		!strings.Contains(string(reopened.pkg.document), escapeText(headingTOCInstruction)) {
		t.Fatalf("unexpected native table of contents: %+v", toc)
	}
	saved := fileBytes(t, filename)
	if _, err := insert(reopened, 1); err == nil {
		t.Fatal("a second table of contents was accepted")
	}
	if !bytes.Equal(saved, fileBytes(t, filename)) {
		t.Fatal("refusing a duplicate changed the source")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original source")
	}
}

func TestDetectExistingComplexTableOfContentsWithSplitInstruction(t *testing.T) {
	root, err := parseXML([]byte(bodyXML(`<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> TO</w:instrText></w:r><w:r><w:instrText>C \o "1-3"</w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r></w:p><w:p><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`)))
	if err != nil {
		t.Fatal(err)
	}
	if !hasTableOfContents(root.child("body")) {
		t.Fatal("split native TOC instruction was missed")
	}
}

func TestRefreshTableOfContentsKeepsNativeFieldAndRebuildsLabels(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>start</w:t></w:r></w:p><w:p><w:pPr><w:outlineLvl w:val="0"/></w:pPr><w:r><w:t>old heading</w:t></w:r></w:p>`))
	apply := func(v Snapshot, op Operation) Snapshot {
		t.Helper()
		snapshot, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}, "human")
		if err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	v := mustSnapshot(t, s)
	v = apply(v, Operation{Type: "update_reference", TargetID: v.Blocks[0].ID, Reference: &ReferenceEdit{Action: "insert_toc"}})
	heading := v.Blocks[2].Paragraph.Runs[0]
	v = apply(v, Operation{Type: "replace_run_text", TargetID: heading.ID, ExpectedText: heading.Text, Text: "새 제목"})
	before := fileBytes(t, filename)
	refresh := Operation{Type: "update_reference", TargetID: v.Blocks[0].Paragraph.Runs[0].ID, Reference: &ReferenceEdit{Action: "update_toc"}}
	if !v.Blocks[0].Paragraph.Runs[0].Object.Editable || v.Blocks[0].Paragraph.Runs[0].Object.Text != "old heading" {
		t.Fatal("original TOC cache was not present")
	}
	v = apply(v, refresh)
	if v.Blocks[0].Paragraph.Runs[0].Object.Text != "새 제목" {
		t.Fatal("table of contents did not gather the updated heading")
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	if v := mustSnapshot(t, reopened); v.Blocks[0].Paragraph.Runs[0].Object.Text != "새 제목" ||
		!strings.Contains(string(reopened.pkg.document), escapeText(headingTOCInstruction)) {
		t.Fatal("saved DOCX lost native instruction or cached headings")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo changed the saved heading along with the table of contents")
	}
}

func TestRefreshTableOfContentsDoesNotExposeLockedOrComplexField(t *testing.T) {
	s, _ := withDocument(t, bodyXML(`<w:p><w:fldSimple w:dirty="true" w:fldLock="true" w:instr="`+escapeText(headingTOCInstruction)+`"><w:r><w:t>protected cache</w:t></w:r></w:fldSimple></w:p>`))
	v := mustSnapshot(t, s)
	run := v.Blocks[0].Paragraph.Runs[0]
	if run.Object == nil || run.Object.Editable {
		t.Fatal("protected native table of contents was exposed as editable")
	}
	if _, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{
		Type: "update_reference", TargetID: run.ID, Reference: &ReferenceEdit{Action: "update_toc"},
	}}}, "human"); err == nil {
		t.Fatal("protected native table of contents could be changed")
	}
}
