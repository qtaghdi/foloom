package docx

import (
	"bytes"
	"context"
	"testing"
)

func TestProofingLanguageChangesOnlySelectedScriptAndRange(t *testing.T) {
	ctx := context.Background()
	source := bodyXML(`<w:p><w:r><w:rPr><w:rFonts w:ascii="Arial" w:eastAsia="Gulim"/><w:i/><w:lang w:val="en-US" w:eastAsia="ko-KR" w:bidi="ar-SA"/></w:rPr><w:t>Latin 日本語</w:t></w:r></w:p>`)
	s, filename := withDocument(t, source)
	before := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	p := v.Blocks[0].Paragraph
	input := paragraphPlan(v, ParagraphContent{SourceID: p.ID, Runs: []RunContent{
		{SourceID: p.Runs[0].ID, Text: "Latin "},
		{SourceID: p.Runs[0].ID, Text: "日本語", Style: &InlineStyle{EastAsiaLang: "ja-JP"}},
	}})
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, s)
	runs := v.Blocks[0].Paragraph.Runs
	if len(runs) != 2 || runs[0].EastAsiaLang != "ko-KR" || runs[1].EastAsiaLang != "ja-JP" ||
		runs[0].ProofingLang != "en-US" || runs[1].ProofingLang != "en-US" || !runs[1].Italic {
		t.Fatalf("unexpected proofing metadata: %+v", runs)
	}
	root, err := parseXML(s.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	props := root.child("body").child("p").children[1].child("rPr")
	if props.child("rFonts").attr("eastAsia") != "Gulim" || props.child("lang").attr("bidi") != "ar-SA" {
		t.Fatal("unrelated font or language was changed")
	}
	p = v.Blocks[0].Paragraph
	input = paragraphPlan(v, ParagraphContent{SourceID: p.ID, Runs: []RunContent{
		{SourceID: p.Runs[0].ID, Text: "Latin ", Style: &InlineStyle{ProofingLang: "en-GB"}},
		{SourceID: p.Runs[1].ID, Text: "日本語", Style: &InlineStyle{Reset: true}},
	}})
	v, err = s.Apply(ctx, input, "human")
	if err != nil {
		t.Fatal(err)
	}
	runs = v.Blocks[0].Paragraph.Runs
	if runs[0].ProofingLang != "en-GB" || runs[0].EastAsiaLang != "ko-KR" || runs[1].EastAsiaLang != "ja-JP" || runs[1].Italic {
		t.Fatal("Latin edit or reset changed another script language")
	}
	historyRestore(t, s, "undo")
	historyRestore(t, s, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original package")
	}
}

func TestInvalidProofingLanguageRejectsWholeBatch(t *testing.T) {
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>body</w:t></w:r></w:p>`))
	v := mustSnapshot(t, s)
	p := v.Blocks[0].Paragraph
	original := fileBytes(t, filename)
	for _, lang := range []string{"en_US", "en!!", "ko-INVALIDLANGUAGEPART"} {
		input := paragraphPlan(v, ParagraphContent{SourceID: p.ID, Runs: []RunContent{
			{SourceID: p.Runs[0].ID, Text: "changed", Style: &InlineStyle{EastAsiaLang: lang}},
		}})
		if _, err := s.Apply(context.Background(), input, "human"); err == nil {
			t.Fatalf("accepted malformed language tag %q", lang)
		}
		if !bytes.Equal(original, fileBytes(t, filename)) {
			t.Fatal("invalid language changed the package")
		}
	}
}

func TestProofingLanguageRetainsOtherLanguagesFromStyle(t *testing.T) {
	ctx := context.Background()
	_, filename := withDocument(t, bodyXML(`<w:p><w:r><w:rPr><w:rStyle w:val="Language"/></w:rPr><w:t>Latin 日本語</w:t></w:r></w:p>`))
	styles := []byte(`<s:styles xmlns:s="` + wordNS + `"><s:style s:type="character" s:styleId="Language"><s:rPr><s:lang s:val="en-GB" s:eastAsia="ko-KR" s:bidi="fa-IR"/></s:rPr></s:style></s:styles>`)
	rewritePackage(t, filename, func(_ string, data []byte) []byte { return data }, map[string][]byte{
		"word/design.xml":              styles,
		"word/_rels/document.xml.rels": []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="rIdStyle" Type="` + officeRelNS + `/styles" Target="design.xml"/></Relationships>`),
	})
	s, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v := mustSnapshot(t, s)
	p := v.Blocks[0].Paragraph
	run := p.Runs[0]
	if run.ProofingLang != "en-GB" || run.EastAsiaLang != "ko-KR" {
		t.Fatal("missing inherited language metadata")
	}
	v, err = s.Apply(ctx, paragraphPlan(v, ParagraphContent{SourceID: p.ID, Runs: []RunContent{
		{SourceID: run.ID, Text: "Latin "},
		{SourceID: run.ID, Text: "日本語", Style: &InlineStyle{EastAsiaLang: "ja-JP"}},
	}}), "human")
	if err != nil {
		t.Fatal(err)
	}
	p = v.Blocks[0].Paragraph
	if p.Runs[0].EastAsiaLang != "ko-KR" || p.Runs[1].ProofingLang != "en-GB" || p.Runs[1].EastAsiaLang != "ja-JP" {
		t.Fatal("inherited language was lost on split or override")
	}
	root, err := parseXML(s.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	runs := root.child("body").child("p").children
	if runs[0].child("rPr").child("lang") != nil || runs[1].child("rPr").child("lang").attr("bidi") != "fa-IR" ||
		!bytes.Equal(styles, s.pkg.partData("word/design.xml", nil)) {
		t.Fatal("override changed styles or unselected run metadata")
	}
}
