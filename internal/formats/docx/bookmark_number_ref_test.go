package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNumberedBookmarkFieldUsesNativeListAndPreservesDisplayOnDelete(t *testing.T) {
	ctx := context.Background()
	_, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>See </w:t></w:r></w:p><w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="7"/></w:numPr></w:pPr><w:r><w:t>first</w:t></w:r></w:p><w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="7"/></w:numPr></w:pPr><w:bookmarkStart w:id="1" w:name="Clause"/><w:r><w:t>target</w:t></w:r><w:bookmarkEnd w:id="1"/></w:p><w:p><w:bookmarkStart w:id="2" w:name="Plain"/><w:r><w:t>ordinary</w:t></w:r><w:bookmarkEnd w:id="2"/></w:p>`))
	num := []byte(`<w:numbering xmlns:w="` + wordNS + `"><w:abstractNum w:abstractNumId="1"><w:lvl w:ilvl="0"><w:start w:val="4"/><w:numFmt w:val="upperRoman"/><w:lvlText w:val="Appendix %1."/></w:lvl></w:abstractNum><w:num w:numId="7"><w:abstractNumId w:val="1"/></w:num></w:numbering>`)
	rels := []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="lists" Type="` + numberingRelationship + `" Target="numbering.xml"/></Relationships>`)
	rewritePackage(t, filename, func(_ string, data []byte) []byte { return data }, map[string][]byte{
		"word/numbering.xml": num, documentRelationships: rels,
	})
	s, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v := mustSnapshot(t, s)
	if !v.Bookmarks[0].NumberReference || v.Bookmarks[1].NumberReference {
		t.Fatalf("number references advertised for incorrect targets: %+v", v.Bookmarks)
	}
	before := fileBytes(t, filename)
	apply := func(v Snapshot, id string, reference ReferenceEdit) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{
			Type: "update_reference", TargetID: id, Reference: &reference,
		}}}, "human")
	}
	if _, err := apply(v, v.Blocks[0].ID, ReferenceEdit{Action: "insert_bookmark_number_ref", Bookmark: "Plain"}); err == nil {
		t.Fatal("unnumbered bookmark should not be a valid number reference")
	}
	v, err = apply(v, v.Blocks[0].ID, ReferenceEdit{Action: "insert_bookmark_number_ref", Bookmark: "Clause"})
	if err != nil {
		t.Fatal(err)
	}
	if run := v.Blocks[0].Paragraph.Runs[1]; run.Link == nil || run.Text != "Appendix V" ||
		!strings.Contains(string(s.pkg.document), `instr="REF Clause \n \h"`) {
		t.Fatalf("incorrect native numbered bookmark result: %+v", run)
	}
	v, err = apply(v, v.Bookmarks[0].ParagraphID, ReferenceEdit{Action: "rename_bookmark", Bookmark: "Clause", NewBookmark: "Item"})
	if err != nil || v.Blocks[0].Paragraph.Runs[1].Link.Bookmark != "Item" {
		t.Fatal("number reference did not follow the bookmark rename", err)
	}
	s, err = Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, s)
	v, err = apply(v, v.Bookmarks[0].ParagraphID, ReferenceEdit{Action: "delete_bookmark", Bookmark: "Item"})
	if err != nil {
		t.Fatal(err)
	}
	if run := v.Blocks[0].Paragraph.Runs[1]; run.Link != nil || run.Text != "Appendix V" ||
		!bytes.Equal(num, s.pkg.partData("word/numbering.xml", nil)) {
		t.Fatalf("deleting the bookmark changed the saved label or numbering part: %+v", run)
	}
	for range 3 {
		historyRestore(t, s, "undo")
	}
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore original bytes")
	}
}
