package docx

import (
	"encoding/xml"
	"fmt"
	"strings"
)

const inlinePlacement = "inline"

var pictureAnchorAttrs = map[string]string{
	"distT": "0", "distB": "0", "distL": "0", "distR": "0",
	"simplePos": "0", "relativeHeight": "0", "behindDoc": "0",
	"locked": "0", "layoutInCell": "1", "allowOverlap": "0",
}

func drawingFields(n *node, values map[string]string) bool {
	seen := make(map[string]bool, len(values))
	for _, attr := range n.attrs {
		if isNamespaceAttribute(attr) {
			continue
		}
		value, ok := values[attr.Name.Local]
		if attr.Name.Space != "" || !ok || value != attr.Value || seen[attr.Name.Local] {
			return false
		}
		seen[attr.Name.Local] = true
	}
	return len(seen) == len(values)
}

func supportedPicturePlacement(t target, data []byte, layout ParagraphLayout) string {
	if !plainVisualParagraphProperties(t.paragraph) || !dedicatedDrawingParagraph(t, data) ||
		layout.Bidi || layout.TextDirection != "" || layout.Left != nil || layout.Right != nil ||
		layout.FirstLine != nil || layout.Hanging != nil || t.paragraph.child("pPr").child("ind") != nil {
		return ""
	}
	frame := t.run.child("drawing").children[0]
	mode, _, _ := picturePlacementContent(frame, data)
	return mode
}

// Placement changes only the frame around a simple unrotated drawing. The
// original extent and graphic bytes remain the inline fallback on conversion.
func picturePlacementContent(frame *node, data []byte) (mode string, first, rest []byte) {
	if frame.name.Space != drawingWordNS || strings.TrimSpace(frame.text.String()) != "" {
		return "", nil, nil
	}
	if _, rotation, ok := supportedPictureRotation(frame, data); !ok || rotation != 0 {
		return "", nil, nil
	}
	children := frame.children
	offset := frame.inner
	mode = inlinePlacement
	if frame.name.Local == "anchor" {
		if len(children) < 7 || !drawingFields(frame, pictureAnchorAttrs) {
			return "", nil, nil
		}
		pos, horizontal, vertical, wrap := children[0], children[1], children[2], children[4]
		if !sameDrawingName(pos, "simplePos") || !drawingFields(pos, map[string]string{"x": "0", "y": "0"}) ||
			len(pos.children) != 0 || strings.TrimSpace(pos.text.String()) != "" ||
			!sameDrawingName(horizontal, "positionH") || !drawingFields(horizontal, map[string]string{"relativeFrom": "column"}) ||
			len(horizontal.children) != 1 || strings.TrimSpace(horizontal.text.String()) != "" ||
			!sameDrawingName(vertical, "positionV") || !drawingFields(vertical, map[string]string{"relativeFrom": "paragraph"}) ||
			len(vertical.children) != 1 || strings.TrimSpace(vertical.text.String()) != "" ||
			!sameDrawingName(wrap, "wrapTopAndBottom") || !drawingFields(wrap, nil) ||
			len(wrap.children) != 0 || strings.TrimSpace(wrap.text.String()) != "" {
			return "", nil, nil
		}
		x, y := horizontal.children[0], vertical.children[0]
		if !sameDrawingName(x, "align") || !drawingFields(x, nil) || len(x.children) != 0 ||
			!sameDrawingName(y, "posOffset") || !drawingFields(y, nil) || len(y.children) != 0 || y.text.String() != "0" {
			return "", nil, nil
		}
		switch x.text.String() {
		case "left", "center", "right":
			mode = "floating_" + x.text.String()
		default:
			return "", nil, nil
		}
		offset = vertical.end
		children = append([]*node{children[3]}, children[5:]...)
		rest = data[wrap.end:frame.endStart]
	} else if frame.name.Local != "inline" || !drawingFields(frame, nil) {
		return "", nil, nil
	}
	if len(children) < 3 || len(children) > 4 || !sameDrawingName(children[0], "extent") ||
		!sameDrawingName(children[1], "docPr") ||
		len(children) == 4 && !sameDrawingName(children[2], "cNvGraphicFramePr") {
		return "", nil, nil
	}
	graphic := children[len(children)-1]
	dataNode := childNS(graphic, drawingNS, "graphicData")
	if graphic.name.Space != drawingNS || graphic.name.Local != "graphic" || len(graphic.children) != 1 ||
		dataNode == nil || attribute(dataNode, "", "uri") != pictureNS || len(dataNode.children) != 1 ||
		childNS(dataNode, pictureNS, "pic") == nil {
		return "", nil, nil
	}
	extent := children[0]
	first = data[offset:extent.end]
	if mode == inlinePlacement {
		rest = data[extent.end:frame.endStart]
	}
	return mode, first, rest
}

func sameDrawingName(n *node, name string) bool {
	return n.name.Space == drawingWordNS && n.name.Local == name
}

func (p *documentPackage) setPicturePlacement(t target, placement string) (replacement, error) {
	fail := func() (replacement, error) {
		return replacement{}, fmt.Errorf("unsupported DOCX picture placement or unchanged layout")
	}
	switch placement {
	case inlinePlacement, "floating_left", "floating_center", "floating_right":
	default:
		return fail()
	}
	if t.kind != "picture" || t.picturePlacement == "" || t.picturePlacement == placement {
		return fail()
	}
	frame := t.run.child("drawing").children[0]
	if _, relation := pictureFrameInfo(frame); relation == "" {
		return fail()
	} else if _, _, err := imageConfig(p.partData(p.picturePartMap()[relation], nil), maxImageReadBytes); err != nil {
		return replacement{}, err
	}
	_, first, rest := picturePlacementContent(frame, p.document)
	token, err := xml.NewDecoder(strings.NewReader(string(p.document[frame.start:frame.inner]))).RawToken()
	if err != nil {
		return replacement{}, err
	}
	element, ok := token.(xml.StartElement)
	if !ok {
		return fail()
	}
	prefix := ""
	if element.Name.Space != "" {
		prefix = element.Name.Space + ":"
	}
	kind := "inline"
	if placement != inlinePlacement {
		kind = "anchor"
	}
	var raw strings.Builder
	raw.WriteString("<" + prefix + kind)
	for _, attr := range element.Attr {
		if attr.Name.Space == "xmlns" {
			raw.WriteString(` xmlns:` + attr.Name.Local + `="` + escapeText(attr.Value) + `"`)
		} else if attr.Name.Space == "" && attr.Name.Local == "xmlns" {
			raw.WriteString(` xmlns="` + escapeText(attr.Value) + `"`)
		}
	}
	if kind == "inline" {
		raw.WriteString(">")
	} else {
		raw.WriteString(` distT="0" distB="0" distL="0" distR="0" simplePos="0" relativeHeight="0" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="0">`)
		raw.WriteString("<" + prefix + `simplePos x="0" y="0"/>`)
		raw.WriteString("<" + prefix + `positionH relativeFrom="column"><` + prefix + "align>" +
			strings.TrimPrefix(placement, "floating_") + "</" + prefix + "align></" + prefix + "positionH>")
		raw.WriteString("<" + prefix + `positionV relativeFrom="paragraph"><` + prefix + "posOffset>0</" +
			prefix + "posOffset></" + prefix + "positionV>")
	}
	raw.Write(first)
	if kind == "anchor" {
		raw.WriteString("<" + prefix + "wrapTopAndBottom/>")
	}
	raw.Write(rest)
	raw.WriteString("</" + prefix + kind + ">")
	return replacement{frame.start, frame.end, []byte(raw.String())}, nil
}
