package docx

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
)

func TestResizeInlineChartChangesOnlyFrame(t *testing.T) {
	ctx := context.Background()
	drawing := strings.Replace(chartTitleDrawing, "<wp:inline>",
		`<wp:inline><wp:extent cx="3657600" cy="2438400"/><wp:docPr id="15" name="Licensed chart"/>`, 1)
	s, filename := chartDocument(t, "<w:p>"+drawing+"</w:p>", chartTitlePart, embeddedChartData)
	before := fileBytes(t, filename)
	original := s.pkg
	v := mustSnapshot(t, s)
	run := v.Blocks[0].Paragraph.Runs[0]
	if run.Object.ChartFrame == nil || *run.Object.ChartFrame != (ChartFrame{3657600, 2438400}) {
		t.Fatalf("missing original chart dimensions: %+v", run.Object)
	}
	width := 0
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "resize_chart", TargetID: run.ID, ChartWidthEMU: &width},
	}}
	if _, err := s.Apply(ctx, input, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("invalid chart frame changed the source")
	}
	width = 1828800
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	chart := v.Blocks[0].Paragraph.Runs[0].Object
	if chart.ChartFrame == nil || *chart.ChartFrame != (ChartFrame{1828800, 1219200}) || chart.Text != "Old title" {
		t.Fatalf("unexpected dimensions after resize: %+v", chart)
	}
	extent := regexp.MustCompile(`<wp:extent [^>]+?(?:/>|></wp:extent>)`)
	if !bytes.Equal(extent.ReplaceAll(original.document, nil), extent.ReplaceAll(reopened.pkg.document, nil)) {
		t.Fatal("resizing changed the drawing identity or content outside the chart frame")
	}
	for _, p := range original.parts {
		if p.header.Name != "word/document.xml" && !bytes.Equal(p.data, reopened.pkg.partData(p.header.Name, nil)) {
			t.Fatalf("resizing changed unrelated part %s", p.header.Name)
		}
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original package")
	}
}

func TestResizeRejectsUnsupportedFrameWithoutHidingChartTitle(t *testing.T) {
	for _, tc := range []struct {
		name, drawing string
	}{
		{"floating", strings.ReplaceAll(chartTitleDrawing, "wp:inline", "wp:anchor")},
		{"effect margin", strings.Replace(chartTitleDrawing, "<wp:inline>",
			`<wp:inline><wp:extent cx="3657600" cy="2438400"/><wp:effectExtent l="0" t="0" r="100" b="0"/>`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, filename := chartDocument(t, "<w:p>"+tc.drawing+"</w:p>", chartTitlePart, embeddedChartData)
			before := fileBytes(t, filename)
			v := mustSnapshot(t, s)
			run := v.Blocks[0].Paragraph.Runs[0]
			if run.Object == nil || !run.Object.Editable || run.Object.ChartFrame != nil {
				t.Fatal("chart title should remain visible and editable for a frame that cannot be resized")
			}
			width := 1828800
			if _, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
				{Type: "resize_chart", TargetID: run.ID, ChartWidthEMU: &width},
			}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
				t.Fatal("unsupported chart frame allowed a write")
			}
		})
	}
}
