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

func TestRemoveSectionBoundaryKeepsFollowingSettingsAndText(t *testing.T) {
	ctx := context.Background()
	const boundary = `<w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr>`
	const following = `<w:sectPr><w:pgSz w:w="15840" w:h="12240" w:orient="landscape"/><w:cols w:num="2"/></w:sectPr>`
	s, path := withDocument(t, bodyXML(`<w:p><w:pPr><w:keepNext/>`+boundary+`</w:pPr><w:r><w:t>first</w:t></w:r></w:p><w:p><w:r><w:t>next</w:t></w:r></w:p>`+following))
	original := fileBytes(t, path)
	expected := bytes.Replace(s.pkg.document, []byte(boundary), nil, 1)
	before := mustSnapshot(t, s)
	next, err := s.Apply(ctx, ApplyInput{SessionID: before.ID, BaseRevision: before.Revision, Operations: []Operation{
		{Type: "update_boundary", TargetID: before.Blocks[0].ID, Boundary: "remove_section"},
	}}, "human")
	if err != nil || len(next.Sections) != 1 || next.Sections[0].Orientation != "landscape" || next.Sections[0].Columns != 2 || !bytes.Equal(expected, s.pkg.document) {
		t.Fatalf("section removal altered other content: %v", err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(mustSnapshot(t, reopened).Sections) != 1 {
		t.Fatal("section boundary retained after reopening")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("undo did not restore original section")
	}
}

func TestMoveSectionBoundaryPreservesSettingsAndRequiresContentBothSides(t *testing.T) {
	ctx := context.Background()
	const first = `<w:p><w:r><w:t>first</w:t></w:r></w:p>`
	const source = `<w:p><w:pPr><w:keepNext/><w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgNumType w:start="12" w:fmt="upperRoman"/></w:sectPr></w:pPr><w:r><w:t>boundary</w:t></w:r></w:p>`
	const next = `<w:p><w:pPr/><w:r><w:t>next</w:t></w:r></w:p>`
	const final = `<w:p><w:r><w:t>last</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="15840" w:h="12240" w:orient="landscape"/></w:sectPr>`
	for _, tc := range []struct {
		mode string
		end  int
	}{
		{"move_section_earlier", 0},
		{"move_section_later", 2},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			s, filename := withDocument(t, bodyXML(first+source+next+final))
			original := fileBytes(t, filename)
			snapshot := mustSnapshot(t, s)
			move := func(v Snapshot, index int, mode string) (Snapshot, error) {
				return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
					{Type: "update_boundary", TargetID: v.Blocks[index].ID, Boundary: mode},
				}}, "human")
			}
			result, err := move(snapshot, 1, tc.mode)
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(ctx, filename)
			if err != nil {
				t.Fatal(err)
			}
			result = mustSnapshot(t, reopened)
			if len(result.Sections) != 2 || result.Sections[0].EndBlock != tc.end ||
				result.Sections[0].Page != snapshot.Sections[0].Page || result.Sections[0].PageNumberStart == nil ||
				*result.Sections[0].PageNumberStart != 12 || result.Sections[0].PageNumberFormat != "upperRoman" ||
				result.Sections[1].Orientation != "landscape" {
				t.Fatalf("move changed original settings or section boundary: %+v", result.Sections)
			}
			for i, text := range []string{"first", "boundary", "next", "last"} {
				if result.Blocks[i].Paragraph.Runs[0].Text != text {
					t.Fatal("move changed body content")
				}
			}
			current := fileBytes(t, filename)
			if _, err := move(mustSnapshot(t, s), tc.end, tc.mode); err == nil || !bytes.Equal(current, fileBytes(t, filename)) {
				t.Fatal("a section with no body content was allowed")
			}
			historyRestore(t, reopened, "undo")
			if !bytes.Equal(original, fileBytes(t, filename)) {
				t.Fatal("undo changed original bytes")
			}
		})
	}
}

func TestInsertParitySectionBoundary(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct{ mode, kind string }{
		{"section_even_page", "evenPage"},
		{"section_odd_page", "oddPage"},
	} {
		t.Run(test.kind, func(t *testing.T) {
			const paragraph = `<w:p><w:r><w:t>first</w:t></w:r></w:p><w:p><w:r><w:t>next</w:t></w:r></w:p>`
			const layout = `<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgNumType w:start="3" w:fmt="upperRoman"/></w:sectPr>`
			s, path := withDocument(t, bodyXML(paragraph+layout))
			original := fileBytes(t, path)
			before := mustSnapshot(t, s)
			input := ApplyInput{SessionID: before.ID, BaseRevision: before.Revision, Operations: []Operation{
				{Type: "update_boundary", TargetID: before.Blocks[1].ID, Boundary: test.mode},
			}}
			if _, err := s.Apply(ctx, input, "ai"); err == nil || !bytes.Equal(original, fileBytes(t, path)) {
				t.Fatal("inserting an empty trailing section changed the file")
			}
			input.Operations[0].TargetID = before.Blocks[0].ID
			if _, err := s.Apply(ctx, input, "ai"); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			after := mustSnapshot(t, reopened)
			if len(after.Sections) != 2 {
				t.Fatalf("expected two sections: %+v", after.Sections)
			}
			left, right := after.Sections[0], after.Sections[1]
			if left.EndBlock != 0 || right.StartBlock != 1 || right.BreakType != test.kind ||
				left.PageNumberStart == nil || *left.PageNumberStart != 3 || right.PageNumberStart != nil ||
				right.PageNumberFormat != left.PageNumberFormat || right.Page != left.Page ||
				after.Blocks[0].Paragraph.Runs[0].Text != "first" || after.Blocks[1].Paragraph.Runs[0].Text != "next" {
				t.Fatalf("section boundary or retained settings differ: %+v", after.Sections)
			}
			historyRestore(t, reopened, "undo")
			if !bytes.Equal(original, fileBytes(t, path)) {
				t.Fatal("undo did not restore original bytes")
			}
		})
	}
}
