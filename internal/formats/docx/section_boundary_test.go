package docx

import (
	"bytes"
	"context"
	"testing"
)

func TestBoundarySeparatesPageBreaksAndSectionLayout(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>first</w:t></w:r></w:p><w:p><w:r><w:t>second</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgNumType w:start="25" w:fmt="upperRoman"/></w:sectPr>`))
	original := fileBytes(t, filename)
	apply := func(index int, mode string) (Snapshot, error) {
		v := mustSnapshot(t, s)
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "update_boundary", TargetID: v.Blocks[index].ID, Boundary: mode}}}, "human")
	}
	if _, err := apply(1, "section_next_page"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("empty trailing section was accepted")
	}
	v, err := apply(1, "page_before")
	if err != nil || len(v.Sections) != 1 || !v.Blocks[1].Paragraph.PageBreakBefore {
		t.Fatalf("page break: %v, %+v", err, v)
	}
	v, err = apply(1, "no_page_before")
	if err != nil || v.Blocks[1].Paragraph.PageBreakBefore {
		t.Fatalf("page break removal: %v, %+v", err, v)
	}
	v, err = apply(0, "section_continuous")
	if err != nil || len(v.Sections) != 2 || v.Sections[0].EndBlock != 0 || v.Sections[1].BreakType != "continuous" ||
		v.Sections[0].PageNumberStart == nil || *v.Sections[0].PageNumberStart != 25 || v.Sections[1].PageNumberStart != nil ||
		v.Sections[1].PageNumberFormat != "upperRoman" || v.Sections[1].Page != v.Sections[0].Page {
		t.Fatalf("section split: %v, %+v", err, v.Sections)
	}
	if _, err = apply(0, "section_next_page"); err == nil {
		t.Fatal("split of an existing boundary was accepted")
	}
	historyRestore(t, s, "undo")
	historyRestore(t, s, "undo")
	historyRestore(t, s, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo changed original bytes")
	}
}

func TestRemoveTrailingPageBreakPreservesRunAndSection(t *testing.T) {
	ctx := context.Background()
	s, path := withDocument(t, bodyXML(`<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>first</w:t><w:br w:type="page"/></w:r></w:p><w:p><w:r><w:br w:type="page"/><w:t>middle</w:t></w:r></w:p><w:p><w:r><w:t>opaque</w:t><w:br w:type="page" w:clear="all"/></w:r></w:p><w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr>`))
	original := fileBytes(t, path)
	expected := bytes.Replace(s.pkg.document, []byte(`<w:br w:type="page"/>`), nil, 1)
	before := mustSnapshot(t, s)
	remove := func(block int) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: before.ID, BaseRevision: before.Revision, Operations: []Operation{
			{Type: "update_boundary", TargetID: before.Blocks[block].ID, Boundary: "no_page_after"},
		}}, "human")
	}
	for _, block := range []int{1, 2} {
		if _, err := remove(block); err == nil || !bytes.Equal(original, fileBytes(t, path)) {
			t.Fatal("unsupported break removal changed the file")
		}
	}
	after, err := remove(0)
	if err != nil || after.Blocks[0].Paragraph.PageBreakAfter || !bytes.Equal(expected, s.pkg.document) {
		t.Fatalf("trailing break removal changed other XML: %v", err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if mustSnapshot(t, reopened).Blocks[0].Paragraph.PageBreakAfter {
		t.Fatal("trailing break still present after reopening")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("undo did not restore original bytes")
	}
}
