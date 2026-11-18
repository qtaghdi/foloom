package docx

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMoveChartOnlySwapsParagraphsAndKeepsSharedParts(t *testing.T) {
	ctx := context.Background()
	above := `<w:p><w:pPr><w:spacing w:after="120"/></w:pPr><w:r><w:rPr><w:b/></w:rPr><w:t>Above</w:t></w:r></w:p>`
	below := `<w:p><w:r><w:t>Below</w:t></w:r></w:p>`
	first := `<w:p><w:pPr><w:jc w:val="center"/></w:pPr>` + chartDeleteDrawing + `<w:r><w:t/></w:r></w:p>`
	second := "<w:p>" + strings.Replace(chartDeleteDrawing, `id="15" name="First chart"`, `id="16" name="Second chart"`, 1) + "</w:p>"
	gap := "\n  "
	s, filename := chartDocument(t, above+gap+first+below+second, chartTitlePart, embeddedChartData)
	baseline, original := s.pkg, fileBytes(t, filename)
	initial := mustSnapshot(t, s)
	chart := initial.Blocks[1].Paragraph.Runs[0]
	if chart.Object == nil || !chart.Object.MoveChartUp || !chart.Object.MoveChartDown || chart.Object.Editable {
		t.Fatalf("shared chart should allow placement movement only: %+v", chart.Object)
	}

	input := ApplyInput{SessionID: initial.ID, BaseRevision: initial.Revision, Operations: []Operation{
		{Type: "move_chart_up", TargetID: chart.ID},
	}}
	after, err := s.Apply(ctx, input, "human")
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != initial.Revision+1 || after.Sequence != initial.Sequence+1 || s.events[0].TextOnly {
		t.Fatal("chart movement was not recorded as one structural operation")
	}
	moved := after.Blocks[0].Paragraph.Runs[0].Object
	if moved == nil || moved.MoveChartUp || !moved.MoveChartDown || moved.Text != "Old title" {
		t.Fatalf("unexpected post-move chart: %+v", moved)
	}
	if _, err := s.Apply(ctx, input, "ai"); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale move returned %v", err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(reopened.pkg.document) != bodyXML(first+gap+above+below+second) {
		t.Fatal("movement did not exactly swap the neighboring paragraphs")
	}
	if len(baseline.parts) != len(reopened.pkg.parts) {
		t.Fatal("movement changed the part count")
	}
	for _, part := range baseline.parts {
		if part.header.Name != "word/document.xml" && !bytes.Equal(reopened.pkg.partData(part.header.Name, nil), part.data) {
			t.Fatalf("movement changed %s", part.header.Name)
		}
	}
	reloaded := mustSnapshot(t, reopened)
	target := reloaded.Blocks[0].Paragraph.Runs[0]
	_, err = reopened.Apply(ctx, ApplyInput{SessionID: reloaded.ID, BaseRevision: reloaded.Revision, Operations: []Operation{
		{Type: "move_chart_down", TargetID: target.ID},
	}}, "ai")
	if err != nil || !bytes.Equal(reopened.pkg.document, baseline.document) {
		t.Fatalf("moving chart back did not restore original document XML: %v", err)
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore original package bytes")
	}
}

func TestMoveChartRejectsProtectedBoundariesAndInvalidFields(t *testing.T) {
	textParagraph := `<w:p><w:r><w:t>Above</w:t></w:r></w:p>`
	chartParagraph := "<w:p>" + chartDeleteDrawing + "</w:p>"
	for _, tc := range []struct {
		name, before, chart, after string
	}{
		{"first paragraph", "", chartParagraph, textParagraph},
		{"other run", textParagraph, strings.Replace(chartParagraph, "</w:p>", `<w:r><w:t>Other content</w:t></w:r></w:p>`, 1), ""},
		{"section boundary", `<w:p><w:pPr><w:sectPr/></w:pPr></w:p>`, chartParagraph, ""},
		{"page boundary", `<w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:t>New page</w:t></w:r></w:p>`, chartParagraph, ""},
		{"list paragraph", `<w:p><w:pPr><w:numPr><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>List item</w:t></w:r></w:p>`, chartParagraph, ""},
		{"caption after chart", textParagraph, chartParagraph, `<w:p><w:fldSimple w:instr=" SEQ Figure "><w:r><w:t>1</w:t></w:r></w:fldSimple></w:p>`},
		{"styled caption after chart", textParagraph, chartParagraph, `<w:p><w:pPr><w:pStyle w:val="Caption"/></w:pPr><w:r><w:t>Figure</w:t></w:r></w:p>`},
		{"moving beside a caption", `<w:p><w:pPr><w:pStyle w:val="Caption"/></w:pPr><w:r><w:t>Figure</w:t></w:r></w:p>` + textParagraph, chartParagraph, ""},
		{"covering range", `<w:p><w:bookmarkStart w:id="7" w:name="chart"/><w:r><w:t>Range start</w:t></w:r></w:p>` + textParagraph, chartParagraph, `<w:p><w:bookmarkEnd w:id="7"/></w:p>`},
		{"boundary markup", textParagraph + "<!-- keep location -->", chartParagraph, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, filename := chartDocument(t, tc.before+tc.chart+tc.after, chartTitlePart, embeddedChartData)
			original := fileBytes(t, filename)
			v := mustSnapshot(t, s)
			var chartID string
			for _, block := range v.Blocks {
				for _, run := range block.Paragraph.Runs {
					if run.Object != nil && run.Object.Kind == "chart" {
						chartID = run.ID
						if run.Object.MoveChartUp {
							t.Fatal("unexpected chart movement capability")
						}
					}
				}
			}
			if chartID == "" {
				t.Fatal("chart was missing")
			}
			_, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
				{Type: "move_chart_up", TargetID: chartID},
			}}, "human")
			if err == nil || !bytes.Equal(original, fileBytes(t, filename)) || mustSnapshot(t, s).Revision != v.Revision {
				t.Fatal("rejected movement modified the package or revision")
			}
		})
	}

	s, filename := chartDocument(t, textParagraph+chartParagraph, chartTitlePart, embeddedChartData)
	original, view := fileBytes(t, filename), mustSnapshot(t, s)
	valid := Operation{Type: "move_chart_up", TargetID: view.Blocks[1].Paragraph.Runs[0].ID}
	withText := valid
	withText.Text = "unexpected text"
	for _, invalid := range [][]Operation{{valid, valid}, {withText}, {{Type: valid.Type, TargetID: view.Blocks[0].Paragraph.ID}}} {
		_, err := s.Apply(context.Background(), ApplyInput{SessionID: view.ID, BaseRevision: view.Revision, Operations: invalid}, "human")
		if err == nil || !bytes.Equal(original, fileBytes(t, filename)) || mustSnapshot(t, s).Revision != view.Revision {
			t.Fatal("invalid movement wrote an edit")
		}
	}
}
