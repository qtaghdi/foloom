package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

const chartTitleDrawing = `<w:r xmlns:wp="` + drawingWordNS + `" xmlns:a="` + drawingNS + `" xmlns:c="` + chartNS + `" xmlns:r="` + officeRelNS + `"><w:drawing><wp:inline><a:graphic><a:graphicData uri="` + chartNS + `"><c:chart r:id="chart1"/></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`

const chartTitlePart = `<c:chartSpace xmlns:c="` + chartNS + `" xmlns:a="` + drawingNS + `" xmlns:r="` + officeRelNS + `">
<c:chart><c:title><c:tx><c:rich><a:bodyPr/><a:p><a:r><a:rPr b="1"/><a:t>Old title</a:t></a:r></a:p></c:rich></c:tx></c:title>
<c:plotArea><c:pieChart><c:ser><c:val><c:numLit><c:ptCount val="1"/><c:pt idx="0"><c:v>7</c:v></c:pt></c:numLit></c:val></c:ser></c:pieChart></c:plotArea></c:chart>
<c:externalData r:id="data1"/></c:chartSpace>`

func chartDocument(t *testing.T, body, chart, dataRelation string) (*Session, string) {
	t.Helper()
	_, filename := createTest(t)
	rewritePackage(t, filename, func(name string, data []byte) []byte {
		switch name {
		case "word/document.xml":
			return []byte(bodyXML(body))
		case "[Content_Types].xml":
			return []byte(strings.Replace(string(data), "</Types>", `<Override PartName="/word/charts/chart1.xml" ContentType="`+chartContentType+`"/></Types>`, 1))
		default:
			return data
		}
	}, map[string][]byte{
		documentRelationships:               []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="chart1" Type="` + officeRelNS + `/chart" Target="charts/chart1.xml"/></Relationships>`),
		"word/charts/chart1.xml":            []byte(chart),
		"word/charts/_rels/chart1.xml.rels": []byte(`<Relationships xmlns="` + packageRelNS + `">` + dataRelation + `</Relationships>`),
		"word/embeddings/source.xlsx":       []byte("opaque workbook"),
	})
	session, err := Open(context.Background(), filename)
	if err != nil {
		t.Fatal(err)
	}
	return session, filename
}

const embeddedChartData = `<Relationship Id="data1" Type="` + officeRelNS + `/package" Target="../embeddings/source.xlsx"/>`

func TestChartTitleEditsExactlyOneLocalPart(t *testing.T) {
	ctx := context.Background()
	s, filename := chartDocument(t, "<w:p>"+chartTitleDrawing+"</w:p>", chartTitlePart, embeddedChartData)
	before := fileBytes(t, filename)
	oldPackage := s.pkg
	v := mustSnapshot(t, s)
	run := v.Blocks[0].Paragraph.Runs[0]
	if run.Object == nil || !run.Object.Editable || run.Object.Text != "Old title" {
		t.Fatalf("chart title did not project: %+v", run.Object)
	}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_chart_title", TargetID: run.ID, ExpectedText: "stale title", Text: "분기별 실적 & 성장"},
	}}
	if _, err := s.Apply(ctx, input, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("stale chart title changed the source")
	}
	input.Operations[0].ExpectedText = "Old title"
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	if got := v.Blocks[0].Paragraph.Runs[0].Object.Text; got != "분기별 실적 & 성장" {
		t.Fatalf("chart title after reopen: %q", got)
	}
	for _, p := range oldPackage.parts {
		want := p.data
		if p.header.Name == "word/charts/chart1.xml" {
			want = []byte(strings.Replace(chartTitlePart, "Old title", "분기별 실적 &amp; 성장", 1))
		}
		if !bytes.Equal(want, reopened.pkg.partData(p.header.Name, nil)) {
			t.Fatalf("unexpected bytes in %s after title edit", p.header.Name)
		}
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original chart package")
	}
}

func TestChartTitleProtectsExternalSharedAndComplexCharts(t *testing.T) {
	for _, tc := range []struct {
		name, body, chart, relation string
	}{
		{"same relationship used twice", chartTitleDrawing + chartTitleDrawing, chartTitlePart, embeddedChartData},
		{"external data", chartTitleDrawing, chartTitlePart, `<Relationship Id="data1" Type="` + officeRelNS + `/package" Target="https://example.invalid/workbook.xlsx" TargetMode="External"/>`},
		{"compound title", chartTitleDrawing, strings.Replace(chartTitlePart, "</a:r></a:p>", "</a:r><a:r><a:t>Second</a:t></a:r></a:p>", 1), embeddedChartData},
		{"3D chart", chartTitleDrawing, strings.ReplaceAll(chartTitlePart, "pieChart", "pie3DChart"), embeddedChartData},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, filename := chartDocument(t, "<w:p>"+tc.body+"</w:p>", tc.chart, tc.relation)
			before := fileBytes(t, filename)
			v := mustSnapshot(t, s)
			run := v.Blocks[0].Paragraph.Runs[0]
			if run.Object == nil || run.Object.Editable {
				t.Fatal("chart should stay read-only")
			}
			_, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
				{Type: "update_chart_title", TargetID: run.ID, ExpectedText: "Old title", Text: "new title"},
			}}, "human")
			if err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
				t.Fatal("protected chart allowed a write")
			}
		})
	}
}
