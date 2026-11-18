package docx

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

var chartDeleteDrawing = strings.Replace(chartTitleDrawing, "<wp:inline>",
	`<wp:inline><wp:extent cx="3657600" cy="2438400"/><wp:docPr id="15" name="First chart"/>`, 1)

func TestDeleteInlineChartPreservesSharedDrawingAndAllParts(t *testing.T) {
	ctx := context.Background()
	first := strings.Replace(chartDeleteDrawing, "><w:drawing>", "><w:rPr><w:noProof/></w:rPr><w:drawing>", 1)
	second := strings.Replace(chartDeleteDrawing, `id="15" name="First chart"`, `id="16" name="Second chart"`, 1)
	prefix := `<w:r><w:t xml:space="preserve">Before chart </w:t></w:r>`
	suffix := `<w:r><w:t xml:space="preserve"> after chart</w:t></w:r>`
	body := "<w:p>" + prefix + first + suffix + "</w:p><w:p>" + second + "</w:p>"
	s, filename := chartDocument(t, body, chartTitlePart, embeddedChartData)
	original := fileBytes(t, filename)
	baseline := s.pkg
	initial := mustSnapshot(t, s)
	chart := initial.Blocks[0].Paragraph.Runs[1]
	if chart.Object == nil || !chart.Object.DeleteEditable || chart.Object.Editable || chart.Object.ChartFrame != nil {
		t.Fatal("shared chart must allow removal without advertising chart-part edits")
	}
	input := ApplyInput{SessionID: initial.ID, BaseRevision: initial.Revision, Operations: []Operation{
		{Type: "delete_chart", TargetID: chart.ID},
	}}
	after, err := s.Apply(ctx, input, "human")
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != initial.Revision+1 || after.Sequence != initial.Sequence+1 || s.events[0].TextOnly {
		t.Fatal("chart deletion was not saved as one structural operation")
	}
	if _, err := s.Apply(ctx, input, "ai"); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale deletion returned %v", err)
	}
	firstDeletion := fileBytes(t, filename)
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	checkOnlyDocument := func(want string) {
		t.Helper()
		if !bytes.Equal(reopened.pkg.document, []byte(want)) {
			t.Fatal("chart deletion changed other document XML")
		}
		for _, part := range baseline.parts {
			if part.header.Name != "word/document.xml" && !bytes.Equal(reopened.pkg.partData(part.header.Name, nil), part.data) {
				t.Fatalf("chart deletion changed %s", part.header.Name)
			}
		}
		if len(baseline.parts) != len(reopened.pkg.parts) {
			t.Fatal("chart deletion changed the part count")
		}
	}
	checkOnlyDocument(bodyXML(strings.Replace(body, first, "", 1)))
	snapshot := mustSnapshot(t, reopened)
	remaining := snapshot.Blocks[1].Paragraph.Runs[0]
	if remaining.Object == nil || !remaining.Object.DeleteEditable || !remaining.Object.Editable || remaining.Object.Text != "Old title" {
		t.Fatal("the surviving chart reference lost its title or editability")
	}
	_, err = reopened.Apply(ctx, ApplyInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision, Operations: []Operation{
		{Type: "delete_chart", TargetID: remaining.ID},
	}}, "ai")
	if err != nil {
		t.Fatal(err)
	}
	checkOnlyDocument(bodyXML("<w:p>" + prefix + suffix + "</w:p><w:p></w:p>"))
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(firstDeletion, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the final chart placement")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore exact original file")
	}
}

func TestDeleteInlineChartRejectsUnsupportedPlacements(t *testing.T) {
	for _, tc := range []struct {
		name, body, chart, relation string
	}{
		{"floating", strings.ReplaceAll(chartDeleteDrawing, "wp:inline", "wp:anchor"), chartTitlePart, embeddedChartData},
		{"mixed text", strings.Replace(chartDeleteDrawing, "</w:r>", "<w:t>Keep</w:t></w:r>", 1), chartTitlePart, embeddedChartData},
		{"bookmark", `<w:bookmarkStart w:id="6" w:name="protected"/>` + chartDeleteDrawing + `<w:bookmarkEnd w:id="6"/>`, chartTitlePart, embeddedChartData},
		{"tracked paragraph", chartDeleteDrawing + `<w:ins w:id="6"><w:r><w:t>Protected</w:t></w:r></w:ins>`, chartTitlePart, embeddedChartData},
		{"extension child", strings.Replace(chartDeleteDrawing, "</wp:inline>", "<wp:unknown/></wp:inline>", 1), chartTitlePart, embeddedChartData},
		{"external workbook", chartDeleteDrawing, chartTitlePart, `<Relationship Id="data1" Type="` + officeRelNS + `/package" Target="https://example.invalid/workbook.xlsx" TargetMode="External"/>`},
		{"unsupported chart", chartDeleteDrawing, strings.ReplaceAll(chartTitlePart, "pieChart", "pie3DChart"), embeddedChartData},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, filename := chartDocument(t, "<w:p>"+tc.body+"</w:p>", tc.chart, tc.relation)
			original := fileBytes(t, filename)
			v := mustSnapshot(t, s)
			chartID := ""
			for _, run := range v.Blocks[0].Paragraph.Runs {
				if run.Object != nil && run.Object.Kind == "chart" {
					if run.Object.DeleteEditable {
						t.Fatal("protected chart was offered for deletion")
					}
					chartID = run.ID
				}
			}
			if chartID == "" {
				t.Fatal("protected chart was not present in the snapshot")
			}
			_, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
				{Type: "delete_chart", TargetID: chartID},
			}}, "human")
			if err == nil || !bytes.Equal(original, fileBytes(t, filename)) || mustSnapshot(t, s).Revision != v.Revision {
				t.Fatal("protected chart deletion modified the source")
			}
		})
	}
}
