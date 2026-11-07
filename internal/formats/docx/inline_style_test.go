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

func TestDirectFormattingAndResetPreserveLanguageAndStyle(t *testing.T) {
	s, path := withDocument(t, bodyXML(`<w:p><w:r><w:rPr><w:rStyle w:val="Emphasis"/><w:rFonts w:ascii="Latin" w:eastAsia="한글"/><w:lang w:val="en-US" w:eastAsia="ko-KR"/><w:u w:val="double"/><w:strike/></w:rPr><w:t>before after</w:t></w:r></w:p>`))
	original := fileBytes(t, path)
	snap := mustSnapshot(t, s)
	p := snap.Blocks[0].Paragraph
	yes := true
	input := paragraphPlan(snap, ParagraphContent{SourceID: p.ID, Runs: []RunContent{
		{SourceID: p.Runs[0].ID, Text: "before "},
		{SourceID: p.Runs[0].ID, Text: "after", Style: &InlineStyle{Underline: &yes, Strike: &yes, Highlight: "yellow", Vertical: "superscript"}},
	}})
	v, err := s.Apply(context.Background(), input, "human")
	if err != nil {
		t.Fatal(err)
	}
	r := v.Blocks[0].Paragraph.Runs[1]
	if !r.Underline || !r.Strike || r.Highlight != "yellow" || r.Vertical != "superscript" {
		t.Fatalf("format preview mismatch %+v", r)
	}
	if !strings.Contains(string(s.pkg.document), `w:eastAsia="한글"`) || !strings.Contains(string(s.pkg.document), `w:eastAsia="ko-KR"`) {
		t.Fatal("language fonts changed")
	}
	p = v.Blocks[0].Paragraph
	input = paragraphPlan(v, ParagraphContent{SourceID: p.ID, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "before "}, {SourceID: p.Runs[1].ID, Text: "after", Style: &InlineStyle{Reset: true}}}})
	v, err = s.Apply(context.Background(), input, "human")
	if err != nil {
		t.Fatal(err)
	}
	r = v.Blocks[0].Paragraph.Runs[1]
	if r.Underline || r.Strike || r.Highlight != "" || r.Vertical != "" || r.Font != "" {
		t.Fatal("direct formatting not cleared", r)
	}
	if strings.Count(string(s.pkg.document), `w:rStyle w:val="Emphasis"`) < 2 || strings.Count(string(s.pkg.document), `w:lang w:val="en-US"`) < 2 {
		t.Fatal("reset discarded style or language")
	}
	historyRestore(t, s, "undo")
	historyRestore(t, s, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("format undo lost original package")
	}
}
func TestInvalidExtendedInlineFormatIsAtomic(t *testing.T) {
	for _, style := range []*InlineStyle{{Highlight: "url(file)"}, {Vertical: "bad"}} {
		s, path := createTest(t)
		v := mustSnapshot(t, s)
		p := v.Blocks[0].Paragraph
		before := fileBytes(t, path)
		input := paragraphPlan(Snapshot{ID: v.ID, Revision: v.Revision, Blocks: v.Blocks[:1]}, ParagraphContent{SourceID: p.ID, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "bad", Style: style}}})
		if _, err := s.Apply(context.Background(), input, "human"); err == nil {
			t.Fatal("invalid style accepted")
		}
		if !bytes.Equal(before, fileBytes(t, path)) || mustSnapshot(t, s).Revision != v.Revision {
			t.Fatal("invalid format mutated file")
		}
	}
}

func TestStrikeStyleToggleAndDirectOverride(t *testing.T) {
	root, err := parseXML([]byte(`<w:rPr xmlns:w="` + wordNS + `"><w:strike/></w:rPr>`))
	if err != nil {
		t.Fatal(err)
	}
	r := Run{}
	styleRun(&r, root)
	if !r.Strike {
		t.Fatal("style did not enable strike")
	}
	styleRun(&r, root)
	if r.Strike {
		t.Fatal("inherited style did not toggle strike")
	}
	directRun(&r, root)
	if !r.Strike {
		t.Fatal("direct strike did not override style")
	}
}
