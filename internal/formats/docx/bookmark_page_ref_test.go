package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestBookmarkPageReferencePersistsAndTracksBookmarkRename(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>Referenced page: </w:t></w:r></w:p><w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:t>Second page</w:t></w:r></w:p><w:p><w:bookmarkStart w:id="1" w:name="Target"/><w:r><w:t>Target on second page</w:t></w:r><w:bookmarkEnd w:id="1"/></w:p>`))
	before := fileBytes(t, filename)
	apply := func(v Snapshot, target string, ref ReferenceEdit) Snapshot {
		t.Helper()
		out, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{
			Type: "update_reference", TargetID: target, Reference: &ref,
		}}}, "human")
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	v := mustSnapshot(t, s)
	v = apply(v, v.Blocks[0].ID, ReferenceEdit{Action: "insert_bookmark_page_ref", Bookmark: "Target"})
	if run := v.Blocks[0].Paragraph.Runs[1]; run.Link == nil || !run.Link.PageReference || run.Text != "?" ||
		!strings.Contains(string(s.pkg.document), `dwr:instr="PAGEREF Target \h"`) {
		t.Fatalf("missing unresolved native page field: %+v", run)
	}
	v = apply(v, v.Bookmarks[0].ParagraphID, ReferenceEdit{Action: "rename_bookmark", Bookmark: "Target", NewBookmark: "Moved"})
	s, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, s)
	if run := v.Blocks[0].Paragraph.Runs[1]; run.Link == nil || run.Link.Bookmark != "Moved" ||
		!strings.Contains(string(s.pkg.document), `instr="PAGEREF Moved \h"`) {
		t.Fatalf("rename did not update the page field: %+v from %s", run.Link, s.pkg.document)
	}
	v = apply(v, v.Bookmarks[0].ParagraphID, ReferenceEdit{Action: "delete_bookmark", Bookmark: "Moved"})
	if run := v.Blocks[0].Paragraph.Runs[1]; run.Link != nil || run.Text != "?" || !run.Editable {
		t.Fatalf("bookmark removal changed the cached field result: %+v", run)
	}
	for range 3 {
		historyRestore(t, s, "undo")
	}
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original file")
	}
}

func TestDeletingBookmarkKeepsNativePageFieldCache(t *testing.T) {
	ctx := context.Background()
	s, _ := withDocument(t, bodyXML(`<w:p><w:fldSimple w:instr="PAGEref Target \h"><w:r><w:rPr><w:b/></w:rPr><w:t>2</w:t></w:r></w:fldSimple></w:p><w:p><w:bookmarkStart w:id="1" w:name="Target"/><w:r><w:t>target text</w:t></w:r><w:bookmarkEnd w:id="1"/></w:p>`))
	v := mustSnapshot(t, s)
	if run := v.Blocks[0].Paragraph.Runs[0]; run.Link == nil || !run.Link.PageReference || run.Text != "2" {
		t.Fatalf("native result was replaced with the target text: %+v", run)
	}
	after, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{
		Type: "update_reference", TargetID: v.Bookmarks[0].ParagraphID,
		Reference: &ReferenceEdit{Action: "delete_bookmark", Bookmark: "Target"},
	}}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	if run := after.Blocks[0].Paragraph.Runs[0]; run.Link != nil || !run.Bold || run.Text != "2" {
		t.Fatalf("native result or formatting changed on unlink: %+v", run)
	}
}
