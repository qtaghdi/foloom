package docx

import (
	"encoding/xml"
	"fmt"
	"strconv"
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

func pictureColumnPage(body *node) (PageLayout, bool) {
	section := body.child("sectPr")
	cols := section.child("cols")
	if countChild(body, "sectPr") > 1 || section != nil && body.children[len(body.children)-1] != section ||
		countChild(section, "cols") > 1 || section.child("bidi") != nil ||
		(cols != nil && (len(cols.children) != 0 || cols.attr("num") != "" && cols.attr("num") != "1")) {
		return PageLayout{}, false
	}
	for _, block := range body.children {
		if block.is("p") && block.child("pPr").child("sectPr") != nil {
			return PageLayout{}, false
		}
	}
	page := sectionPage(section)
	if gutter := previewNumber(section.child("pgMar").attr("gutter"), 0, 50000); gutter != nil {
		page.Left += *gutter
	}
	return page, page.Width-page.Left-page.Right > 0
}

// Square wrapping is editable within a simple single-column text flow. Picture
// placement outside this subset can still be preserved or changed back.
func canSquareWrapPicture(t target, data []byte, page PageLayout) bool {
	body := t.paragraph.parent
	section := body.child("sectPr")
	frame := t.run.child("drawing").children[0]
	info, _ := pictureFrameInfo(frame)
	if info == nil {
		return false
	}
	bodyWidth := page.Width - page.Left - page.Right
	if bodyWidth <= 0 || info.WidthEMU*2 > bodyWidth*635 ||
		info.HeightEMU > (page.Height-page.Top-page.Bottom)*635 {
		return false
	}
	foundPicture, followingText := false, false
	for _, block := range body.children {
		if block == t.paragraph {
			foundPicture = true
		} else if block != section {
			if !block.is("p") || block.has("br") || !plainVisualParagraphProperties(block) ||
				!structuralParagraph(block, data) {
				return false
			}
			if foundPicture && strings.TrimSpace(textContent(block)) != "" {
				followingText = true
			}
		}
	}
	return followingText
}

func pictureOffsetLimit(t target, page PageLayout) *int {
	frame := t.run.child("drawing").children[0]
	info, _ := pictureFrameInfo(frame)
	if info == nil {
		return nil
	}
	limit := (page.Width-page.Left-page.Right)*635 - info.WidthEMU
	if limit < 0 {
		return nil
	}
	return &limit
}

func pictureHorizontalOffset(frame *node) *int {
	x := childNS(childNS(frame, drawingWordNS, "positionH"), drawingWordNS, "posOffset")
	if x == nil || !drawingFields(x, nil) || len(x.children) != 0 {
		return nil
	}
	value, err := strconv.ParseInt(x.text.String(), 10, 32)
	if err != nil || value < 0 || value > 50000*635 {
		return nil
	}
	offset := int(value)
	return &offset
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
			len(wrap.children) != 0 || strings.TrimSpace(wrap.text.String()) != "" {
			return "", nil, nil
		}
		x, y := horizontal.children[0], vertical.children[0]
		if !drawingFields(x, nil) || len(x.children) != 0 ||
			!sameDrawingName(y, "posOffset") || !drawingFields(y, nil) || len(y.children) != 0 || y.text.String() != "0" {
			return "", nil, nil
		}
		side := x.text.String()
		aligned := sameDrawingName(x, "align")
		topAndBottom := sameDrawingName(wrap, "wrapTopAndBottom") && drawingFields(wrap, nil)
		switch {
		case topAndBottom && pictureHorizontalOffset(frame) != nil:
			mode = "floating_offset"
		case aligned && sameDrawingName(wrap, "wrapSquare") && squareWrapTextSide(side) != "" &&
			drawingFields(wrap, map[string]string{"wrapText": squareWrapTextSide(side)}):
			mode = "square_" + side
		case aligned && topAndBottom && (side == "left" || side == "center" || side == "right"):
			mode = "floating_" + side
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

func squareWrapTextSide(pictureSide string) string {
	switch pictureSide {
	case "left":
		return "right"
	case "right":
		return "left"
	default:
		return ""
	}
}

func sameDrawingName(n *node, name string) bool {
	return n.name.Space == drawingWordNS && n.name.Local == name
}

func (p *documentPackage) setPicturePlacement(t target, placement string, offset *int) (replacement, error) {
	fail := func() (replacement, error) {
		return replacement{}, fmt.Errorf("unsupported DOCX picture placement or unchanged layout")
	}
	if placement != "floating_offset" && offset != nil {
		return fail()
	}
	switch placement {
	case "floating_offset":
		if offset == nil || t.pictureOffsetLimit == nil || *offset < 0 || *offset > *t.pictureOffsetLimit {
			return fail()
		}
	case inlinePlacement, "floating_left", "floating_center", "floating_right":
	case "square_left", "square_right":
		if !t.squareWrap {
			return fail()
		}
	default:
		return fail()
	}
	if t.kind != "picture" || t.picturePlacement == "" {
		return fail()
	}
	frame := t.run.child("drawing").children[0]
	if t.picturePlacement == placement {
		current := pictureHorizontalOffset(frame)
		if offset == nil || current != nil && *current == *offset {
			return fail()
		}
	}
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
	side := strings.TrimPrefix(strings.TrimPrefix(placement, "floating_"), "square_")
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
		raw.WriteString("<" + prefix + `positionH relativeFrom="column">`)
		if offset != nil {
			raw.WriteString("<" + prefix + "posOffset>" + strconv.Itoa(*offset) + "</" + prefix + "posOffset>")
		} else {
			raw.WriteString("<" + prefix + "align>" + side + "</" + prefix + "align>")
		}
		raw.WriteString("</" + prefix + "positionH>")
		raw.WriteString("<" + prefix + `positionV relativeFrom="paragraph"><` + prefix + "posOffset>0</" +
			prefix + "posOffset></" + prefix + "positionV>")
	}
	raw.Write(first)
	if strings.HasPrefix(placement, "square_") {
		raw.WriteString("<" + prefix + "wrapSquare wrapText=\"" + squareWrapTextSide(side) + "\"/>")
	} else if kind == "anchor" {
		raw.WriteString("<" + prefix + "wrapTopAndBottom/>")
	}
	raw.Write(rest)
	raw.WriteString("</" + prefix + kind + ">")
	return replacement{frame.start, frame.end, []byte(raw.String())}, nil
}
