package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func headerSlot(t *testing.T, s Snapshot, index int, kind, variant string) HeaderFooterInfo {
	t.Helper()
	for _, info := range s.Sections[index].HeaderFooters {
		if info.Kind == kind && info.Variant == variant {
			return info
		}
	}
	t.Fatal("missing header/footer slot")
	return HeaderFooterInfo{}
}
func headerBatch(s Snapshot, index int, updates ...HeaderFooterUpdate) ApplyInput {
	return ApplyInput{SessionID: s.ID, BaseRevision: s.Revision, Operations: []Operation{{Type: "update_section", SectionID: s.Sections[index].ID, Section: &SectionFormat{HeaderFooters: updates}}}}
}
func headerDocument(t *testing.T) (*Session, string) {
	return withDocument(t, bodyXML(`<w:p><w:pPr><w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr></w:pPr><w:r><w:t>first</w:t></w:r></w:p><w:p><w:r><w:t>second</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr>`))
}

func TestHeadersUseIndependentPartsAndPreservePreviousLinks(t *testing.T) {
	ctx := context.Background()
	s, path := headerDocument(t)
	v := mustSnapshot(t, s)
	original := fileBytes(t, path)
	title, footer, own := "머리글\n다음 줄", "바닥글", "두 번째 전용"
	first := true
	input := headerBatch(v, 0, HeaderFooterUpdate{Kind: "header", Variant: "default", Text: &title}, HeaderFooterUpdate{Kind: "footer", Variant: "first", Text: &footer})
	input.Operations[0].Section.FirstPage = &first
	input.Operations = append(input.Operations, Operation{Type: "define_style", StyleDefinition: &StyleDefinition{ID: "NewHeading", Name: "heading", Kind: "paragraph"}})
	v, err := s.Apply(ctx, input, "human")
	if err != nil {
		t.Fatal(err)
	}
	if headerSlot(t, v, 0, "header", "default").Text != title || !headerSlot(t, v, 1, "header", "default").Linked || headerSlot(t, v, 1, "header", "default").SourceSectionID != v.Sections[0].ID || !v.Sections[0].FirstPage {
		t.Fatalf("header metadata %+v", v.Sections)
	}
	raw := s.pkg.partData(s.pkg.stylesPath, nil)
	if !strings.Contains(string(raw), "NewHeading") {
		t.Fatal("same-batch style relationship lost")
	}
	preserved := map[string][]byte{}
	for _, part := range s.pkg.parts {
		if strings.Contains(part.header.Name, "header") || strings.Contains(part.header.Name, "footer") {
			preserved[part.header.Name] = append([]byte(nil), part.data...)
		}
	}
	v, err = s.Apply(ctx, headerBatch(v, 1, HeaderFooterUpdate{Kind: "header", Variant: "default", Text: &own}), "human")
	if err != nil {
		t.Fatal(err)
	}
	if headerSlot(t, v, 0, "header", "default").Text != title || headerSlot(t, v, 1, "header", "default").Text != own || headerSlot(t, v, 1, "header", "default").Linked {
		t.Fatal("header edit leaked between sections")
	}
	for name, data := range preserved {
		if !bytes.Equal(data, s.pkg.partData(name, nil)) {
			t.Fatal("shared original part was overwritten")
		}
	}
	v, err = s.Apply(ctx, headerBatch(v, 1, HeaderFooterUpdate{Kind: "header", Variant: "default", LinkPrevious: true}), "human")
	if err != nil {
		t.Fatal(err)
	}
	if !headerSlot(t, v, 1, "header", "default").Linked || headerSlot(t, v, 1, "header", "default").SourceSectionID != v.Sections[0].ID {
		t.Fatal("previous link not restored")
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if !headerSlot(t, mustSnapshot(t, reopened), 1, "header", "default").Linked {
		t.Fatal("link lost on reopen")
	}
	for i := 0; i < 3; i++ {
		historyRestore(t, reopened, "undo")
	}
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("header undo did not restore exact package")
	}
}

func TestInvalidHeaderUpdatesAreAtomic(t *testing.T) {
	text := "safe"
	for _, update := range []HeaderFooterUpdate{{Kind: "invalid", Variant: "default", Text: &text}, {Kind: "header", Variant: "bad", Text: &text}, {Kind: "header", Variant: "default"}, {Kind: "header", Variant: "default", LinkPrevious: true}, {Kind: "header", Variant: "default", LinkPrevious: true, Text: &text}} {
		s, path := headerDocument(t)
		v := mustSnapshot(t, s)
		before := fileBytes(t, path)
		_, err := s.Apply(context.Background(), headerBatch(v, 0, update), "human")
		if err == nil || !bytes.Equal(before, fileBytes(t, path)) {
			t.Fatalf("invalid header mutated: %+v", update)
		}
	}
}

func TestFormattedInheritedHeaderIsProtected(t *testing.T) {
	s, path := headerDocument(t)
	v := mustSnapshot(t, s)
	text := "original"
	v, err := s.Apply(context.Background(), headerBatch(v, 0, HeaderFooterUpdate{Kind: "header", Variant: "default", Text: &text}), "human")
	if err != nil {
		t.Fatal(err)
	}
	rewritePackage(t, path, func(name string, data []byte) []byte {
		if strings.HasPrefix(name, "word/header") {
			return bytes.Replace(data, []byte(`<w:r>`), []byte(`<w:r><w:rPr><w:b/></w:rPr>`), 1)
		}
		return data
	}, nil)
	s, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, s)
	before := fileBytes(t, path)
	if headerSlot(t, v, 1, "header", "default").Editable {
		t.Fatal("formatted inherited header marked editable")
	}
	_, err = s.Apply(context.Background(), headerBatch(v, 1, HeaderFooterUpdate{Kind: "header", Variant: "default", Text: &text}), "human")
	if err == nil || !bytes.Equal(before, fileBytes(t, path)) {
		t.Fatal("protected inherited header flattened")
	}
}

func TestPageNumberFieldPersistsAndRespectsSectionLinks(t *testing.T) {
	ctx := context.Background()
	s, filename := headerDocument(t)
	before := fileBytes(t, filename)
	initial := mustSnapshot(t, s)
	yes, no := true, false
	text := "Page marker"
	created, err := s.Apply(ctx, headerBatch(initial, 0, HeaderFooterUpdate{Kind: "footer", Variant: "default", Text: &text, PageNumber: &yes}), "human")
	if err != nil {
		t.Fatal(err)
	}
	firstFooter := headerSlot(t, created, 0, "footer", "default")
	secondFooter := headerSlot(t, created, 1, "footer", "default")
	if !firstFooter.PageNumber || firstFooter.Text != text || !secondFooter.PageNumber || !secondFooter.Linked {
		t.Fatal("field or inherited footer not exposed")
	}
	root, err := parseXML(s.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	section := inspectSections(root, created.ID, created.Revision)[0]
	ref, err := headerReference(section.props, "footer", "default")
	if err != nil {
		t.Fatal(err)
	}
	part := s.pkg.relatedPart(attribute(ref, officeRelNS, "id"), "footer", nil)
	originalFooter := bytes.Clone(s.pkg.partData(part, nil))
	if !bytes.Contains(originalFooter, []byte(`w:instr="PAGE"`)) {
		t.Fatal("PAGE field missing from saved footer")
	}
	text = "Revised text"
	retained, err := s.Apply(ctx, headerBatch(created, 0, HeaderFooterUpdate{Kind: "footer", Variant: "default", Text: &text}), "human")
	if err != nil || !headerSlot(t, retained, 0, "footer", "default").PageNumber {
		t.Fatal("existing text edit cleared page number", err)
	}
	removed, err := s.Apply(ctx, headerBatch(retained, 0, HeaderFooterUpdate{Kind: "footer", Variant: "default", Text: &text, PageNumber: &no}), "human")
	if err != nil || headerSlot(t, removed, 0, "footer", "default").PageNumber ||
		!bytes.Equal(originalFooter, s.pkg.partData(part, nil)) {
		t.Fatal("removing field changed a previously shared part", err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	if headerSlot(t, mustSnapshot(t, reopened), 0, "footer", "default").PageNumber {
		t.Fatal("removal did not survive reopen")
	}
	for range 3 {
		historyRestore(t, reopened, "undo")
	}
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo changed original file")
	}
}

func TestTotalPageFieldKeepsPageNumberAndSectionLinks(t *testing.T) {
	ctx := context.Background()
	s, filename := headerDocument(t)
	before := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	yes, no := true, false
	text := "Footer"
	both, err := s.Apply(ctx, headerBatch(v, 0, HeaderFooterUpdate{
		Kind: "footer", Variant: "default", Text: &text, PageNumber: &yes, TotalPages: &yes,
	}), "human")
	if err != nil {
		t.Fatal(err)
	}
	for index := range 2 {
		slot := headerSlot(t, both, index, "footer", "default")
		if !slot.PageNumber || !slot.TotalPages || !slot.Editable || slot.Linked != (index > 0) {
			t.Fatalf("section %d lost a field or link: %+v", index, slot)
		}
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	view := mustSnapshot(t, reopened)
	if first := headerSlot(t, view, 0, "footer", "default"); first.Text != text || !first.TotalPages {
		t.Fatal("page count field did not survive reopen")
	}
	withoutCount, err := reopened.Apply(ctx, headerBatch(view, 0, HeaderFooterUpdate{
		Kind: "footer", Variant: "default", Text: &text, TotalPages: &no,
	}), "human")
	if err != nil {
		t.Fatal(err)
	}
	if slot := headerSlot(t, withoutCount, 0, "footer", "default"); !slot.PageNumber || slot.TotalPages {
		t.Fatal("removing the total affected the independent PAGE field")
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original document")
	}
}
