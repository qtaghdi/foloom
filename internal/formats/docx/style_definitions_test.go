package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestStyleDefinitionsAndSelectionApplyShareOneBatch(t *testing.T) {
	s, path := createTest(t)
	original := fileBytes(t, path)
	v := mustSnapshot(t, s)
	p := v.Blocks[0].Paragraph
	yes := true
	size := 32
	base, heading, accent := "Body", "Heading", "Accent"
	plan := paragraphPlan(Snapshot{ID: v.ID, Revision: v.Revision, Blocks: v.Blocks[:1]}, ParagraphContent{SourceID: p.ID, Format: &ParagraphFormat{StyleID: &heading}, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "styled", Style: &InlineStyle{StyleID: &accent}}}})
	plan.Operations = append([]Operation{
		{Type: "define_style", StyleDefinition: &StyleDefinition{ID: base, Name: "본문", Kind: "paragraph", Run: &InlineStyle{Bold: &yes, Font: "한글 Font"}}},
		{Type: "define_style", StyleDefinition: &StyleDefinition{ID: heading, Name: "제목", Kind: "paragraph", BasedOn: &base, Run: &InlineStyle{SizeHalfPoints: &size}, Paragraph: &ParagraphFormat{Align: "center"}}},
		{Type: "define_style", StyleDefinition: &StyleDefinition{ID: accent, Name: "강조", Kind: "character", Run: &InlineStyle{Underline: &yes, Color: "#123456"}}},
	}, plan.Operations...)
	v, err := s.Apply(context.Background(), plan, "human")
	if err != nil {
		t.Fatal(err)
	}
	r := v.Blocks[0].Paragraph.Runs[0]
	if v.Revision != 2 || len(v.Styles) != 3 || v.Blocks[0].Paragraph.StyleID != heading || r.CharacterStyleID != accent || !r.Bold || !r.Underline || r.SizeHalfPoints != 32 || r.Font != "한글 Font" || r.Color != "#123456" {
		t.Fatalf("style application mismatch %+v %+v", v.Styles, r)
	}
	if v.Blocks[0].Paragraph.Layout.Align != "center" {
		t.Fatal("paragraph style not inherited")
	}
	if s.pkg.stylesPath != "word/styles.xml" {
		t.Fatal("new style mapping missing")
	}
	// Alter a base definition without flattening the derived style or character override.
	v, err = s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "define_style", StyleDefinition: &StyleDefinition{ID: base, Kind: "paragraph", Run: &InlineStyle{Font: "새 글꼴"}}}}}, "ai")
	if err != nil {
		t.Fatal(err)
	}
	r = v.Blocks[0].Paragraph.Runs[0]
	if r.Font != "새 글꼴" || r.SizeHalfPoints != 32 || !r.Underline {
		t.Fatal("style modification lost inheritance", r)
	}
	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(mustSnapshot(t, reopened).Styles) != 3 {
		t.Fatal("style catalog lost after reopen")
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("style undo did not restore original whole package")
	}
	historyRestore(t, reopened, "redo")
	if mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[0].CharacterStyleID != accent {
		t.Fatal("style redo failed")
	}
}
func TestStyleDefinitionsRejectCyclesUnknownAndWrongKindAtomically(t *testing.T) {
	s, path := createTest(t)
	v := mustSnapshot(t, s)
	base, derived := "Base", "Derived"
	v, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "define_style", StyleDefinition: &StyleDefinition{ID: base, Name: "Base", Kind: "paragraph"}},
		{Type: "define_style", StyleDefinition: &StyleDefinition{ID: derived, Name: "Derived", Kind: "paragraph", BasedOn: &base}},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	before := fileBytes(t, path)
	for _, d := range []StyleDefinition{{ID: base, Kind: "paragraph", BasedOn: &derived}, {ID: "UnknownBase", Name: "x", Kind: "paragraph", BasedOn: newString("missing")}, {ID: base, Kind: "character"}, {ID: "Bad", Name: "bad", Kind: "character", Paragraph: &ParagraphFormat{Align: "center"}}} {
		_, err = s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "define_style", StyleDefinition: &d}}}, "human")
		if err == nil {
			t.Fatal("invalid style accepted", d)
		}
		if !bytes.Equal(before, fileBytes(t, path)) || mustSnapshot(t, s).Revision != v.Revision {
			t.Fatal("invalid style mutated file")
		}
	}
	p := v.Blocks[0].Paragraph
	input := paragraphPlan(Snapshot{ID: v.ID, Revision: v.Revision, Blocks: v.Blocks[:1]}, ParagraphContent{SourceID: p.ID, Format: &ParagraphFormat{StyleID: newString("missing")}, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "wrong"}}})
	if _, err = s.Apply(context.Background(), input, "human"); err == nil {
		t.Fatal("unknown style reference accepted")
	}
	input.Operations[0].Paragraphs[0].Format.StyleID = &base
	input.Operations[0].Paragraphs[0].Runs[0].Style = &InlineStyle{StyleID: &base}
	if _, err = s.Apply(context.Background(), input, "human"); err == nil {
		t.Fatal("paragraph style applied as character style")
	}
}
func newString(s string) *string { return &s }
func TestStyleDefinitionsPreserveAliasedPartAndOpaqueMetadata(t *testing.T) {
	_, path := createTest(t)
	styles := []byte(`<q:styles xmlns:q="` + wordNS + `"><q:style q:type="paragraph" q:styleId="Body" q:customStyle="1"><q:name q:val="Old"/><q:uiPriority q:val="7"/><q:rPr><q:lang q:val="ko-KR"/></q:rPr></q:style></q:styles>`)
	rels := []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="styles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="design.xml"/></Relationships>`)
	rewritePackage(t, path, func(_ string, b []byte) []byte { return b }, map[string][]byte{"word/design.xml": styles, "word/_rels/document.xml.rels": rels, "customXml/keep.xml": []byte(`<keep>untouched</keep>`)})
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	v := mustSnapshot(t, s)
	old := s.pkg
	yes := true
	_, err = s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "define_style", StyleDefinition: &StyleDefinition{ID: "Body", Name: "New", Kind: "paragraph", Run: &InlineStyle{Bold: &yes}}}}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	if s.pkg.stylesPath != "word/design.xml" {
		t.Fatal("style part mapping changed")
	}
	for i, part := range old.parts {
		if part.header.Name != "word/design.xml" && !bytes.Equal(part.data, s.pkg.parts[i].data) {
			t.Fatal("unrelated part changed", part.header.Name)
		}
	}
	for _, keep := range []string{`q:customStyle="1"`, `<q:uiPriority q:val="7"/>`, `<q:lang q:val="ko-KR"/>`} {
		if !strings.Contains(string(s.pkg.styleData()), keep) {
			t.Fatal("style metadata lost", keep)
		}
	}
}
func TestStyleCreationHandlesSelfClosingRoot(t *testing.T) {
	data := []byte(`<w:styles xmlns:w="` + wordNS + `"/>`)
	root, err := parseXML(data)
	if err != nil {
		t.Fatal(err)
	}
	out, err := patchStyle(root, data, StyleDefinition{ID: "Custom", Name: "새 스타일", Kind: "paragraph"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseXML(out)
	if err != nil || parsed.child("style") == nil {
		t.Fatalf("invalid style root %s %v", out, err)
	}
}
