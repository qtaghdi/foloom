package docx

import (
	"fmt"
	"strconv"
	"strings"
)

const maxChartInsertPoints = 8

type ChartInsert struct {
	Kind   string            `json:"kind,omitempty" jsonschema:"pie, bar for vertical columns, or line. Omit to create a pie chart"`
	Title  string            `json:"title" jsonschema:"Plain chart title, up to 256 UTF-8 bytes"`
	Points []ChartInputPoint `json:"points" jsonschema:"One to eight points in display order"`
}

type ChartInputPoint struct {
	Label string `json:"label" jsonschema:"Plain category label, up to 120 UTF-8 bytes"`
	Value string `json:"value" jsonschema:"Decimal string with absolute value at most 10^15. Pie values must be nonnegative"`
}

func validChartInsertPoints(kind string, points []ChartInputPoint) bool {
	if len(points) < 1 || len(points) > maxChartInsertPoints || kind == "line" && len(points) < 2 {
		return false
	}
	positive := false
	for _, point := range points {
		if len(point.Label) > 120 || strings.TrimSpace(point.Label) == "" || !validText(point.Label) ||
			!validChartValue(point.Value) {
			return false
		}
		if kind == "pie" {
			value, _ := strconv.ParseFloat(point.Value, 64)
			if value < 0 {
				return false
			}
			positive = positive || value > 0
		}
	}
	return kind != "pie" || positive
}

func (p *documentPackage) insertChart(paragraph *node, input ChartInsert, drawingID uint32, parts map[string][]byte) (replacement, error) {
	if input.Kind == "" {
		input.Kind = "pie"
	}
	if input.Kind != "pie" && input.Kind != "bar" && input.Kind != "line" {
		return replacement{}, fmt.Errorf("unsupported DOCX chart type")
	}
	if !plainChartTitle(input.Title) || !validChartInsertPoints(input.Kind, input.Points) {
		return replacement{}, fmt.Errorf("invalid DOCX chart title, categories, or values")
	}
	filename := p.availablePart(parts, "word/charts/foloom-chart", ".xml")
	relation, err := p.addRelatedPart(parts, filename, officeRelNS+"/chart", chartContentType, fmt.Sprintf("rIdFoloomChart%d", drawingID))
	if err != nil {
		return replacement{}, err
	}
	parts[filename] = []byte(nativeChartXML(input))
	drawing := fmt.Sprintf(`<dwc:r xmlns:dwc="%s" xmlns:wp="%s" xmlns:a="%s" xmlns:c="%s" xmlns:r="%s"><dwc:drawing><wp:inline><wp:extent cx="3657600" cy="2438400"/><wp:docPr id="%d" name="Chart %d"/><wp:cNvGraphicFramePr/><a:graphic><a:graphicData uri="%s"><c:chart r:id="%s"/></a:graphicData></a:graphic></wp:inline></dwc:drawing></dwc:r>`,
		wordNS, drawingWordNS, drawingNS, chartNS, officeRelNS, drawingID, drawingID, chartNS, relation)
	return appendParagraphContent(p.document, paragraph, drawing), nil
}

const chartPointLabels = `<c:dLbls><c:showLegendKey val="0"/><c:showVal val="1"/><c:showCatName val="0"/><c:showSerName val="0"/><c:showPercent val="0"/><c:showBubbleSize val="0"/></c:dLbls>`

func nativeChartXML(input ChartInsert) string {
	var labels, values strings.Builder
	for index, point := range input.Points {
		fmt.Fprintf(&labels, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, index, escapeText(point.Label))
		fmt.Fprintf(&values, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, index, point.Value)
	}
	series := fmt.Sprintf(`%s<c:cat><c:strLit><c:ptCount val="%d"/>%s</c:strLit></c:cat>
<c:val><c:numLit><c:formatCode>General</c:formatCode><c:ptCount val="%d"/>%s</c:numLit></c:val>`,
		chartPointLabels, len(input.Points), labels.String(), len(input.Points), values.String())
	var plot, legend string
	if input.Kind == "pie" {
		plot = pieChartPlot(series, len(input.Points))
		legend = `<c:legend><c:legendPos val="r"/><c:layout/><c:overlay val="0"/></c:legend>`
	} else {
		plot = axisChartPlot(input.Kind, series)
	}
	return fmt.Sprintf(`<c:chartSpace xmlns:c="%s" xmlns:a="%s"><c:chart>
<c:title><c:tx><c:rich><a:bodyPr/><a:p><a:r><a:t>%s</a:t></a:r></a:p></c:rich></c:tx><c:layout/><c:overlay val="0"/></c:title>
<c:plotArea><c:layout/>%s</c:plotArea>%s<c:plotVisOnly val="1"/></c:chart></c:chartSpace>`,
		chartNS, drawingNS, escapeText(input.Title), plot, legend)
}

func pieChartPlot(series string, points int) string {
	colors := [maxChartInsertPoints]string{"167D79", "D66D46", "4C6DB0", "9D68AA", "A56D29", "577641", "BD5B86", "516473"}
	var styles strings.Builder
	for index, color := range colors[:points] {
		// Explicit fills keep each category visible without a source theme.
		fmt.Fprintf(&styles, `<c:dPt><c:idx val="%d"/><c:spPr><a:solidFill><a:srgbClr val="%s"/></a:solidFill></c:spPr></c:dPt>`, index, color)
	}
	return `<c:pieChart><c:varyColors val="1"/><c:ser><c:idx val="0"/><c:order val="0"/>` + styles.String() + series +
		`</c:ser><c:firstSliceAng val="0"/></c:pieChart>`
}

func axisChartPlot(kind, series string) string {
	const color = `<a:solidFill><a:srgbClr val="167D79"/></a:solidFill>`
	const axisIDs = `<c:axId val="1"/><c:axId val="2"/>`
	const seriesStart = `<c:ser><c:idx val="0"/><c:order val="0"/>`
	var family, crossBetween string
	if kind == "bar" {
		family = `<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/><c:varyColors val="0"/>` +
			seriesStart + `<c:spPr>` + color + `</c:spPr><c:invertIfNegative val="0"/>` + series +
			`</c:ser><c:gapWidth val="150"/>` + axisIDs + `</c:barChart>`
		crossBetween = "between"
	} else {
		line := `<a:ln w="28575">` + color + `<a:prstDash val="solid"/></a:ln>`
		marker := `<c:marker><c:symbol val="circle"/><c:size val="6"/><c:spPr>` + color + line + `</c:spPr></c:marker>`
		family = `<c:lineChart><c:grouping val="standard"/><c:varyColors val="0"/>` + seriesStart +
			`<c:spPr>` + line + `</c:spPr>` + marker + series + `<c:smooth val="0"/></c:ser><c:marker val="1"/>` +
			axisIDs + `</c:lineChart>`
		crossBetween = "midCat"
	}
	return family + `<c:catAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:tickLblPos val="low"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblAlgn val="ctr"/><c:lblOffset val="100"/></c:catAx>
<c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:majorGridlines/><c:numFmt formatCode="General" sourceLinked="0"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="` +
		crossBetween + `"/></c:valAx>`
}
