package docx

import (
	"path"
	"strings"
)

const (
	diagramNS              = "http://schemas.openxmlformats.org/drawingml/2006/diagram"
	diagramDataContentType = "application/vnd.openxmlformats-officedocument.drawingml.diagramData+xml"
	maxSmartArtItems       = 32
	maxSmartArtItemBytes   = 256
	maxSmartArtPreviews    = 64
)

type SmartArtPreview struct {
	Items     []string `json:"items"`
	Truncated bool     `json:"truncated,omitempty"`
}

func diagramDataID(run *node) string {
	container := singlePayload(run.child("drawing"))
	if container == nil || container.name.Space != drawingWordNS ||
		container.name.Local != "inline" && container.name.Local != "anchor" {
		return ""
	}
	graphic := childNS(childNS(container, drawingNS, "graphic"), drawingNS, "graphicData")
	relations := singlePayload(graphic)
	if attribute(graphic, "", "uri") != diagramNS || relations == nil ||
		relations.name.Space != diagramNS || relations.name.Local != "relIds" {
		return ""
	}
	return attribute(relations, officeRelNS, "dm")
}

func (p *documentPackage) diagramParts() map[string]string {
	out := map[string]string{}
	manifest, err := parseXML(p.partData("[Content_Types].xml", nil))
	if err != nil {
		return out
	}
	types := map[string]bool{}
	for _, child := range manifest.children {
		if attribute(child, "", "ContentType") == diagramDataContentType {
			types[strings.TrimPrefix(attribute(child, "", "PartName"), "/")] = true
		}
	}
	rels, err := parseXML(p.partData(documentRelationships, nil))
	if err != nil {
		return out
	}
	for _, rel := range rels.children {
		mode := attribute(rel, "", "TargetMode")
		if rel.name.Space != packageRelNS || rel.name.Local != "Relationship" ||
			attribute(rel, "", "Type") != officeRelNS+"/diagramData" || mode != "" && mode != "Internal" {
			continue
		}
		filename := attribute(rel, "", "Target")
		if strings.HasPrefix(filename, "/") {
			filename = strings.TrimPrefix(filename, "/")
		} else {
			filename = path.Join("word", filename)
		}
		if path.Dir(filename) == "word/diagrams" && strings.HasSuffix(filename, ".xml") && types[filename] {
			out[attribute(rel, "", "Id")] = filename
		}
	}
	return out
}

func diagramPointLabel(point *node) string {
	kind := attribute(point, "", "type")
	if kind != "" && kind != "node" && kind != "asst" ||
		onOff(attribute(childNS(point, diagramNS, "prSet"), "", "phldr"), false) {
		return ""
	}
	body := childNS(point, diagramNS, "t")
	if body == nil {
		return ""
	}
	var label strings.Builder
	for _, paragraph := range body.children {
		if paragraph.name.Space != drawingNS || paragraph.name.Local != "p" {
			continue
		}
		if label.Len() != 0 {
			label.WriteByte(' ')
		}
		for _, run := range paragraph.children {
			if run.name.Space != drawingNS {
				continue
			}
			switch run.name.Local {
			case "r":
				if text := childNS(run, drawingNS, "t"); text != nil && len(text.children) == 0 {
					label.WriteString(text.text.String())
				}
			case "br":
				label.WriteByte(' ')
			}
		}
	}
	return strings.Join(strings.Fields(label.String()), " ")
}

func previewSmartArt(data []byte) *SmartArtPreview {
	if len(data) == 0 || len(data) > 1<<20 {
		return nil
	}
	root, err := parseXML(data)
	if err != nil || root.name.Space != diagramNS || root.name.Local != "dataModel" {
		return nil
	}
	points := childNS(root, diagramNS, "ptLst")
	if points == nil {
		return nil
	}
	preview := &SmartArtPreview{}
	for _, point := range points.children {
		if point.name.Space != diagramNS || point.name.Local != "pt" {
			continue
		}
		label := diagramPointLabel(point)
		if label == "" {
			continue
		}
		if len(preview.Items) == maxSmartArtItems {
			preview.Truncated = true
			break
		}
		preview.Truncated = preview.Truncated || len(label) > maxSmartArtItemBytes
		preview.Items = append(preview.Items, reviewText(label, maxSmartArtItemBytes))
	}
	if len(preview.Items) == 0 {
		return nil
	}
	return preview
}

func (p *documentPackage) projectSmartArt(blocks []Block, targets map[string]target) {
	var parts map[string]string
	cache := map[string]*SmartArtPreview{}
	count := 0
	project := func(paragraph *Paragraph) {
		for index := range paragraph.Runs {
			if count == maxSmartArtPreviews {
				return
			}
			run := &paragraph.Runs[index]
			if run.Object == nil || run.Object.Kind != "smartart" {
				continue
			}
			id := diagramDataID(targets[run.ID].run)
			if id == "" {
				continue
			}
			if parts == nil {
				parts = p.diagramParts()
			}
			filename := parts[id]
			if filename == "" {
				continue
			}
			preview, cached := cache[filename]
			if !cached {
				// Only the data model is read. Layout, drawing and other relationships
				// stay opaque, including links found inside the data model itself.
				preview = previewSmartArt(p.partData(filename, nil))
				cache[filename] = preview
			}
			if preview != nil {
				run.Object.SmartArt = preview
				count++
			}
		}
	}
	for i := range blocks {
		block := &blocks[i]
		if block.Paragraph != nil {
			project(block.Paragraph)
		}
		for r := range block.Rows {
			for c := range block.Rows[r].Cells {
				for p := range block.Rows[r].Cells[c].Paragraphs {
					project(&block.Rows[r].Cells[c].Paragraphs[p])
				}
			}
		}
	}
}
