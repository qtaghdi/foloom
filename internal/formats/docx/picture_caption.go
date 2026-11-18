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
	if t.kind != "picture" || p == nil || (!p.parent.is("body") && !t.pictureCellCaption) || t.run == nil || t.run.parent != p ||
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

func editableCaptionParagraph(paragraph *node, data []byte) (*node, string) {
	caption := readFigureCaption(paragraph, data)
	return caption.label, caption.text
}

type figureCaption struct {
	number, label, bookmark *node
	text                    string
}

func readFigureCaption(p *node, data []byte) figureCaption {
	if p == nil || !p.is("p") || pictureCaptionBody(p) == nil || p.has("sectPr") ||
		bytes.Contains(data[p.start:p.end], []byte("<!--")) {
		return figureCaption{}
	}
	children := p.children
	if len(children) > 0 && children[0].is("pPr") {
		children = children[1:]
	}
	var field, bookmark *node
	switch {
	case len(children) == 3:
		field = children[1]
	case len(children) == 5 && isFigureBookmarkPair(children[1], children[3]):
		bookmark, field = children[1], children[2]
	default:
		return figureCaption{}
	}
	number := captionSequenceValue(field, data, supportedFigureSequence)
	_, prefix := plainTextRunNodes(children[:1])
	labels, text := plainTextRunNodes(children[len(children)-1:])
	if number == nil || len(labels) != 1 || prefix != "그림 " || !strings.HasPrefix(text, ". ") ||
		len(text) <= 2 || len(text) > 1026 || !validText(text) {
		return figureCaption{}
	}
	return figureCaption{number: number, label: labels[0], bookmark: bookmark, text: strings.TrimPrefix(text, ". ")}
}

func captionSequenceValue(field *node, data []byte, supported func(string) bool) *node {
	if !field.is("fldSimple") || !supported(field.attr("instr")) || len(field.children) != 1 ||
		!field.parent.is("p") || bytes.Contains(data[field.start:field.end], []byte("<!--")) {
		return nil
	}
	for _, attr := range field.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || attr.Name.Local != "instr" && attr.Name.Local != "dirty") {
			return nil
		}
	}
	nodes, text := plainTextRunNodes(field.children)
	number, err := strconv.ParseInt(text, 10, 32)
	if len(nodes) != 1 || err != nil || number < 1 {
		return nil
	}
	return nodes[0]
}

func documentFigureFields(body *node) ([]*node, error) {
	return documentCaptionFields(body, "figure", isFigureSequence, supportedFigureSequence, true)
}

// Cell figures share the document body sequence in XML order.
func pictureCaptionBody(p *node) *node {
	parent := p.parent
	if parent.is("body") {
		return parent
	}
	if parent.is("tc") && parent.parent.is("tr") && parent.parent.parent.is("tbl") &&
		parent.parent.parent.parent.is("body") {
		return parent.parent.parent.parent
	}
	return nil
}

func documentCaptionFields(body *node, label string, isSequence, supported func(string) bool, allowCells bool) ([]*node, error) {
	var values []*node
	var ranged map[*node]bool
	safeTables := map[*node]bool{}
	var walk func(*node) error
	walk = func(n *node) error {
		if n.is("fldChar") || n.is("instrText") || n.is("delInstrText") {
			return fmt.Errorf("DOCX %s numbering cannot be derived with complex body fields", label)
		}
		if n.is("fldSimple") && isSequence(n.attr("instr")) {
			value := captionSequenceValue(n, body.parent.source, supported)
			if value == nil {
				return fmt.Errorf("protected or invalid DOCX %s sequence", label)
			}
			paragraph := n.parent
			if paragraph.parent != body {
				if !allowCells || pictureCaptionBody(paragraph) != body {
					return fmt.Errorf("protected DOCX %s sequence outside the body", label)
				}
				if ranged == nil {
					var valid bool
					ranged, valid = bodyRangeBlocks(body)
					if !valid || body.has("permStart") || body.has("permEnd") {
						return fmt.Errorf("protected DOCX numbering ranges")
					}
				}
				table := paragraph.parent.parent.parent
				safe, checked := safeTables[table]
				if !checked {
					_, safe = simpleTable(table)
					safeTables[table] = safe && !ranged[table]
				}
				caption := readFigureCaption(paragraph, body.parent.source)
				if !safeTables[table] || caption.number != value || caption.bookmark != nil {
					return fmt.Errorf("protected DOCX figure sequence in a table cell")
				}
			}
			values = append(values, value)
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
		return nil, fmt.Errorf("DOCX caption requires plain text and an uncaptioned picture")
	}
	info, relationship := inlinePictureInfo(t.run)
	if info == nil {
		return nil, fmt.Errorf("DOCX caption requires an inline picture")
	}
	if _, _, err := imageConfig(p.partData(p.picturePartMap()[relationship], nil), maxImageReadBytes); err != nil {
		return nil, err
	}
	body := pictureCaptionBody(t.paragraph)
	values, err := documentFigureFields(body)
	if err != nil {
		return nil, err
	}
	edits, err := numberedCaptionEdits(p.document, values, t.paragraph.end, figureSequence, "그림 ", text, false)
	if err != nil {
		return nil, err
	}
	refs, err := figureReferenceNumberEdits(body, p.document, values, t.paragraph.end)
	if err != nil {
		return nil, err
	}
	return append(edits, refs...), nil
}

func numberedCaptionEdits(data []byte, values []*node, at int, instruction, prefix, text string, keepNext bool) ([]replacement, error) {
	number := 1
	var edits []replacement
	for i, value := range values {
		if value.start < at {
			number++
			continue
		}
		next := strconv.Itoa(i + 2)
		if value.text.String() != next {
			edit, err := replaceTextNode(data, value, next)
			if err != nil {
				return nil, err
			}
			edits = append(edits, edit)
		}
	}
	properties := ""
	if keepNext {
		properties = `<w:pPr><w:keepNext/></w:pPr>`
	}
	xml := `<w:p xmlns:w="` + wordNS + `">` + properties + `<w:r>` + inlineXML(prefix) + `</w:r><w:fldSimple w:instr="` + instruction +
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
	ranged, valid := bodyRangeBlocksExcept(body, figureBookmarkMarkers(root))
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
