package docx

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
)

var literalChartPart = strings.NewReplacer(
	`<c:externalData r:id="data1"/>`, "",
	`<c:ptCount val="1"/><c:pt idx="0"><c:v>7</c:v></c:pt>`,
	`<c:ptCount val="2"/><c:pt idx="0"><c:v>7</c:v></c:pt><c:pt idx="1"><c:v>3.5</c:v></c:pt>`,
).Replace(chartTitlePart)

func TestChartValueChangesOnlySelectedLiteral(t *testing.T) {
	ctx := context.Background()
	s, filename := chartDocument(t, "<w:p>"+chartTitleDrawing+"</w:p>", literalChartPart, "")
	original := fileBytes(t, filename)
	packageBefore := s.pkg
	opened := mustSnapshot(t, s)
	run := opened.Blocks[0].Paragraph.Runs[0]
	if run.Object == nil || !slices.Equal(run.Object.ChartValues, []string{"7", "3.5"}) {
		t.Fatal("literal chart points were not projected")
	}
	point := 1
	input := ApplyInput{SessionID: opened.ID, BaseRevision: opened.Revision, Operations: []Operation{
		{Type: "update_chart_value", TargetID: run.ID, ChartValue: &ChartValueEdit{Point: &point, ExpectedValue: "3.5", Value: "4.125"}},
	}}
	result, err := s.Apply(ctx, input, "human")
	if err != nil {
		t.Fatal(err)
	}
	if result.Revision != opened.Revision+1 || result.Sequence != opened.Sequence+1 {
		t.Fatal("value edit did not commit as one revision")
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	object := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[0].Object
	if object == nil || !slices.Equal(object.ChartValues, []string{"7", "4.125"}) || object.Text != "Old title" {
		t.Fatal("reopened chart lost its data or title")
	}
	for _, part := range packageBefore.parts {
		want := part.data
		if part.header.Name == "word/charts/chart1.xml" {
			want = []byte(strings.Replace(literalChartPart, ">3.5</c:v>", ">4.125</c:v>", 1))
		}
		if !bytes.Equal(want, reopened.pkg.partData(part.header.Name, nil)) {
			t.Fatalf("chart value changed unexpected bytes in %s", part.header.Name)
		}
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore original package bytes")
	}
}

func TestChartValueProtectsLinkedSharedAndUnsupportedSeries(t *testing.T) {
	for _, tc := range []struct {
		name, body, chart, relation string
	}{
		{"embedded workbook", chartTitleDrawing, chartTitlePart, embeddedChartData},
		{"external data", chartTitleDrawing, chartTitlePart, `<Relationship Id="data1" Type="` + officeRelNS + `/package" Target="https://example.invalid/source.xlsx" TargetMode="External"/>`},
		{"shared chart", chartTitleDrawing + chartTitleDrawing, literalChartPart, ""},
		{"formula categories", chartTitleDrawing, strings.Replace(literalChartPart, "<c:val>", "<c:cat><c:strRef><c:f>Sheet!A1</c:f></c:strRef></c:cat><c:val>", 1), ""},
		{"multiple series", chartTitleDrawing, strings.Replace(literalChartPart, "</c:ser>", "</c:ser><c:ser/>", 1), ""},
		{"duplicate index", chartTitleDrawing, strings.Replace(literalChartPart, `idx="1"`, `idx="0"`, 1), ""},
		{"extension data", chartTitleDrawing, strings.Replace(literalChartPart, "</c:chartSpace>", "<c:extLst/></c:chartSpace>", 1), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, filename := chartDocument(t, "<w:p>"+tc.body+"</w:p>", tc.chart, tc.relation)
			original := fileBytes(t, filename)
			opened := mustSnapshot(t, s)
			run := opened.Blocks[0].Paragraph.Runs[0]
			if run.Object == nil || len(run.Object.ChartValues) != 0 {
				t.Fatal("protected chart presented editable values")
			}
			point := 0
			input := ApplyInput{SessionID: opened.ID, BaseRevision: opened.Revision, Operations: []Operation{
				{Type: "update_chart_value", TargetID: run.ID, ChartValue: &ChartValueEdit{Point: &point, ExpectedValue: "7", Value: "42"}},
			}}
			if _, err := s.Apply(context.Background(), input, "ai"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
				t.Fatal("protected chart value write changed original bytes")
			}
		})
	}
}

func TestChartValueRejectsStaleIndexAndInvalidText(t *testing.T) {
	s, filename := chartDocument(t, "<w:p>"+chartTitleDrawing+"</w:p>", literalChartPart, "")
	original := fileBytes(t, filename)
	opened := mustSnapshot(t, s)
	run := opened.Blocks[0].Paragraph.Runs[0]
	first, missing := 0, 2
	for _, edit := range []ChartValueEdit{
		{ExpectedValue: "7", Value: "32"},
		{Point: &first, ExpectedValue: "3.5", Value: "32"},
		{Point: &missing, ExpectedValue: "7", Value: "32"},
		{Point: &first, ExpectedValue: "7", Value: "1e99"},
		{Point: &first, ExpectedValue: "7", Value: "NaN"},
	} {
		_, err := s.Apply(context.Background(), ApplyInput{SessionID: opened.ID, BaseRevision: opened.Revision, Operations: []Operation{
			{Type: "update_chart_value", TargetID: run.ID, ChartValue: &edit},
		}}, "human")
		if err == nil || !bytes.Equal(original, fileBytes(t, filename)) || mustSnapshot(t, s).Revision != opened.Revision {
			t.Fatal("invalid point update modified the original chart")
		}
	}
}
