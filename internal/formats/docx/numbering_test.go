package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNewListsShareBatchWithStylesAndPreserveUndo(t *testing.T) {
	s, path := withDocument(t, bodyXML(`<w:p><w:r><w:t>first</w:t></w:r></w:p><w:p><w:r><w:t>second</w:t></w:r></w:p>`))
	original := fileBytes(t, path)
	v := mustSnapshot(t, s)
	contents := []ParagraphContent{}
	for i, b := range v.Blocks {
		p := b.Paragraph
		contents = append(contents, ParagraphContent{SourceID: p.ID, Format: &ParagraphFormat{List: &ParagraphList{Key: "newList", Level: i}}, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: p.Runs[0].Text}}})
	}
	input := paragraphPlan(v, contents...)
	input.Operations = append([]Operation{
		{Type: "define_list", ListDefinition: &ListDefinition{Key: "newList", Kind: "decimal", Start: 3}},
		{Type: "define_style", StyleDefinition: &StyleDefinition{ID: "NewStyle", Name: "새 스타일", Kind: "paragraph"}},
	}, input.Operations...)
	v, err := s.Apply(context.Background(), input, "human")
	if err != nil {
		t.Fatal(err)
	}
	if v.Revision != 2 || len(v.Lists) != 1 || v.Blocks[0].Paragraph.ListLabel != "3." || v.Blocks[1].Paragraph.ListLabel != "1." {
		t.Fatalf("incorrect list preview: %+v", v)
	}
	if v.Blocks[0].Paragraph.List.ID != v.Blocks[1].Paragraph.List.ID {
		t.Fatal("list instance not shared")
	}
	if s.pkg.numberingPath == "" || s.pkg.stylesPath == "" {
		t.Fatal("missing part relationship")
	}
	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if mustSnapshot(t, reopened).Blocks[0].Paragraph.ListLabel != "3." {
		t.Fatal("numbering lost after reopen")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("undo changed original package")
	}
}

func TestInvalidListBatchDoesNotWriteDefinitionsOrText(t *testing.T) {
	for _, list := range []ParagraphList{{Key: "unknown"}, {Key: "new", Level: 9}, {ID: 200}, {Key: "new", ID: 1}} {
		s, path := withDocument(t, bodyXML(`<w:p><w:r><w:t>before</w:t></w:r></w:p>`))
		v := mustSnapshot(t, s)
		p := v.Blocks[0].Paragraph
		before := fileBytes(t, path)
		input := paragraphPlan(v, ParagraphContent{SourceID: p.ID, Format: &ParagraphFormat{List: &list}, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "after"}}})
		input.Operations = append(input.Operations, Operation{Type: "define_list", ListDefinition: &ListDefinition{Key: "new", Kind: "bullet"}})
		if _, err := s.Apply(context.Background(), input, "human"); err == nil {
			t.Fatal("invalid list accepted", list)
		}
		if !bytes.Equal(before, fileBytes(t, path)) {
			t.Fatal("rejected list changed package")
		}
	}
}

func TestNewListDoesNotReuseDanglingReferences(t *testing.T) {
	s, _ := withDocument(t, bodyXML(`<w:p><w:pPr><w:numPr><w:numId w:val="99"/></w:numPr></w:pPr><w:r><w:t>x</w:t></w:r></w:p>`))
	v := mustSnapshot(t, s)
	v, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "define_list", ListDefinition: &ListDefinition{Key: "new", Kind: "decimal"}}}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Lists) != 1 || v.Lists[0].ID <= 99 || v.Blocks[0].Paragraph.ListLabel != "" {
		t.Fatal("dangling reference unexpectedly became numbered")
	}
	if !strings.Contains(string(s.pkg.document), `w:val="99"`) {
		t.Fatal("existing reference changed")
	}
}

func TestListsPreserveCustomNumberingAndOtherParts(t *testing.T) {
	_, path := withDocument(t, bodyXML(`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="7"/></w:numPr></w:pPr><w:r><w:t>existing</w:t></w:r></w:p>`))
	numbering := []byte(`<q:numbering xmlns:q="` + wordNS + `"><q:abstractNum q:abstractNumId="5"><q:nsid q:val="AABBCCDD"/><q:lvl q:ilvl="0"><q:start q:val="1"/><q:numFmt q:val="bullet"/><q:lvlText q:val="★"/></q:lvl></q:abstractNum><q:num q:numId="7"><q:abstractNumId q:val="5"/></q:num><q:numIdMacAtCleanup q:val="8"/></q:numbering>`)
	rels := []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="numbering" Type="` + numberingRelationship + `" Target="custom-lists.xml"/></Relationships>`)
	rewritePackage(t, path, func(_ string, b []byte) []byte { return b }, map[string][]byte{"word/custom-lists.xml": numbering, "word/_rels/document.xml.rels": rels, "customXml/keep.xml": []byte("<keep/>")})
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	before := s.pkg
	v := mustSnapshot(t, s)
	if v.Blocks[0].Paragraph.ListLabel != "★" {
		t.Fatal("custom bullet not displayed")
	}
	_, err = s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "define_list", ListDefinition: &ListDefinition{Key: "new", Kind: "decimal", Start: 4}}}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	if s.pkg.numberingPath != "word/custom-lists.xml" {
		t.Fatal("numbering part mapping lost")
	}
	raw := s.pkg.partData(s.pkg.numberingPath, nil)
	for _, name := range []string{"abstractNum", "num", "numIdMacAtCleanup"} {
		oldRoot, _ := parseXML(numbering)
		newRoot, _ := parseXML(raw)
		old, next := oldRoot.child(name), newRoot.child(name)
		if !bytes.Equal(numbering[old.start:old.end], raw[next.start:next.end]) {
			t.Fatal("unrelated numbering changed", name)
		}
	}
	for i, p := range before.parts {
		if p.header.Name != s.pkg.numberingPath && !bytes.Equal(p.data, s.pkg.parts[i].data) {
			t.Fatal("unrelated part changed", p.header.Name)
		}
	}
}

func TestListRemovalOverridesStyleWithoutDiscardingRunProperties(t *testing.T) {
	s, _ := withDocument(t, bodyXML(`<w:p><w:pPr><w:numPr xmlns:k="urn:custom" k:keep="yes"><w:ilvl w:val="2"/><w:numId w:val="7"/></w:numPr><w:spacing w:after="100"/></w:pPr><w:r><w:rPr><w:b/></w:rPr><w:t>x</w:t></w:r></w:p>`))
	v := mustSnapshot(t, s)
	p := v.Blocks[0].Paragraph
	_, err := s.Apply(context.Background(), paragraphPlan(v, ParagraphContent{SourceID: p.ID, Format: &ParagraphFormat{List: &ParagraphList{}}, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "x"}}}), "human")
	if err != nil {
		t.Fatal(err)
	}
	root, _ := parseXML(s.pkg.document)
	props := root.child("body").child("p").child("pPr")
	if props.child("numPr").child("numId").attr("val") != "0" || props.child("numPr").child("ilvl") != nil || props.child("spacing").attr("after") != "100" || !strings.Contains(string(s.pkg.document), `k:keep="yes"`) {
		t.Fatal("list removal lost unrelated properties")
	}
}
