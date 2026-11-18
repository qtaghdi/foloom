package docx

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
)

func TestInsertInlineRectangleKeepsOtherDrawingAndUndoes(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>상자</w:t></w:r></w:p><w:p><w:r><w:t>도형</w:t></w:r></w:p>`))
	before := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "insert_textbox", TargetID: v.Blocks[0].ID, Text: "unchanged box"},
		{Type: "insert_rectangle", TargetID: v.Blocks[1].ID, Text: "unexpected shape text"},
	}}
	if _, err := s.Apply(ctx, input, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("invalid rectangle did not reject the complete batch")
	}
	input.Operations[1].Text = ""
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	box, shape := v.Blocks[0].Paragraph.Runs[1].Object, v.Blocks[1].Paragraph.Runs[1].Object
	if box == nil || box.Kind != "textbox" || box.Text != "unchanged box" ||
		shape == nil || shape.Kind != "shape" || shape.Rectangle == nil ||
		*shape.Rectangle != (RectanglePreview{WidthEMU: 1143000, HeightEMU: 571500, Fill: "D7EAE4", Stroke: "0F766E"}) {
		t.Fatalf("reopened box = %+v, rectangle = %+v", box, shape)
	}
	raw := string(reopened.pkg.document)
	if !strings.Contains(raw, `<wp:docPr id="1" name="Text Box 1"`) ||
		!strings.Contains(raw, `<wp:docPr id="2" name="Rectangle 2"`) ||
		!strings.Contains(raw, `<a:prstGeom prst="rect"><a:avLst/>`) {
		t.Fatal("native rectangle or unique drawing IDs missing")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original package")
	}
}

func TestRectangleColorsPreserveGeometryAndOriginalNeighbors(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>before</w:t></w:r><!--keep this-->`+
		inlineRectangle(5)+`<w:r><w:t>after</w:t></w:r></w:p><w:p><w:ins w:id="6" w:author="Alice">`+
		inlineRectangle(7)+`</w:ins></w:p>`))
	before := fileBytes(t, filename)
	original := bytes.Clone(s.pkg.document)
	v := mustSnapshot(t, s)
	rectangle := v.Blocks[0].Paragraph.Runs[1]
	if rectangle.Object == nil || !rectangle.Object.Editable {
		t.Fatal("plain inline rectangle must expose its own editable target")
	}
	edit := Operation{Type: "update_rectangle_colors", TargetID: rectangle.ID,
		RectangleColors: &RectangleColorEdit{Fill: "a12B3C", Stroke: "invalid"}}
	if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{edit}}, "human"); err == nil ||
		!bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("invalid outline changed the source")
	}
	edit.RectangleColors.Stroke = "0F766E"
	if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{edit}}, "human"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(s.pkg.document), `<a:srgbClr val="0F766E"/>`) {
		t.Fatal("leaving the outline unchanged must preserve its bytes")
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	rectangle = v.Blocks[0].Paragraph.Runs[1]
	edit.TargetID = rectangle.ID
	edit.RectangleColors.Stroke = "1f4f32"
	if _, err := reopened.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{edit}}, "human"); err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	want := RectanglePreview{WidthEMU: 1143000, HeightEMU: 571500, Fill: "A12B3C", Stroke: "1F4F32"}
	if got := v.Blocks[0].Paragraph.Runs[1].Object.Rectangle; got == nil || *got != want {
		t.Fatalf("updated rectangle = %+v, want %+v", got, want)
	}
	colors := regexp.MustCompile(`<a:srgbClr [^>]+?(?:/>|></a:srgbClr>)`)
	if !bytes.Equal(colors.ReplaceAll(original, []byte("color")), colors.ReplaceAll(reopened.pkg.document, []byte("color"))) {
		t.Fatal("changing colors also changed geometry, ids, or neighboring XML")
	}
	if !strings.Contains(string(reopened.pkg.document), string(original[bytes.Index(original, []byte("<w:ins")):])) {
		t.Fatal("recoloring touched a separate, tracked rectangle")
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original package")
	}
}
