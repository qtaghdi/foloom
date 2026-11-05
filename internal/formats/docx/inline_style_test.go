package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestClipboardInlineStylePreservesOtherProperties(t *testing.T) {
	s, path := withDocument(t, bodyXML(`<w:p><w:r><w:rPr><w:b/><w:bCs/><w:u w:val="single"/><w:lang w:val="ko-KR"/></w:rPr><w:t>source</w:t></w:r></w:p>`))
	before, err := loadPackage(fileBytes(t, path))
	if err != nil {
		t.Fatal(err)
	}
	snap := mustSnapshot(t, s)
	p := snap.Blocks[0].Paragraph
	no, yes, size := false, true, 28
	input := paragraphPlan(snap, ParagraphContent{SourceID: p.ID, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "pasted & 한글", Style: &InlineStyle{Bold: &no, Italic: &yes, Color: "#112233", Font: "한글 Font & <>", SizeHalfPoints: &size}}}})
	after, err := s.Apply(context.Background(), input, "human")
	if err != nil {
		t.Fatal(err)
	}
	run := after.Blocks[0].Paragraph.Runs[0]
	if run.Bold || !run.Italic || run.Color != "#112233" || run.SizeHalfPoints != 28 || run.Font != "한글 Font & <>" {
		t.Fatalf("style did not round trip: %+v", run)
	}
	xml := string(s.pkg.document)
	for _, preserved := range []string{`<w:u w:val="single"/>`, `<w:lang w:val="ko-KR"/>`, `dw:bCs`, `dw:val="0"`} {
		if !strings.Contains(xml, preserved) {
			t.Fatalf("style property lost: %s", preserved)
		}
	}
	for i, part := range before.parts {
		if part.header.Name != "word/document.xml" && !bytes.Equal(part.data, s.pkg.parts[i].data) {
			t.Fatal("unrelated part changed")
		}
	}
	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !mustSnapshot(t, reopened).Blocks[0].Paragraph.StructureEditable {
		t.Fatal("pasted style stopped paragraph editing")
	}
}
func TestInvalidClipboardStyleRejectsWholePlan(t *testing.T) {
	size := 500
	for _, style := range []*InlineStyle{{Color: "url(bad)"}, {Font: "bad\nfont"}, {Font: strings.Repeat("x", 121)}, {SizeHalfPoints: &size}} {
		s, path := createTest(t)
		snap := mustSnapshot(t, s)
		p := snap.Blocks[0].Paragraph
		before := fileBytes(t, path)
		input := ApplyInput{SessionID: snap.ID, BaseRevision: snap.Revision, Operations: []Operation{{Type: "replace_paragraphs", TargetID: p.ID, EndTargetID: p.ID, ExpectedText: "hello", Paragraphs: []ParagraphContent{{SourceID: p.ID, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "new", Style: style}}}}}}}
		if _, err := s.Apply(context.Background(), input, "human"); err == nil {
			t.Fatal("invalid clipboard style accepted")
		}
		if !bytes.Equal(before, fileBytes(t, path)) || mustSnapshot(t, s).Revision != 1 {
			t.Fatal("invalid style mutated document")
		}
	}
}
