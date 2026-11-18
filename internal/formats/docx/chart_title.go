package docx

import (
	"fmt"
	"path"
	"strings"
)

const (
	chartNS          = "http://schemas.openxmlformats.org/drawingml/2006/chart"
	chartContentType = "application/vnd.openxmlformats-officedocument.drawingml.chart+xml"
	maxChartTitle    = 256
)

func chartPartPath(name string) bool {
	return path.Dir(name) == "word/charts" && strings.HasSuffix(name, ".xml")
}

func chartRelationshipID(run *node) string {
	drawing := run.child("drawing")
	container := singlePayload(drawing)
	if container == nil || container.name.Space != drawingWordNS ||
		container.name.Local != "inline" && container.name.Local != "anchor" {
		return ""
	}
	graphic := childNS(childNS(container, drawingNS, "graphic"), drawingNS, "graphicData")
	chart := singlePayload(graphic)
	if attribute(graphic, "", "uri") != chartNS || chart == nil || chart.name.Space != chartNS || chart.name.Local != "chart" {
		return ""
	}
	return attribute(chart, officeRelNS, "id")
}

func (p *documentPackage) chartUseCounts(root *node) map[string]int {
	counts := map[string]int{}
	var visit func(*node)
	visit = func(n *node) {
		for _, attr := range n.attrs {
			if attr.Name.Space == officeRelNS && p.chartRelationships[attr.Value] != "" {
				counts[attr.Value]++
			}
		}
		for _, child := range n.children {
			visit(child)
		}
	}
	visit(root)
	return counts
}

func (p *documentPackage) chartTitle(relationship string) (*node, string, []byte) {
	filename := p.chartRelationships[relationship]
	if p.chartContentTypes[filename] != 1 {
		return nil, "", nil
	}
	data := p.partData(filename, nil)
	// Chart data stays opaque. Bound the XML inspected for title projection.
	if len(data) > 2<<20 {
		return nil, "", nil
	}
	root, err := parseXML(data)
	if err != nil || root.name.Space != chartNS || root.name.Local != "chartSpace" {
		return nil, "", nil
	}
	if !p.localChartData(root, filename) {
		return nil, "", nil
	}
	chart := onlyChartChild(root, "chart")
	title := onlyChartChild(chart, "title")
	tx := onlyChartChild(title, "tx")
	rich := singlePayload(tx)
	if chartFamily(onlyChartChild(chart, "plotArea")) == nil || rich == nil || rich.name.Space != chartNS || rich.name.Local != "rich" {
		return nil, "", nil
	}
	paragraph := plainChartTextChild(rich, "p", "bodyPr", "lstStyle")
	run := plainChartTextChild(paragraph, "r", "pPr", "endParaRPr")
	text := plainChartTextChild(run, "t", "rPr")
	if text == nil || len(text.children) != 0 || !plainChartTitle(text.text.String()) ||
		strings.ContainsRune(string(data[text.inner:text.endStart]), '<') {
		return nil, "", nil
	}
	return text, filename, data
}

func chartFamily(plot *node) *node {
	if plot == nil {
		return nil
	}
	var family *node
	for _, child := range plot.children {
		if child.name.Space != chartNS || !strings.HasSuffix(child.name.Local, "Chart") {
			continue
		}
		if family != nil {
			return nil
		}
		switch child.name.Local {
		case "barChart", "lineChart", "pieChart":
			family = child
		default:
			return nil
		}
	}
	return family
}

func plainChartTitle(value string) bool {
	return len(value) <= maxChartTitle && strings.TrimSpace(value) != "" && validText(value)
}

func onlyChartChild(parent *node, local string) *node {
	if parent == nil {
		return nil
	}
	var child *node
	for _, n := range parent.children {
		if n.name.Space == chartNS && n.name.Local == local {
			if child != nil {
				return nil
			}
			child = n
		}
	}
	return child
}

func plainChartTextChild(parent *node, local string, formatting ...string) *node {
	if parent == nil || strings.TrimSpace(parent.text.String()) != "" {
		return nil
	}
	var result *node
	for _, child := range parent.children {
		if child.name.Space != drawingNS {
			return nil
		}
		if child.name.Local == local && result == nil {
			result = child
			continue
		}
		supported := false
		for _, property := range formatting {
			supported = supported || child.name.Local == property
		}
		if !supported {
			return nil
		}
	}
	return result
}

func (p *documentPackage) localChartData(chart *node, filename string) bool {
	relationshipID := ""
	for _, child := range chart.children {
		if child.name.Space == chartNS && child.name.Local == "externalData" {
			if relationshipID != "" {
				return false
			}
			relationshipID = attribute(child, officeRelNS, "id")
			if relationshipID == "" {
				return false
			}
		}
	}
	data := p.partData(path.Join(path.Dir(filename), "_rels", path.Base(filename)+".rels"), nil)
	if len(data) == 0 {
		return relationshipID == ""
	}
	root, err := parseXML(data)
	if err != nil {
		return false
	}
	embedded := relationshipID == ""
	for _, rel := range root.children {
		if strings.EqualFold(attribute(rel, "", "TargetMode"), "External") {
			return false
		}
		if attribute(rel, "", "Id") == relationshipID {
			embedded = attribute(rel, "", "Type") == officeRelNS+"/package" &&
				strings.HasSuffix(strings.ToLower(attribute(rel, "", "Target")), ".xlsx")
		}
	}
	return embedded
}

func (p *documentPackage) projectChartTitles(root *node, blocks []Block, targets map[string]target) {
	if len(p.chartRelationships) == 0 {
		return
	}
	var counts map[string]int
	type chartPreview struct {
		title  string
		values []string
	}
	previews := map[string]chartPreview{}
	for index := range blocks {
		paragraph := blocks[index].Paragraph
		if paragraph == nil {
			continue
		}
		for i := range paragraph.Runs {
			run := &paragraph.Runs[i]
			t := targets[run.ID]
			if t.kind != "chart" {
				continue
			}
			if counts == nil {
				counts = p.chartUseCounts(root)
			}
			relationship := chartRelationshipID(t.run)
			filename := p.chartRelationships[relationship]
			preview, cached := previews[filename]
			editable := p.uniqueChart(relationship, counts)
			if !cached {
				title, _, data := p.chartTitle(relationship)
				if title != nil {
					preview.title = title.text.String()
					if editable {
						for _, point := range literalChartValues(title, data) {
							preview.values = append(preview.values, point.text.String())
						}
					}
				}
				previews[filename] = preview
			}
			if preview.title != "" {
				run.Object.Text = preview.title
				run.Object.DeleteEditable = deletableInlineChart(t, p.document)
				run.Object.MoveChartUp, run.Object.MoveChartDown = t.moveUp != nil, t.moveDown != nil
				run.Object.Editable = editable
				if editable {
					run.Object.ChartFrame, _ = inlineChartFrame(t.run, p.document)
					run.Object.ChartValues = preview.values
				}
			}
		}
	}
}

func (p *documentPackage) uniqueChart(relationship string, counts map[string]int) bool {
	return p.chartReferences[p.chartRelationships[relationship]] == 1 && counts[relationship] == 1
}

func (p *documentPackage) updateChartTitle(t target, counts map[string]int, op Operation, changes map[string][]byte) error {
	relationship := chartRelationshipID(t.run)
	if !p.uniqueChart(relationship, counts) {
		return fmt.Errorf("unsupported or shared DOCX chart title")
	}
	title, filename, data := p.chartTitle(relationship)
	if title == nil {
		return fmt.Errorf("unsupported DOCX chart title structure")
	}
	if op.ExpectedText != title.text.String() || !plainChartTitle(op.Text) || op.Text == op.ExpectedText {
		return fmt.Errorf("invalid, unchanged or stale DOCX chart title")
	}
	updated := make([]byte, 0, len(data))
	updated = append(updated, data[:title.inner]...)
	updated = append(updated, escapeText(op.Text)...)
	changes[filename] = append(updated, data[title.endStart:]...)
	return nil
}
