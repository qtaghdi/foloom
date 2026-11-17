package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRenameBookmarkUpdatesInternalLinksAndPreservesRuns(t *testing.T) {
	ctx := context.Background()
	marked := `<w:p><w:bookmarkStart w:id="3" w:name="Old"/><w:r><w:t>target</w:t></w:r><w:bookmarkEnd w:id="3"/></w:p>`
	link := `<w:p><w:hyperlink w:anchor="Old" w:tooltip="display hint"><w:r><w:rPr><w:b/></w:rPr><w:t>click</w:t></w:r></w:hyperlink></w:p>`
	other := `<w:p><w:bookmarkStart w:id="4" w:name="Other"/><w:r><w:t>other</w:t></w:r><w:bookmarkEnd w:id="4"/></w:p>`
	s, filename := withDocument(t, bodyXML(marked+link+other))
	before := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	if len(v.Bookmarks) != 2 || !v.Bookmarks[0].RenameEditable {
		t.Fatal("simple bookmark was not exposed for rename")
	}
	rename := ReferenceEdit{Action: "rename_bookmark", Bookmark: "Old", NewBookmark: "other"}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: v.Bookmarks[0].ParagraphID, Reference: &rename},
	}}
	if _, err := s.Apply(ctx, input, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("invalid rename was persisted")
	}
	rename.NewBookmark = "Updated"
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	result := mustSnapshot(t, reopened)
	if result.Bookmarks[0].Name != "Updated" || result.Bookmarks[0].ParagraphID != result.Blocks[0].ID ||
		result.Blocks[1].Paragraph.Runs[0].Link == nil || result.Blocks[1].Paragraph.Runs[0].Link.Bookmark != "Updated" {
		t.Fatal("bookmark or linked location was not updated")
	}
	doc := string(reopened.pkg.document)
	if !strings.Contains(doc, other) || !strings.Contains(doc, `w:id="3"`) ||
		!strings.Contains(doc, `<w:r><w:rPr><w:b/></w:rPr><w:t>click</w:t></w:r>`) ||
		!strings.Contains(doc, `w:tooltip="display hint"`) {
		t.Fatal("rename changed related formatting or bookmark identity")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore initial document bytes")
	}
}

func TestRenameBookmarkRejectsOpaqueFieldReferences(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:bookmarkStart w:id="3" w:name="Old"/><w:r><w:t>x</w:t></w:r><w:bookmarkEnd w:id="3"/></w:p><w:p><w:fldSimple w:instr=" REF Old "><w:r><w:t>x</w:t></w:r></w:fldSimple></w:p>`))
	before := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: v.Bookmarks[0].ParagraphID, Reference: &ReferenceEdit{Action: "rename_bookmark", Bookmark: "Old", NewBookmark: "New"}},
	}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("bookmark rename broke an opaque field")
	}
}
