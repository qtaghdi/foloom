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
