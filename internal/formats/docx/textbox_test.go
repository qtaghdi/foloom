package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func mirroredBoxXML(modern, legacy string) string {
	return `<w:r><mc:AlternateContent xmlns:mc="` + markupCompatNS + `" xmlns:wps="` + shapeWordNS + `">` +
		`<mc:Choice Requires="wps"><w:drawing xmlns:wp="` + drawingWordNS + `" xmlns:a="` + drawingNS + `">` +
		`<wp:inline><wp:extent cx="1000000" cy="400000"/><a:graphic><a:graphicData uri="` + shapeWordNS + `">` +
		`<wps:wsp><wps:txbx><w:txbxContent><w:p><w:r><w:rPr><w:b/></w:rPr><w:t>` + modern +
		`</w:t></w:r></w:p></w:txbxContent></wps:txbx></wps:wsp></a:graphicData></a:graphic></wp:inline></w:drawing></mc:Choice>` +
		`<mc:Fallback><w:pict xmlns:v="` + visualMarkupNS + `"><v:rect style="width:80pt;height:40pt"><v:textbox>` +
		`<w:txbxContent><w:p><w:r><w:rPr><w:i/></w:rPr><w:t>` + legacy +
		`</w:t></w:r></w:p></w:txbxContent></v:textbox></v:rect></w:pict></mc:Fallback></mc:AlternateContent></w:r>`
}

func TestTextBoxUpdatesBothStoriesWithoutChangingLayout(t *testing.T) {
	ctx := context.Background()
	input := bodyXML(`<w:p><w:r><w:t>prefix</w:t></w:r>` + mirroredBoxXML("원본", "원본") + `</w:p>`)
	s, filename := withDocument(t, input)
	original := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	run := v.Blocks[0].Paragraph.Runs[1]
	if run.Object == nil || run.Object.Kind != "textbox" || !run.Object.Editable || run.Object.Text != "원본" {
		t.Fatalf("mirrored text box was not recognized: %+v", run)
	}
	op := Operation{Type: "update_textbox_text", TargetID: run.ID, ExpectedText: "stale", Text: "새 값 & 문구"}
	batch := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}
	if _, err := s.Apply(ctx, batch, "human"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("stale known text box was changed")
	}
	batch.Operations[0].ExpectedText = "원본"
	if _, err := s.Apply(ctx, batch, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	saved := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[1]
	doc := string(reopened.pkg.document)
	if saved.Object == nil || saved.Object.Text != op.Text || strings.Count(doc, "새 값 &amp; 문구") != 2 ||
		!strings.Contains(doc, `<wp:extent cx="1000000" cy="400000"/>`) ||
		!strings.Contains(doc, `style="width:80pt;height:40pt"`) ||
		!strings.Contains(doc, `<w:rPr><w:b/></w:rPr>`) || !strings.Contains(doc, `<w:rPr><w:i/></w:rPr>`) {
		t.Fatal("stored text box lost text in one story, layout or formatting")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original text box package")
	}
}

func TestTextBoxProtectsImportWhenCompatibilityStoriesDiffer(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p>`+mirroredBoxXML("modern", "fallback")+`</w:p>`))
	before := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	box := v.Blocks[0].Paragraph.Runs[0]
	if box.Object == nil || box.Object.Editable || box.Object.Text != "modern" {
		t.Fatal("mismatched text boxes should remain readable")
	}
	if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_textbox_text", TargetID: box.ID, ExpectedText: "modern", Text: "rewritten"},
	}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("mismatched text box stories were overwritten")
	}
}
