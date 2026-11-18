package docx

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

const figureSequence = `SEQ Figure \* ARABIC`

func isFigureSequence(instruction string) bool {
	fields := strings.Fields(instruction)
	return len(fields) >= 2 && strings.EqualFold(fields[0], "SEQ") && strings.EqualFold(fields[1], "Figure")
}

func supportedFigureSequence(instruction string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(instruction), " "), figureSequence)
}

func captionFollowingPicture(t target) (*node, bool) {
	p := t.paragraph
	if t.kind != "picture" || p == nil || !p.parent.is("body") || t.run == nil || t.run.parent != p ||
		p.has("sectPr") || p.has("br") || p.has("bookmarkStart") || p.has("bookmarkEnd") ||
		p.has("commentRangeStart") || p.has("commentRangeEnd") {
		return nil, false
	}
	for _, child := range p.children {
		if child == t.run || child.is("pPr") {
			continue
		}
		if !child.is("r") || child.has("drawing") || child.has("pict") || child.has("object") {
			return nil, false
		}
	}
	for i, child := range p.parent.children {
		if child == p && i+1 < len(p.parent.children) {
			return p.parent.children[i+1], true
		}
	}
	return nil, true
}

func captionParagraph(t target) bool {
	next, eligible := captionFollowingPicture(t)
	if !eligible || next == nil || !next.is("p") {
		return eligible
	}
	if strings.EqualFold(next.child("pPr").child("pStyle").attr("val"), "Caption") {
		return false
	}
	for _, field := range next.children {
		if field.is("fldSimple") && isFigureSequence(field.attr("instr")) {
			return false
		}
	}
	return true
}

func editablePictureCaption(t target, data []byte) (*node, string) {
	next, eligible := captionFollowingPicture(t)
	if !eligible {
		return nil, ""
	}
	return editableCaptionParagraph(next, data)
}

func editableCaptionParagraph(next *node, data []byte) (*node, string) {
	if next == nil || !next.is("p") || next.has("sectPr") ||
		strings.Contains(string(data[next.start:next.end]), "<!--") {
		return nil, ""
	}
	children := next.children
	if len(children) > 0 && children[0].is("pPr") {
		children = children[1:]
	}
	if len(children) != 3 || !children[1].is("fldSimple") || !supportedFigureSequence(children[1].attr("instr")) {
		return nil, ""
	}
	_, prefix := plainTextRunNodes(children[:1])
	nodes, label := plainTextRunNodes(children[2:])
	if len(nodes) != 1 || prefix != "그림 " || !strings.HasPrefix(label, ". ") || len(label) <= 2 || len(label) > 1026 || !validText(label) {
		return nil, ""
	}
	return nodes[0], strings.TrimPrefix(label, ". ")
}

func documentFigureFields(body *node) ([]*node, error) {
	var values []*node
	var walk func(*node) error
	walk = func(n *node) error {
		if n.is("fldChar") || n.is("instrText") || n.is("delInstrText") {
			return fmt.Errorf("DOCX figure numbering cannot be derived with complex body fields")
		}
		if n.is("fldSimple") && isFigureSequence(n.attr("instr")) {
			if !supportedFigureSequence(n.attr("instr")) || len(n.children) != 1 || !n.children[0].is("r") || !n.parent.is("p") || !n.parent.parent.is("body") ||
				strings.Contains(string(body.parent.source[n.start:n.end]), "<!--") {
				return fmt.Errorf("protected DOCX figure sequence")
			}
			for _, attr := range n.attrs {
				if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || attr.Name.Local != "instr" && attr.Name.Local != "dirty") {
					return fmt.Errorf("protected DOCX figure sequence")
				}
			}
			nodes, label := plainTextRunNodes(n.children)
			number, err := strconv.ParseInt(label, 10, 32)
			if len(nodes) != 1 || err != nil || number < 1 {
				return fmt.Errorf("invalid DOCX figure sequence result")
			}
			values = append(values, nodes[0])
		}
		for _, child := range n.children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	err := walk(body)
	return values, err
}

func (p *documentPackage) pictureCaption(t target, text string) ([]replacement, error) {
	if !captionParagraph(t) || !validText(text) || strings.TrimSpace(text) == "" || len(text) > 1024 {
		return nil, fmt.Errorf("DOCX caption requires plain text and an uncaptioned body picture")
	}
	info, relationship := inlinePictureInfo(t.run)
	if info == nil {
		return nil, fmt.Errorf("DOCX caption requires an inline picture")
	}
	if _, _, err := imageConfig(p.partData(p.picturePartMap()[relationship], nil), maxImageReadBytes); err != nil {
		return nil, err
	}
	values, err := documentFigureFields(t.paragraph.parent)
	if err != nil {
		return nil, err
	}
	at := t.paragraph.end
	number := 1
	var edits []replacement
	for i, value := range values {
		if value.start < at {
			number++
			continue
		}
		next := strconv.Itoa(i + 2)
		if value.text.String() != next {
			edit, err := replaceTextNode(p.document, value, next)
			if err != nil {
				return nil, err
			}
			edits = append(edits, edit)
		}
	}
	xml := `<w:p xmlns:w="` + wordNS + `"><w:r>` + inlineXML("그림 ") + `</w:r><w:fldSimple w:instr="` + figureSequence +
		`" w:dirty="true"><w:r>` + inlineXML(strconv.Itoa(number)) + `</w:r></w:fldSimple><w:r>` +
		inlineXML(". "+strings.TrimSpace(text)) + `</w:r></w:p>`
	return append(edits, replacement{start: at, end: at, data: []byte(xml)}), nil
}
func (p *documentPackage) editPictureCaption(t target, expected, text string) (replacement, error) {
	node, current := editablePictureCaption(t, p.document)
	if node == nil || current != expected || !validText(text) || strings.TrimSpace(text) == "" || len(text) > 1024 {
		return replacement{}, fmt.Errorf("DOCX caption requires the current plain numbered picture caption")
	}
	return replaceTextNode(p.document, node, ". "+strings.TrimSpace(text))
}

// Moving only across a plain sibling keeps the figure sequence in order and
// leaves the drawing, caption and surrounding package parts untouched.
func markCaptionedPictureMoves(root *node, paragraphs map[*node]*Paragraph, protected map[*node]bool, targets map[string]target) {
	body := root.child("body")
	if !body.has("fldSimple") || !body.has("drawing") {
		return
	}
	ranged, valid := bodyRangeBlocks(body)
	if !valid {
		return
	}
	data := root.source
	safe := func(p *node) bool {
		paragraph := paragraphs[p]
		return paragraph != nil && !protected[p] && !ranged[p] && !paragraph.PageBreakBefore &&
			!paragraph.PageBreakAfter && plainVisualParagraphProperties(p)
	}
	for i := 0; i+1 < len(body.children); i++ {
		picture, caption := body.children[i], body.children[i+1]
		if !safe(picture) || !picture.has("drawing") || !safe(caption) || !whitespaceBetween(data, picture, caption) {
			continue
		}
		label, _ := editableCaptionParagraph(caption, data)
		if label == nil || strings.TrimSpace(caption.text.String()) != "" || caption.has("rPrChange") {
			continue
		}
		for _, run := range paragraphs[picture].Runs {
			t := targets[run.ID]
			if t.kind != "picture" || protected[t.run] || !dedicatedPictureParagraph(t, data) {
				continue
			}
			if i > 0 {
				before := body.children[i-1]
				if safe(before) && paragraphs[before].StructureEditable && whitespaceBetween(data, before, picture) {
					t.moveUp = before
				}
			}
			if i+2 < len(body.children) {
				after := body.children[i+2]
				if safe(after) && paragraphs[after].StructureEditable && whitespaceBetween(data, caption, after) {
					t.moveDown = after
				}
			}
			targets[run.ID] = t
		}
	}
}

func (p *documentPackage) movePictureCaption(t target, operation string) (replacement, error) {
	other := t.moveDown
	if operation == "move_picture_caption_up" {
		other = t.moveUp
	}
	if t.kind != "picture" || other == nil {
		return replacement{}, fmt.Errorf("protected DOCX picture caption movement boundary")
	}
	inline, relationship := inlinePictureInfo(t.run)
	if inline == nil {
		return replacement{}, fmt.Errorf("DOCX caption movement requires an inline picture")
	}
	if _, _, err := imageConfig(p.partData(p.picturePartMap()[relationship], nil), maxImageReadBytes); err != nil {
		return replacement{}, err
	}
	caption, eligible := captionFollowingPicture(t)
	if !eligible || caption == nil {
		return replacement{}, fmt.Errorf("DOCX picture requires an adjacent caption")
	}
	picture := t.paragraph
	data := p.document
	pair := data[picture.start:caption.end]
	if operation == "move_picture_caption_up" {
		return replacement{other.start, caption.end, bytes.Join([][]byte{
			pair, data[other.end:picture.start], data[other.start:other.end],
		}, nil)}, nil
	}
	return replacement{picture.start, other.end, bytes.Join([][]byte{
		data[other.start:other.end], data[caption.end:other.start], pair,
	}, nil)}, nil
}
