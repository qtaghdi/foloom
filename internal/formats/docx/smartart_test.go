package docx

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
)

const smartArtTestDrawing = `<w:r xmlns:wp="` + drawingWordNS + `" xmlns:a="` + drawingNS +
	`" xmlns:dgm="` + diagramNS + `" xmlns:r="` + officeRelNS + `"><w:drawing><wp:inline><a:graphic><a:graphicData uri="` + diagramNS +
	`"><dgm:relIds r:dm="data" r:lo="layout"/></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`

const smartArtDataStart = `<dgm:dataModel xmlns:dgm="` + diagramNS + `" xmlns:a="` + drawingNS + `"><dgm:ptLst>`

func TestSmartArtReadsSemanticLabelsAndPreservesParts(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"local", "external"} {
		t.Run(mode, func(t *testing.T) {
			_, filename := createTest(t)
			target := `Target="diagrams/data1.xml"`
			if mode == "external" {
				target = `Target="https://example.invalid/data1.xml" TargetMode="External"`
			}
			data := smartArtDataStart + `
<dgm:pt modelId="1"><dgm:prSet phldrT="[Text]"/><dgm:t><a:p><a:r><a:t>첫</a:t></a:r><a:br/><a:r><a:t>항목</a:t></a:r></a:p></dgm:t></dgm:pt>
<dgm:pt modelId="2" type="pres"><dgm:t><a:p><a:r><a:t>Duplicate display label</a:t></a:r></a:p></dgm:t></dgm:pt>
<dgm:pt modelId="3" type="node"><dgm:prSet phldr="1"/><dgm:t><a:p><a:r><a:t>Placeholder</a:t></a:r></a:p></dgm:t></dgm:pt>
<dgm:pt modelId="4" type="asst"><dgm:t><a:p><a:r><a:t>Assistant</a:t></a:r></a:p></dgm:t></dgm:pt>
</dgm:ptLst></dgm:dataModel>`
			rewritePackage(t, filename, func(name string, content []byte) []byte {
				switch name {
				case "word/document.xml":
					return []byte(bodyXML(`<w:p><w:r><w:t>before</w:t></w:r></w:p><w:p>` + smartArtTestDrawing + `</w:p>`))
				case "[Content_Types].xml":
					return []byte(strings.Replace(string(content), "</Types>", `<Override PartName="/word/diagrams/data1.xml" ContentType="`+diagramDataContentType+`"/></Types>`, 1))
				default:
					return content
				}
			}, map[string][]byte{
				documentRelationships: []byte(`<Relationships xmlns="` + packageRelNS +
					`"><Relationship Id="data" Type="` + officeRelNS + `/diagramData" ` + target +
					`/><Relationship Id="layout" Type="` + officeRelNS + `/diagramLayout" Target="diagrams/layout1.xml"/></Relationships>`),
				"word/diagrams/data1.xml":   []byte(data),
				"word/diagrams/layout1.xml": []byte(`<otherLayout>Opaque layout data</otherLayout>`),
			})
			session, err := Open(ctx, filename)
			if err != nil {
				t.Fatal(err)
			}
			original := session.pkg
			v := mustSnapshot(t, session)
			diagram := v.Blocks[1].Paragraph.Runs[0].Object
			if diagram == nil || diagram.Kind != "smartart" || diagram.Editable {
				t.Fatal("SmartArt is a read-only preview")
			}
			if mode == "external" {
				if diagram.SmartArt != nil {
					t.Fatal("an external relationship must not project a package part")
				}
				return
			}
			if diagram.SmartArt == nil || !reflect.DeepEqual(diagram.SmartArt.Items, []string{"첫 항목", "Assistant"}) || diagram.SmartArt.Truncated {
				t.Fatalf("unexpected semantic labels: %+v", diagram.SmartArt)
			}
			first := v.Blocks[0].Paragraph.Runs[0]
			if _, err := session.Apply(ctx, batch(v, "replace_run_text", first, "after"), "human"); err != nil {
				t.Fatal(err)
			}
			for _, part := range original.parts {
				if part.header.Name != "word/document.xml" &&
					!bytes.Equal(part.data, session.pkg.partData(part.header.Name, nil)) {
					t.Fatalf("editing body changed %s", part.header.Name)
				}
			}
		})
	}
}

func TestSmartArtPreviewLimitsTextAndPointCount(t *testing.T) {
	point := `<dgm:pt><dgm:t><a:p><a:r><a:t>` + strings.Repeat("가", 100) + `</a:t></a:r></a:p></dgm:t></dgm:pt>`
	result := previewSmartArt([]byte(smartArtDataStart + strings.Repeat(point, maxSmartArtItems+1) + "</dgm:ptLst></dgm:dataModel>"))
	if result == nil || !result.Truncated || len(result.Items) != maxSmartArtItems ||
		result.Items[0] != strings.Repeat("가", 85)+"…" {
		t.Fatalf("preview did not truncate whole Korean characters and excess labels: %+v", result)
	}
}
