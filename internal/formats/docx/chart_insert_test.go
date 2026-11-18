package docx

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
)

func TestInsertPieChartsAllocatePartsAndStayEditable(t *testing.T) {
	ctx := context.Background()
	body := `<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>Keep the text</w:t></w:r></w:p><w:p/><w:p>` + chartDeleteDrawing + `</w:p>`
	session, filename := chartDocument(t, body, chartTitlePart, embeddedChartData)
	original := fileBytes(t, filename)
	baseline := session.pkg
	initial := mustSnapshot(t, session)
	input := ChartInsert{Title: "분기별 결과 & 확인", Points: []ChartInputPoint{{Label: "Apple <local>", Value: "2.5"}, {Label: "배", Value: "5"}}}
	second := ChartInsert{Title: "Next chart", Points: []ChartInputPoint{{Label: "One", Value: "10"}}}
	written, err := session.Apply(ctx, ApplyInput{SessionID: initial.ID, BaseRevision: initial.Revision, Operations: []Operation{
		{Type: "insert_chart", TargetID: initial.Blocks[0].ID, ChartInsert: &input},
		{Type: "insert_chart", TargetID: initial.Blocks[1].ID, ChartInsert: &second},
	}}, "ai")
	if err != nil {
		t.Fatal(err)
	}
	if written.Revision != initial.Revision+1 || written.Sequence != initial.Sequence+1 ||
		len(session.events) != 1 || session.events[0].TextOnly {
		t.Fatal("chart batch did not commit as one structural revision")
	}
	afterInsert := fileBytes(t, filename)
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	state := mustSnapshot(t, reopened)
	first, other := state.Blocks[0].Paragraph.Runs[1], state.Blocks[1].Paragraph.Runs[0]
	if first.Object == nil || first.Object.Text != input.Title || !first.Object.Editable || !first.Object.DeleteEditable ||
		first.Object.ChartFrame == nil || !slices.Equal(first.Object.ChartValues, []string{"2.5", "5"}) ||
		other.Object == nil || other.Object.Text != second.Title || !slices.Equal(other.Object.ChartValues, []string{"10"}) {
		t.Fatal("new charts did not reopen with editable titles, values and frames")
	}
	for _, file := range []string{"word/charts/foloom-chart.xml", "word/charts/foloom-chart-1.xml"} {
		raw := reopened.pkg.partData(file, nil)
		root, err := parseXML(raw)
		if err != nil || hasChartDataSource(root) || reopened.pkg.chartContentTypes[file] != 1 || reopened.pkg.chartReferences[file] != 1 {
			t.Fatalf("invalid self-contained chart part %s: %v", file, err)
		}
	}
	if !bytes.Contains(reopened.pkg.document, []byte(`<w:rPr><w:b/></w:rPr><w:t>Keep the text</w:t>`)) ||
		!bytes.Contains(reopened.pkg.document, []byte(`<wp:docPr id="16" name="Chart 16"/>`)) ||
		!bytes.Contains(reopened.pkg.document, []byte(`<wp:docPr id="17" name="Chart 17"/>`)) ||
		!bytes.Contains(reopened.pkg.partData("word/charts/foloom-chart.xml", nil), []byte(`Apple &lt;local&gt;`)) {
		t.Fatal("chart insertion lost source text, unique drawing IDs or escaped categories")
	}
	for _, part := range baseline.parts {
		switch part.header.Name {
		case "word/document.xml", documentRelationships, "[Content_Types].xml":
			continue
		}
		if !bytes.Equal(part.data, reopened.pkg.partData(part.header.Name, nil)) {
			t.Fatalf("chart insertion changed unrelated part %s", part.header.Name)
		}
	}
	point := 1
	edited, err := reopened.Apply(ctx, ApplyInput{SessionID: state.ID, BaseRevision: state.Revision, Operations: []Operation{
		{Type: "update_chart_value", TargetID: first.ID, ChartValue: &ChartValueEdit{Point: &point, ExpectedValue: "5", Value: "12.5"}},
	}}, "human")
	if err != nil || edited.Blocks[0].Paragraph.Runs[1].Object.ChartValues[1] != "12.5" {
		t.Fatalf("could not edit newly inserted native chart: %v", err)
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(afterInsert, fileBytes(t, filename)) {
		t.Fatal("undo did not restore chart data")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not remove chart parts and restore exact original package")
	}
}

func TestInsertPieChartInvalidInputRejectsEntireBatch(t *testing.T) {
	valid := ChartInsert{Title: "Valid chart", Points: []ChartInputPoint{{Label: "Apple", Value: "5"}}}
	for _, tc := range []struct {
		name  string
		input ChartInsert
		block int
	}{
		{"missing title", ChartInsert{Points: valid.Points}, 1},
		{"missing points", ChartInsert{Title: valid.Title}, 1},
		{"negative value", ChartInsert{Title: valid.Title, Points: []ChartInputPoint{{Label: "Apple", Value: "-1"}, {Label: "Pear", Value: "2"}}}, 1},
		{"zero only", ChartInsert{Title: valid.Title, Points: []ChartInputPoint{{Label: "Apple", Value: "0"}}}, 1},
		{"over limit", ChartInsert{Title: valid.Title, Points: []ChartInputPoint{{Label: "Apple", Value: "1e16"}}}, 1},
		{"blank category", ChartInsert{Title: valid.Title, Points: []ChartInputPoint{{Label: " ", Value: "5"}}}, 1},
		{"long category", ChartInsert{Title: valid.Title, Points: []ChartInputPoint{{Label: strings.Repeat("배", 41), Value: "5"}}}, 1},
		{"protected target", valid, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session, filename := withDocument(t, bodyXML(`<w:p/><w:p/><w:p><w:sdt><w:sdtContent><w:r><w:t>Protected</w:t></w:r></w:sdtContent></w:sdt></w:p>`))
			original := fileBytes(t, filename)
			state := mustSnapshot(t, session)
			_, err := session.Apply(context.Background(), ApplyInput{SessionID: state.ID, BaseRevision: state.Revision, Operations: []Operation{
				{Type: "insert_chart", TargetID: state.Blocks[0].ID, ChartInsert: &valid},
				{Type: "insert_chart", TargetID: state.Blocks[tc.block].ID, ChartInsert: &tc.input},
			}}, "human")
			if err == nil || !bytes.Equal(original, fileBytes(t, filename)) || mustSnapshot(t, session).Revision != state.Revision {
				t.Fatal("invalid chart insertion modified the original archive")
			}
		})
	}
}
