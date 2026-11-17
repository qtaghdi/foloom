package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDOCXMovesSelfContainedBookmarkWithParagraph(t *testing.T) {
	ctx := context.Background()
	marked := `<w:p><w:bookmarkStart w:id="1" w:name="MoveMe"/><w:r><w:rPr><w:b/></w:rPr><w:t>marked</w:t></w:r><w:bookmarkEnd w:id="1"/></w:p>`
	first := `<w:p><w:r><w:t>first</w:t></w:r></w:p>`
	boundary := `<w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:t>next page</w:t></w:r></w:p>`
	s, filename := withDocument(t, bodyXML(first+marked+boundary))
	original := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	if !v.Blocks[1].Paragraph.MoveBookmarkUp || v.Blocks[1].Paragraph.MoveBookmarkDown {
		t.Fatal("move availability ignored a page boundary")
	}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "move_bookmark_down", TargetID: v.Blocks[1].ID},
	}}
	if _, err := s.Apply(ctx, input, "human"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("move across a page boundary changed the package")
	}
	input.Operations[0].Type = "move_bookmark_up"
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(s.pkg.document), marked+first+boundary) {
		t.Fatal("movement changed paragraph XML")
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	next := mustSnapshot(t, reopened)
	if len(next.Bookmarks) != 1 || next.Bookmarks[0].ParagraphID != next.Blocks[0].ID {
		t.Fatal("bookmark no longer points to its moved paragraph")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo changed package bytes")
	}
}

func TestDOCXProtectsBookmarkAcrossParagraphs(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:bookmarkStart w:id="1" w:name="Spanning"/><w:r><w:t>a</w:t></w:r></w:p><w:p><w:r><w:t>b</w:t></w:r><w:bookmarkEnd w:id="1"/></w:p>`))
	original := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	if v.Blocks[0].Paragraph.MoveBookmarkDown || v.Blocks[1].Paragraph.MoveBookmarkUp {
		t.Fatal("cross-paragraph bookmark was eligible for moving")
	}
	if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "move_bookmark_down", TargetID: v.Blocks[0].ID},
	}}, "human"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("cross-paragraph bookmark movement changed the package")
	}
}
