package docx

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const maxChartPoints = 32

var chartDecimal = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]{1,3})?$`)

type ChartValueEdit struct {
	Point         *int   `json:"point" jsonschema:"Zero-based point index from chartValues"`
	ExpectedValue string `json:"expectedValue" jsonschema:"Exact current numeric string from chartValues"`
	Value         string `json:"value" jsonschema:"New number as a decimal string, absolute value at most 10^15"`
}

func validChartValue(value string) bool {
	if len(value) > 64 || !chartDecimal.MatchString(value) {
		return false
	}
	number, err := strconv.ParseFloat(value, 64)
	return err == nil && math.Abs(number) <= 1e15
}

func hasChartDataSource(n *node) bool {
	if n.name.Space == chartNS {
		switch n.name.Local {
		case "externalData", "numRef", "strRef", "multiLvlStrRef", "f", "extLst":
			return true
		}
	}
	for _, child := range n.children {
		if hasChartDataSource(child) {
			return true
		}
	}
	return false
}

// A literal series has no workbook or formula to synchronize. A bounded,
// contiguous point list makes each editable index unambiguous.
func literalChartValues(title *node, data []byte) []*node {
	root := title
	for root.parent != nil {
		root = root.parent
	}
	if hasChartDataSource(root) {
		return nil
	}
	plot := onlyChartChild(onlyChartChild(root, "chart"), "plotArea")
	family := chartFamily(plot)
	values := onlyChartChild(onlyChartChild(family, "ser"), "val")
	literals := singlePayload(values)
	if literals == nil || literals.name.Space != chartNS || literals.name.Local != "numLit" {
		return nil
	}
	count := listInt(attribute(onlyChartChild(literals, "ptCount"), "", "val"), 0)
	if count < 1 || count > maxChartPoints {
		return nil
	}
	points := make([]*node, 0, count)
	for _, point := range literals.children {
		if point.name.Space != chartNS || point.name.Local != "pt" {
			continue
		}
		value := singlePayload(point)
		if listInt(attribute(point, "", "idx"), -1) != len(points) ||
			value == nil || value.name.Space != chartNS || value.name.Local != "v" ||
			len(value.children) != 0 || !validChartValue(value.text.String()) ||
			strings.ContainsRune(string(data[value.inner:value.endStart]), '<') {
			return nil
		}
		points = append(points, value)
	}
	if len(points) != count {
		return nil
	}
	return points
}

func (p *documentPackage) updateChartValue(t target, counts map[string]int, edit ChartValueEdit, changes map[string][]byte) error {
	relationship := chartRelationshipID(t.run)
	if !p.uniqueChart(relationship, counts) {
		return fmt.Errorf("unsupported or shared DOCX chart data")
	}
	title, filename, data := p.chartTitle(relationship)
	if title == nil {
		return fmt.Errorf("unsupported DOCX chart data")
	}
	values := literalChartValues(title, data)
	if edit.Point == nil || *edit.Point < 0 || *edit.Point >= len(values) || !validChartValue(edit.Value) || edit.Value == edit.ExpectedValue {
		return fmt.Errorf("invalid chart point or numeric value")
	}
	value := values[*edit.Point]
	if value.text.String() != edit.ExpectedValue {
		return fmt.Errorf("stale DOCX chart point value")
	}
	updated := make([]byte, 0, len(data))
	updated = append(updated, data[:value.inner]...)
	updated = append(updated, edit.Value...)
	changes[filename] = append(updated, data[value.endStart:]...)
	return nil
}
