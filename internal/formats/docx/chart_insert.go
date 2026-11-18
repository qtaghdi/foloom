package docx

import (
	"fmt"
	"strconv"
	"strings"
)

const maxPieChartPoints = 8

type ChartInsert struct {
	Title  string            `json:"title" jsonschema:"Plain chart title, up to 256 UTF-8 bytes"`
	Points []ChartInputPoint `json:"points" jsonschema:"One to eight points in display order"`
}

type ChartInputPoint struct {
	Label string `json:"label" jsonschema:"Plain category label, up to 120 UTF-8 bytes"`
	Value string `json:"value" jsonschema:"Nonnegative decimal string, at most 10^15"`
}

func validPieChartPoints(points []ChartInputPoint) bool {
	if len(points) < 1 || len(points) > maxPieChartPoints {
		return false
	}
	positive := false
	for _, point := range points {
		if len(point.Label) > 120 || strings.TrimSpace(point.Label) == "" || !validText(point.Label) ||
			!validChartValue(point.Value) {
			return false
		}
		value, _ := strconv.ParseFloat(point.Value, 64)
		if value < 0 {
			return false
		}
		positive = positive || value > 0
	}
	return positive
}

func (p *documentPackage) insertPieChart(paragraph *node, input ChartInsert, drawingID uint32, parts map[string][]byte) (replacement, error) {
	if !plainChartTitle(input.Title) || !validPieChartPoints(input.Points) {
		return replacement{}, fmt.Errorf("DOCX pie chart requires a title and 1 to 8 plain categories with nonnegative values and at least one positive value")
	}
	filename := p.availablePart(parts, "word/charts/foloom-chart", ".xml")
	relation, err := p.addRelatedPart(parts, filename, officeRelNS+"/chart", chartContentType, fmt.Sprintf("rIdFoloomChart%d", drawingID))
	if err != nil {
		return replacement{}, err
	}
	parts[filename] = []byte(pieChartXML(input))
	drawing := fmt.Sprintf(`<dwc:r xmlns:dwc="%s" xmlns:wp="%s" xmlns:a="%s" xmlns:c="%s" xmlns:r="%s"><dwc:drawing><wp:inline><wp:extent cx="3657600" cy="2438400"/><wp:docPr id="%d" name="Chart %d"/><wp:cNvGraphicFramePr/><a:graphic><a:graphicData uri="%s"><c:chart r:id="%s"/></a:graphicData></a:graphic></wp:inline></dwc:drawing></dwc:r>`,
		wordNS, drawingWordNS, drawingNS, chartNS, officeRelNS, drawingID, drawingID, chartNS, relation)
	return appendParagraphContent(p.document, paragraph, drawing), nil
}

func pieChartXML(input ChartInsert) string {
	colors := [maxPieChartPoints]string{"167D79", "D66D46", "4C6DB0", "9D68AA", "A56D29", "577641", "BD5B86", "516473"}
	var styles, labels, values strings.Builder
	for index, point := range input.Points {
		// Explicit fills keep each category visible without a theme or workbook.
		fmt.Fprintf(&styles, `<c:dPt><c:idx val="%d"/><c:spPr><a:solidFill><a:srgbClr val="%s"/></a:solidFill></c:spPr></c:dPt>`, index, colors[index])
		fmt.Fprintf(&labels, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, index, escapeText(point.Label))
		fmt.Fprintf(&values, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, index, point.Value)
	}
	return fmt.Sprintf(`<c:chartSpace xmlns:c="%s" xmlns:a="%s"><c:chart>
<c:title><c:tx><c:rich><a:bodyPr/><a:p><a:r><a:t>%s</a:t></a:r></a:p></c:rich></c:tx><c:layout/><c:overlay val="0"/></c:title>
<c:plotArea><c:layout/><c:pieChart><c:varyColors val="1"/>
<c:ser><c:idx val="0"/><c:order val="0"/>%s
<c:dLbls><c:showLegendKey val="0"/><c:showVal val="1"/><c:showCatName val="0"/><c:showSerName val="0"/><c:showPercent val="0"/><c:showBubbleSize val="0"/></c:dLbls>
<c:cat><c:strLit><c:ptCount val="%d"/>%s</c:strLit></c:cat>
<c:val><c:numLit><c:formatCode>General</c:formatCode><c:ptCount val="%d"/>%s</c:numLit></c:val>
</c:ser><c:firstSliceAng val="0"/></c:pieChart></c:plotArea>
<c:legend><c:legendPos val="r"/><c:layout/><c:overlay val="0"/></c:legend>
<c:plotVisOnly val="1"/></c:chart></c:chartSpace>`,
		chartNS, drawingNS, escapeText(input.Title), styles.String(), len(input.Points), labels.String(), len(input.Points), values.String())
}
