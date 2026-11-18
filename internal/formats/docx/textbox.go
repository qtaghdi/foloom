package docx

import (
	"strings"
)

const (
	shapeWordNS    = "http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
	markupCompatNS = "http://schemas.openxmlformats.org/markup-compatibility/2006"
	visualMarkupNS = "urn:schemas-microsoft-com:vml"
)

func singlePayload(parent *node) *node {
	if parent == nil || len(parent.children) != 1 || strings.TrimSpace(parent.text.String()) != "" {
		return nil
	}
	return parent.children[0]
}

func drawingTextBox(drawing *node) *node {
	if drawing == nil || !drawing.is("drawing") {
		return nil
	}
	container := singlePayload(drawing)
	if container == nil || container.name.Space != drawingWordNS ||
		container.name.Local != "inline" && container.name.Local != "anchor" {
		return nil
	}
	graphic := childNS(childNS(container, drawingNS, "graphic"), drawingNS, "graphicData")
	shape := singlePayload(graphic)
	if attribute(graphic, "", "uri") != shapeWordNS || shape == nil ||
		shape.name.Space != shapeWordNS || shape.name.Local != "wsp" {
		return nil
	}
	box := childNS(shape, shapeWordNS, "txbx")
	if box == nil || !onlyNamespaceAttributes(box) || childNS(shape, shapeWordNS, "linkedTxbx") != nil {
		return nil
	}
	return singlePayload(box)
}

func fallbackTextBox(picture *node) *node {
	if picture == nil || !picture.is("pict") {
		return nil
	}
	shape := singlePayload(picture)
	if shape == nil || shape.name.Space != visualMarkupNS ||
		shape.name.Local != "rect" && shape.name.Local != "shape" {
		return nil
	}
	box := childNS(shape, visualMarkupNS, "textbox")
	if box == nil || strings.Contains(strings.ToLower(attribute(shape, "", "style")), "mso-next-textbox") ||
		strings.Contains(strings.ToLower(attribute(box, "", "style")), "mso-next-textbox") {
		return nil
	}
	return singlePayload(box)
}

func namespaceForPrefix(n *node, prefix string) string {
	for ; n != nil; n = n.parent {
		if uri := attribute(n, "xmlns", prefix); uri != "" {
			return uri
		}
	}
	return ""
}

func textBoxStories(run *node) []*node {
	runs := run.children
	if len(runs) > 0 && runs[0].is("rPr") {
		runs = runs[1:]
	}
	if len(runs) != 1 || strings.TrimSpace(run.text.String()) != "" {
		return nil
	}
	payload := runs[0]
	if box := drawingTextBox(payload); box != nil && box.is("txbxContent") {
		return []*node{box}
	}
	if box := fallbackTextBox(payload); box != nil && box.is("txbxContent") {
		return []*node{box}
	}
	if payload.name.Space != markupCompatNS || payload.name.Local != "AlternateContent" ||
		len(payload.children) != 2 || strings.TrimSpace(payload.text.String()) != "" {
		return nil
	}
	choice, fallback := payload.children[0], payload.children[1]
	requires := strings.Fields(attribute(choice, "", "Requires"))
	if choice.name.Space != markupCompatNS || choice.name.Local != "Choice" ||
		fallback.name.Space != markupCompatNS || fallback.name.Local != "Fallback" ||
		len(requires) != 1 || namespaceForPrefix(choice, requires[0]) != shapeWordNS {
		return nil
	}
	modernBox := drawingTextBox(singlePayload(choice))
	legacyBox := fallbackTextBox(singlePayload(fallback))
	if modernBox == nil || legacyBox == nil || !modernBox.is("txbxContent") || !legacyBox.is("txbxContent") {
		return nil
	}
	return []*node{modernBox, legacyBox}
}

func plainTextBoxStories(stories []*node, data []byte) ([][]*node, string) {
	var targets [][]*node
	var first string
	for _, box := range stories {
		p := singlePayload(box)
		if p == nil || !p.is("p") || strings.TrimSpace(p.text.String()) != "" || p.has("pPrChange") ||
			strings.Contains(string(data[box.start:box.end]), "<!--") {
			return nil, ""
		}
		runs := p.children
		if len(runs) > 0 && runs[0].is("pPr") {
			runs = runs[1:]
		}
		nodes, text := plainTextRunNodes(runs)
		if len(nodes) == 0 || len(targets) != 0 && first != text {
			return nil, ""
		}
		targets = append(targets, nodes)
		first = text
	}
	return targets, first
}

func hasLinkedTextBox(n *node) bool {
	if n.name.Space == shapeWordNS && n.name.Local == "linkedTxbx" {
		return true
	}
	for _, child := range n.children {
		if hasLinkedTextBox(child) {
			return true
		}
	}
	return false
}
