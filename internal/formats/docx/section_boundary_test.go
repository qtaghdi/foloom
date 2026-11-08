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
