package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestBookmarkTextReferencePersistsThroughRenameAndDeletion(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:bookmarkStart w:id="1" w:name="Source"/><w:r><w:t>근거 &amp; 사례</w:t></w:r><w:bookmarkEnd w:id="1"/></w:p><w:p><w:r><w:t>See </w:t></w:r></w:p>`))
	before := fileBytes(t, filename)
	apply := func(v Snapshot, id string, ref ReferenceEdit) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
			{Type: "update_reference", TargetID: id, Reference: &ref},
		}}, "human")
	}
	v := mustSnapshot(t, s)
	if len(v.Bookmarks) != 1 || !v.Bookmarks[0].TextReference {
		t.Fatal("plain bookmarks should be offered as reference targets")
	}
	v, err := apply(v, v.Blocks[1].ID, ReferenceEdit{Action: "insert_bookmark_ref", Bookmark: "Source"})
	if err != nil {
		t.Fatal(err)
	}
	run := v.Blocks[1].Paragraph.Runs[1]
	if run.Editable || run.Link == nil || run.Link.Bookmark != "Source" || run.Text != "근거 & 사례" ||
		!strings.Contains(string(s.pkg.document), `dwr:instr="REF Source \h"`) {
		t.Fatalf("native bookmark field or projected link missing: %+v", run)
	}
	v, err = apply(v, v.Bookmarks[0].ParagraphID, ReferenceEdit{Action: "rename_bookmark", Bookmark: "Source", NewBookmark: "Renamed"})
	if err != nil || v.Blocks[1].Paragraph.Runs[1].Link.Bookmark != "Renamed" ||
		!strings.Contains(string(s.pkg.document), `instr="REF Renamed \h"`) {
		t.Fatal("renaming left a stale bookmark reference", err)
	}
	v, err = apply(v, v.Bookmarks[0].ParagraphID, ReferenceEdit{Action: "delete_bookmark", Bookmark: "Renamed"})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	run = v.Blocks[1].Paragraph.Runs[1]
	if len(v.Bookmarks) != 0 || run.Link != nil || run.Text != "근거 & 사례" || !run.Editable ||
		strings.Contains(string(reopened.pkg.document), "fldSimple") {
		t.Fatal("deleting the bookmark removed its displayed field value")
	}
	for range 3 {
		historyRestore(t, reopened, "undo")
	}
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore exact original bookmark document")
	}
}

func TestDeletingBookmarkRetainsCurrentValueAndFormattedFieldResult(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:bookmarkStart w:id="1" w:name="Source"/><w:r><w:t>current</w:t></w:r><w:bookmarkEnd w:id="1"/></w:p><w:p><w:fldSimple w:instr="REF Source \h"><w:r><w:rPr><w:i/></w:rPr><w:t>stale</w:t></w:r></w:fldSimple></w:p>`))
	v := mustSnapshot(t, s)
	if v.Blocks[1].Paragraph.Runs[0].Text != "current" {
		t.Fatal("bookmark reference should show the current source text")
	}
	if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: v.Bookmarks[0].ParagraphID, Reference: &ReferenceEdit{Action: "delete_bookmark", Bookmark: "Source"}},
	}}, "human"); err != nil {
		t.Fatal(err)
	}
	run := mustSnapshot(t, s).Blocks[1].Paragraph.Runs[0]
	if run.Text != "current" || !run.Italic {
		t.Fatalf("removing the field lost display text or formatting in %s: %+v", filename, run)
	}
}
