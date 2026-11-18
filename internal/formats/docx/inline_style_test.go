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
	for _, style := range []*InlineStyle{{Color: "url(bad)"}, {Font: "bad\nfont"}, {EastAsiaFont: "bad\nfont"}, {EastAsiaFont: strings.Repeat("한", 41)}, {Font: strings.Repeat("x", 121)}, {SizeHalfPoints: &size}} {
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

func TestEastAsianFontChangesOnlySelectedSlot(t *testing.T) {
	ctx := context.Background()
	source := bodyXML(`<w:p><w:r><w:rPr><w:rFonts w:ascii="Latin" w:hAnsiTheme="minorHAnsi" w:eastAsia="원본" w:eastAsiaTheme="majorEastAsia" w:cs="Arabic" w:hint="eastAsia"/><w:lang w:val="en-US" w:eastAsia="ko-KR"/><w:i/></w:rPr><w:t>Latin 한글</w:t></w:r></w:p>`)
	s, path := withDocument(t, source)
	before := fileBytes(t, path)
	snapshot := mustSnapshot(t, s)
	p := snapshot.Blocks[0].Paragraph
	input := paragraphPlan(snapshot, ParagraphContent{SourceID: p.ID, Runs: []RunContent{
		{SourceID: p.Runs[0].ID, Text: "Latin "},
		{SourceID: p.Runs[0].ID, Text: "한글", Style: &InlineStyle{EastAsiaFont: "새 글꼴 & 것"}},
	}})
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	runs := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs
	if len(runs) != 2 || runs[0].Font != "Latin" || runs[0].EastAsiaFont != "" ||
		runs[1].Font != "Latin" || runs[1].EastAsiaFont != "새 글꼴 & 것" || !runs[1].Italic {
		t.Fatalf("unexpected reopened format: %+v", runs)
	}
	root, err := parseXML(reopened.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	xmlRuns := root.child("body").child("p").children
	oldFonts := xmlRuns[0].child("rPr").child("rFonts")
	newProps := xmlRuns[1].child("rPr")
	newFonts := newProps.child("rFonts")
	for _, name := range []string{"ascii", "hAnsiTheme", "cs", "hint"} {
		if oldFonts.attr(name) == "" || newFonts.attr(name) != oldFonts.attr(name) {
			t.Fatalf("unexpected change to %s", name)
		}
	}
	if oldFonts.attr("eastAsia") != "원본" || oldFonts.attr("eastAsiaTheme") != "majorEastAsia" ||
		newFonts.attr("eastAsia") != "새 글꼴 & 것" || newFonts.attr("eastAsiaTheme") != "" ||
		newProps.child("lang").attr("eastAsia") != "ko-KR" {
		t.Fatal("selective East Asian font override did not preserve the source and language")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, path)) {
		t.Fatal("undo failed to restore source bytes")
	}
}

func TestLatinFontKeepsOtherSlotsAndOnlyPatchesSelectedRun(t *testing.T) {
	ctx := context.Background()
	source := bodyXML(`<w:p><w:r><w:rPr><w:rFonts w:ascii="Old ASCII" w:hAnsi="Old ANSI" w:asciiTheme="majorAscii" w:hAnsiTheme="majorHAnsi" w:eastAsia="East" w:eastAsiaTheme="minorEastAsia" w:cs="Arabic" w:cstheme="minorBidi" w:hint="eastAsia"/><w:i/></w:rPr><w:t>Latin 한글</w:t></w:r></w:p>`)
	s, filename := withDocument(t, source)
	original := fileBytes(t, filename)
	before := mustSnapshot(t, s)
	p := before.Blocks[0].Paragraph
	input := paragraphPlan(before, ParagraphContent{SourceID: p.ID, Runs: []RunContent{
		{SourceID: p.Runs[0].ID, Text: "Latin", Style: &InlineStyle{LatinFont: "New Latin & Co"}},
		{SourceID: p.Runs[0].ID, Text: " 한글"},
	}})
	_, err := s.Apply(ctx, input, "human")
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseXML(reopened.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	runs := root.child("body").child("p").children
	selected, outside := runs[0].child("rPr").child("rFonts"), runs[1].child("rPr").child("rFonts")
	if selected.attr("ascii") != "New Latin & Co" || selected.attr("hAnsi") != "New Latin & Co" ||
		selected.attr("asciiTheme") != "" || selected.attr("hAnsiTheme") != "" ||
		outside.attr("ascii") != "Old ASCII" || outside.attr("hAnsi") != "Old ANSI" ||
		outside.attr("asciiTheme") != "majorAscii" || outside.attr("hAnsiTheme") != "majorHAnsi" {
		t.Fatal("incorrect selected or unselected Latin font attributes")
	}
	for _, name := range []string{"eastAsia", "eastAsiaTheme", "cs", "cstheme", "hint"} {
		if selected.attr(name) == "" || selected.attr(name) != outside.attr(name) {
			t.Fatalf("other font attribute changed: %s", name)
		}
	}
	after := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs
	if len(after) != 2 || after[0].Font != "New Latin & Co" || !after[0].Italic || !after[1].Italic {
		t.Fatalf("unexpected saved text or style: %+v", after)
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore original bytes")
	}
}
