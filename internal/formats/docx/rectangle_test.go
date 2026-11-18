package docx

import (
	"bytes"
	"context"
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
