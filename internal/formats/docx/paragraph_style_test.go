package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestParagraphFormattingPreservesUnchangedProperties(t *testing.T) {
	s, path := withDocument(t, bodyXML(`<w:p><w:pPr><w:tabs><w:tab w:val="left" w:pos="720"/></w:tabs><w:spacing w:before="10" w:beforeLines="100" w:after="20"/><w:ind w:start="100" w:startChars="200" w:hanging="100"/><w:jc w:val="left"/></w:pPr><w:r><w:rPr><w:lang w:val="ko-KR"/></w:rPr><w:t>문단</w:t></w:r></w:p>`))
	original := fileBytes(t, path)
	v := mustSnapshot(t, s)
	p := v.Blocks[0].Paragraph
	left, first, before, line := 720, 240, 120, 360
	rtl := true
	v, err := s.Apply(context.Background(), paragraphPlan(v, ParagraphContent{SourceID: p.ID, Format: &ParagraphFormat{Align: "center", Left: &left, FirstLine: &first, Before: &before, Line: &line, LineRule: "auto", Bidi: &rtl, TextDirection: "tbRl"}, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: p.Runs[0].Text}}}), "human")
	if err != nil {
		t.Fatal(err)
	}
	layout := v.Blocks[0].Paragraph.Layout
	if layout == nil || layout.Align != "center" || *layout.Left != 720 || *layout.FirstLine != 240 || layout.Hanging != nil || *layout.Line != 360 || !layout.Bidi || layout.TextDirection != "tbRl" {
		t.Fatalf("preview mismatch %+v", layout)
	}
	data := string(s.pkg.document)
	for _, unchanged := range []string{`<w:tabs><w:tab w:val="left" w:pos="720"/></w:tabs>`, `w:after="20"`, `<w:lang w:val="ko-KR"/>`} {
		if !strings.Contains(data, unchanged) {
			t.Fatalf("lost property %s", unchanged)
		}
	}
	for _, removed := range []string{`w:beforeLines=`, `w:start=`, `w:startChars=`, `w:hanging=`} {
		if strings.Contains(data, removed) {
			t.Fatalf("conflicting attribute retained: %s", removed)
		}
	}
	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !mustSnapshot(t, reopened).Blocks[0].Paragraph.StructureEditable {
		t.Fatal("format blocked further editing")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("paragraph undo did not preserve exact original")
	}
}
func TestParagraphFormattingWithAliasedNamespaces(t *testing.T) {
	s, _ := withDocument(t, bodyXML(`<w:p><w:pPr xmlns:q="`+wordNS+`" xmlns:c="urn:custom"><q:spacing q:before="0" c:keep="opaque"/><w:keepNext/></w:pPr><w:r><w:t>x</w:t></w:r></w:p>`))
	v := mustSnapshot(t, s)
	p := v.Blocks[0].Paragraph
	after := 200
	_, err := s.Apply(context.Background(), paragraphPlan(v, ParagraphContent{SourceID: p.ID, Format: &ParagraphFormat{After: &after}, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "x"}}}), "human")
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseXML(s.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	spacing := root.child("body").child("p").child("pPr").child("spacing")
	if spacing.attr("before") != "0" || spacing.attr("after") != "200" || !strings.Contains(string(s.pkg.document), `c:keep="opaque"`) {
		t.Fatal("namespace property changed")
	}
}
func TestInvalidParagraphFormattingDoesNotMutate(t *testing.T) {
	tooLarge, invalidLine, indent := 3000, 10, 240
	for _, f := range []*ParagraphFormat{{Before: &tooLarge}, {Line: &invalidLine}, {FirstLine: &indent, Hanging: &indent}, {LineRule: "exact"}, {Align: "bad"}, {TextDirection: "bad"}} {
		s, path := withDocument(t, bodyXML(`<w:p><w:r><w:t>x</w:t></w:r></w:p>`))
		v := mustSnapshot(t, s)
		p := v.Blocks[0].Paragraph
		before := fileBytes(t, path)
		_, err := s.Apply(context.Background(), paragraphPlan(v, ParagraphContent{SourceID: p.ID, Format: f, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "y"}}}), "human")
		if err == nil {
			t.Fatal("invalid paragraph format accepted")
		}
		if !bytes.Equal(before, fileBytes(t, path)) || mustSnapshot(t, s).Revision != v.Revision {
			t.Fatal("invalid paragraph format mutated document")
		}
	}
}
