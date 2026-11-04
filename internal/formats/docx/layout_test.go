package docx

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestPagePreviewReadsDirectFormattingAndSurvivesTextSave(t *testing.T) {
	_, path := createTest(t)
	rewritePackage(t, path, func(name string, data []byte) []byte {
		if name != "word/document.xml" {
			return data
		}
		xml := strings.Replace(string(data), "<w:p>", `<w:p><w:pPr><w:jc w:val="center"/><w:spacing w:before="0" w:after="240"/></w:pPr>`, 1)
		xml = strings.Replace(xml, "<w:r>", `<w:r><w:rPr><w:rFonts w:ascii="Arial"/><w:sz w:val="36"/><w:color w:val="194B66"/></w:rPr>`, 1)
		xml = strings.Replace(xml, `w:w="11906" w:h="16838"`, `w:w="15840" w:h="12240" w:orient="landscape"`, 1)
		xml = strings.Replace(xml, `w:left="1440"`, `w:left="1080"`, 1)
		return []byte(xml)
	}, nil)
	session, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	before := mustSnapshot(t, session)
	if before.Page == nil || before.Page.Width != 15840 || before.Page.Height != 12240 || before.Page.Left != 1080 {
		t.Fatalf("page preview = %#v", before.Page)
	}
	paragraph := before.Blocks[0].Paragraph
	run := paragraph.Runs[0]
	layout := paragraph.Layout
	if layout == nil || layout.Align != "center" || layout.Before == nil || *layout.Before != 0 || layout.After == nil || *layout.After != 240 {
		t.Fatalf("paragraph preview = %#v", layout)
	}
	if run.SizeHalfPoints != 36 || run.Font != "Arial" || run.Color != "#194B66" {
		t.Fatalf("run preview = %#v", run)
	}
	original := string(session.pkg.document)
	after, err := session.Apply(context.Background(), batch(before, "replace_run_text", run, "updated"), "human")
	if err != nil {
		t.Fatal(err)
	}
	if string(session.pkg.document) != strings.Replace(original, ">hello</w:t>", ">updated</w:t>", 1) {
		t.Fatal("text save changed unrelated OOXML")
	}
	if !reflect.DeepEqual(before.Page, after.Page) || !reflect.DeepEqual(layout, after.Blocks[0].Paragraph.Layout) {
		t.Fatal("preview layout changed after a text-only save")
	}
}

func TestPreviewResolvesRelatedStylesAndPreservesPageBreaks(t *testing.T) {
	_, path := createTest(t)
	styles := []byte(`<w:styles xmlns:w="` + wordNS + `">
		<w:docDefaults><w:rPrDefault><w:rPr><w:sz w:val="22"/></w:rPr></w:rPrDefault></w:docDefaults>
		<w:style w:type="paragraph" w:styleId="Normal" w:default="1"><w:pPr><w:spacing w:before="80" w:after="120"/></w:pPr></w:style>
		<w:style w:type="paragraph" w:styleId="Heading"><w:basedOn w:val="Normal"/><w:pPr><w:jc w:val="center"/><w:pageBreakBefore/></w:pPr><w:rPr><w:b/><w:sz w:val="48"/></w:rPr></w:style>
		<w:style w:type="character" w:styleId="Accent"><w:rPr><w:i/><w:color w:val="115E59"/></w:rPr></w:style>
	</w:styles>`)
	rewritePackage(t, path, func(name string, data []byte) []byte {
		if name != "word/document.xml" {
			return data
		}
		x := strings.Replace(string(data), "<w:p>", `<w:p><w:pPr><w:pStyle w:val="Heading"/><w:spacing w:after="0"/></w:pPr>`, 1)
		x = strings.Replace(x, "<w:r>", `<w:r><w:rPr><w:rStyle w:val="Accent"/><w:b w:val="0"/></w:rPr>`, 1)
		x = strings.Replace(x, "</w:p>", `<w:r><w:br w:type="page"/></w:r></w:p>`, 1)
		return []byte(x)
	}, map[string][]byte{
		"word/design.xml":              styles,
		"word/_rels/document.xml.rels": []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="design.xml"/></Relationships>`),
	})
	session, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	before := mustSnapshot(t, session)
	p := before.Blocks[0].Paragraph
	if !p.PageBreakBefore || !p.PageBreakAfter || len(p.Runs) != 1 || p.Layout == nil ||
		p.Layout.Align != "center" || p.Layout.Before == nil || *p.Layout.Before != 80 ||
		p.Layout.After == nil || *p.Layout.After != 0 {
		t.Fatalf("unexpected styled paragraph: %#v", p)
	}
	run := p.Runs[0]
	if run.Bold || !run.Italic || !run.Editable || run.SizeHalfPoints != 48 || run.Color != "#115E59" {
		t.Fatalf("unexpected run style: %#v", run)
	}
	if normal := before.Blocks[1].Rows[0].Cells[0].Paragraphs[0]; normal.Layout == nil || normal.Runs[0].SizeHalfPoints != 22 {
		t.Fatalf("unexpected default style: %#v", normal)
	}
	original := string(session.pkg.document)
	after, err := session.Apply(context.Background(), batch(before, "replace_run_text", run, "updated"), "human")
	if err != nil {
		t.Fatal(err)
	}
	if string(session.pkg.document) != strings.Replace(original, ">hello</w:t>", ">updated</w:t>", 1) ||
		!reflect.DeepEqual(p.Layout, after.Blocks[0].Paragraph.Layout) || !after.Blocks[0].Paragraph.PageBreakAfter {
		t.Fatal("saving text changed the surrounding layout")
	}
}
