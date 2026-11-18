package docx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

type node struct {
	parent                      *node
	source                      []byte
	name                        xml.Name
	attrs                       []xml.Attr
	start, inner, endStart, end int
	children                    []*node
	text                        strings.Builder
}

func parseXML(data []byte) (*node, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var root *node
	var stack []*node
	count := 0
	tokens := 0
	for {
		start := int(decoder.InputOffset())
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid OOXML: %w", err)
		}
		tokens++
		if tokens > 500000 {
			return nil, fmt.Errorf("OOXML token limit exceeded")
		}
		switch t := token.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("XML directives are unsupported")
		case xml.ProcInst:
			if t.Target != "xml" || root != nil {
				return nil, fmt.Errorf("unsupported XML processing instruction")
			}
		case xml.StartElement:
			count++
			if len(stack) >= 64 || count > 100000 {
				return nil, fmt.Errorf("OOXML structure limit exceeded")
			}
			seenAttrs := map[xml.Name]bool{}
			for _, a := range t.Attr {
				if seenAttrs[a.Name] {
					return nil, fmt.Errorf("duplicate XML attribute")
				}
				seenAttrs[a.Name] = true
			}
			n := &node{name: t.Name, attrs: t.Attr, start: start, inner: int(decoder.InputOffset())}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("multiple XML roots")
				}
				root = n
				root.source = data
			} else {
				n.parent = stack[len(stack)-1]
				stack[len(stack)-1].children = append(stack[len(stack)-1].children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			n := stack[len(stack)-1]
			n.endStart, n.end = start, int(decoder.InputOffset())
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				_, _ = stack[len(stack)-1].text.Write(t)
			} else if strings.TrimSpace(string(t)) != "" {
				return nil, fmt.Errorf("text outside XML root")
			}
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, fmt.Errorf("incomplete XML")
	}
	return root, nil
}

func (n *node) is(local string) bool { return n.name.Space == wordNS && n.name.Local == local }
func (n *node) child(local string) *node {
	if n == nil {
		return nil
	}
	for _, c := range n.children {
		if c.is(local) {
			return c
		}
	}
	return nil
}
func (n *node) has(local string) bool {
	if n.is(local) {
		return true
	}
	for _, c := range n.children {
		if c.has(local) {
			return true
		}
	}
	return false
}
func (n *node) attr(local string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.attrs {
		if a.Name.Space == wordNS && a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}
func textContent(n *node) string {
	if n.is("tab") {
		if n.parent != nil && n.parent.is("r") {
			return "\t"
		}
		return ""
	}
	if n.is("br") && (n.attr("type") == "" || n.attr("type") == "textWrapping") {
		return "\n"
	}
	if n.is("t") {
		return n.text.String()
	}
	var result strings.Builder
	for _, c := range n.children {
		result.WriteString(textContent(c))
	}
	return result.String()
}
func validText(s string) bool {
	if !utf8.ValidString(s) || len(s) > maxTextBytes || strings.ContainsAny(s, "\r\n\t") {
		return false
	}
	for _, r := range s {
		if !(r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF) {
			return false
		}
	}
	return true
}
func escapeText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

type target struct {
	trackable        bool
	textStories      [][]*node
	moveUp, moveDown *node
	paragraph, run   *node
	node             *node
	text, kind       string
}

func inspect(data []byte, sessionID string, revision uint64) ([]Block, map[string]target, bool, error) {
	root, err := parseXML(data)
	if err != nil {
		return nil, nil, false, err
	}
	return inspectDocument(root, sessionID, revision, previewStyles{})
}

func inspectDocument(root *node, sessionID string, revision uint64, styles previewStyles) ([]Block, map[string]target, bool, error) {
	if !root.is("document") || root.child("body") == nil {
		return nil, nil, false, fmt.Errorf("DOCX requires a transitional Word document body")
	}
	body := root.child("body")
	boxesLinked := bytes.Contains(root.source, []byte("linkedTxbx")) && hasLinkedTextBox(root)
	protected := map[*node]bool{}
	fieldDepth := 0
	var markFields func(*node)
	markFields = func(n *node) {
		if n.is("p") && fieldDepth > 0 {
			protected[n] = true
		}
		if n.is("fldChar") {
			switch n.attr("fldCharType") {
			case "begin":
				fieldDepth++
			case "end":
				if fieldDepth > 0 {
					fieldDepth--
				}
			default:
				if fieldDepth == 0 {
					fieldDepth = 1
				}
			}
		}
		if n.is("r") && fieldDepth > 0 {
			protected[n] = true
		}
		for _, c := range n.children {
			markFields(c)
		}
	}
	markFields(body)
	anchors := paragraphAnchors(root)
	targets := map[string]target{}
	blocks := []Block{}
	bodyParagraphs := map[*node]*Paragraph{}
	unsupported := false
	counter := 0
	id := func() string {
		counter++
		return sessionID + ":" + strconv.FormatUint(revision, 10) + ":" + strconv.Itoa(counter)
	}
	var paragraph func(*node, bool, string) Paragraph
	paragraph = func(p *node, allowed bool, kind string) Paragraph {
		allowed = allowed && !protected[p]
		if strings.TrimSpace(p.text.String()) != "" {
			allowed = false
			unsupported = true
		}
		out := Paragraph{ID: id(), Anchor: anchors[p], Runs: []Run{}}
		var trailingBreak *node
		if kind == "replace_run_text" {
			trailingBreak = trailingPageBreak(p)
			out.PageBreakAfter = trailingBreak != nil
		}
		props := p.child("pPr")
		styleID := props.child("pStyle").attr("val")
		if styleText(styleID, true) {
			out.StyleID = styleID
		}
		if style := styles.byID[styleID]; style == nil || style.attr("type") != "paragraph" {
			styleID = styles.paragraph
		}
		layout := ParagraphLayout{}
		var baseRun Run
		directRun(&baseRun, styles.defaults.child("rPrDefault").child("rPr"))
		applyParagraph := func(props *node) {
			if n := props.child("numPr"); n != nil {
				if id := listInt(n.child("numId").attr("val"), -1); id >= 0 {
					level := listInt(n.child("ilvl").attr("val"), 0)
					if level <= 8 {
						out.List = &ParagraphList{ID: id, Level: level}
					}
				}
			}
			applyParagraphPreview(&layout, props)
			if level := paragraphOutlineLevel(props); level != nil {
				out.OutlineLevel = level
			}
			if br := props.child("pageBreakBefore"); br != nil {
				out.PageBreakBefore = onOff(br.attr("val"), true)
			}
		}
		applyParagraph(styles.defaults.child("pPrDefault").child("pPr"))
		chain := styles.chain(styleID, "paragraph")
		for i := len(chain) - 1; i >= 0; i-- {
			applyParagraph(chain[i].child("pPr"))
			styleRun(&baseRun, chain[i].child("rPr"))
		}
		applyParagraph(props)
		out.OutlineOverride = paragraphOutlineLevel(props)
		if hasParagraphLayout(layout) {
			out.Layout = &layout
		}
		// Fields and tracked changes can span runs. Fail closed for the paragraph.
		for _, name := range []string{"fldChar", "instrText", "fldSimple", "ins", "del", "moveFrom", "moveTo", "sdt", "rPrChange", "pPrChange"} {
			if p.has(name) {
				allowed = false
			}
		}
		reviewable := kind == "replace_run_text" && p.parent == body && !protected[p]
		if reviewable && (p.has("ins") || p.has("del")) {
			for _, name := range []string{"fldChar", "instrText", "delInstrText", "fldSimple", "moveFrom", "moveTo", "sdt",
				"pPrChange", "rPrChange", "commentRangeStart", "commentRangeEnd", "bookmarkStart", "bookmarkEnd"} {
				if p.has(name) {
					reviewable = false
					break
				}
			}
			if props := p.child("pPr"); props != nil && (props.has("ins") || props.has("del")) {
				reviewable = false
			}
		}
		trackableParagraph := allowed && reviewable
		for _, name := range []string{"commentRangeStart", "commentRangeEnd", "commentReference", "bookmarkStart", "bookmarkEnd"} {
			if p.has(name) {
				trackableParagraph = false
				break
			}
		}
		for _, r := range p.children {
			if r.is("pPr") {
				continue
			}
			if !r.is("r") {
				if r.is("bookmarkStart") || r.is("bookmarkEnd") || r.is("commentRangeStart") || r.is("commentRangeEnd") {
					continue
				}
				run := Run{ID: id(), Text: textContent(r), Object: objectPreview(r)}
				if run.Object != nil && run.Object.Kind == "math" && run.Object.Editable && allowed && !protected[r] && !strings.Contains(string(root.source[r.start:r.end]), "<!--") {
					targets[run.ID] = target{node: r, paragraph: p, kind: "math_object"}
				}
				if run.Object != nil && run.Object.Editable && targets[run.ID].kind != "math_object" {
					run.Object.Editable = false
				}
				if r.is("ins") || r.is("del") {
					if change := reviewTextRevision(r, root.source); change != nil {
						run.Object = change
						change.Editable = reviewable
						if reviewable {
							targets[run.ID] = target{node: r, paragraph: p, kind: "tracked_text"}
						}
					}
				}
				if r.is("sdt") && reviewable && p.parent == body && !p.has("ins") && !p.has("del") &&
					!p.has("fldSimple") && !p.has("fldChar") && !p.has("moveFrom") && !p.has("moveTo") {
					if texts, value := plainControlText(r, root.source); len(texts) != 0 {
						run.Object = &ObjectInfo{Kind: "control", Text: value, Label: reviewText(r.child("sdtPr").child("alias").attr("val"), 160), Editable: true}
						targets[run.ID] = target{node: r, text: value, paragraph: p, kind: "update_control_text", textStories: [][]*node{texts}}
					}
				}
				if r.is("hyperlink") {
					targets[run.ID] = target{node: r, paragraph: p, kind: "hyperlink"}
				} else if name := bookmarkFieldName(r); name != "" {
					targets[run.ID] = target{node: r, paragraph: p, kind: "bookmark_ref", text: name}
				} else {
					unsupported = true
				}
				out.Runs = append(out.Runs, run)
				continue
			}
			if r.child("commentReference") != nil && len(r.children) == 1 {
				continue
			}
			texts := []*node{}
			plain := strings.TrimSpace(r.text.String()) == ""
			for _, c := range r.children {
				if c.is("t") {
					texts = append(texts, c)
				} else if !(c.is("rPr") || plainLineBreak(c) || plainTab(c)) {
					plain = false
				}
			}
			editable := allowed && !protected[r] && plain
			for _, text := range texts {
				editable = editable && len(text.children) == 0
				if len(texts) != 1 || r.has("br") || r.has("tab") {
					editable = editable && plainTextAttributes(text)
				}
			}
			if len(texts) != 1 || r.has("br") || r.has("tab") {
				props := 0
				for _, child := range r.children {
					if child.is("rPr") {
						props++
					}
				}
				if props > 1 || strings.Contains(string(root.source[r.start:r.end]), "<!--") {
					editable = false
				}
			}
			text := textContent(r)
			if !validInlineText(text) {
				editable = false
			}
			run := baseRun
			run.ID, run.Text, run.Editable = id(), text, editable
			if !editable {
				run.Object = objectPreview(r)
				if stories := textBoxStories(r); len(stories) != 0 {
					run.Object = &ObjectInfo{Kind: "textbox", Text: reviewText(textContent(stories[0]), 1024)}
					if allowed && !protected[r] && p.parent == body && !boxesLinked {
						if nodes, text := plainTextBoxStories(stories, root.source); len(nodes) != 0 {
							run.Object.Text, run.Object.Editable = text, true
							targets[run.ID] = target{node: r, paragraph: p, text: text, kind: "update_textbox_text", textStories: nodes}
						}
					}
				}
			}
			if len(texts) == 0 && r.child("br") == trailingBreak && trailingBreak != nil &&
				(len(r.children) == 1 || len(r.children) == 2 && r.child("rPr") != nil) {
				continue
			}
			runProps := r.child("rPr")
			characters := styles.chain(runProps.child("rStyle").attr("val"), "character")
			for i := len(characters) - 1; i >= 0; i-- {
				styleRun(&run, characters[i].child("rPr"))
			}
			base := run
			run.BaseStyle = &InlineStyle{Bold: &base.Bold, Italic: &base.Italic, Underline: &base.Underline, Strike: &base.Strike, Color: base.Color, Font: base.Font, Highlight: base.Highlight, Vertical: base.Vertical}
			if base.SizeHalfPoints != 0 {
				run.BaseStyle.SizeHalfPoints = &base.SizeHalfPoints
			}
			if id := runProps.child("rStyle").attr("val"); styleText(id, true) {
				run.CharacterStyleID = id
			}
			directRun(&run, runProps)
			run.Trackable = trackableParagraph && editable && trackableRun(r, texts, root.source)
			out.Runs = append(out.Runs, run)
			if allowed && !protected[r] && p.parent == body && run.Object != nil && (run.Object.Kind == "footnote" || run.Object.Kind == "endnote") && run.Object.noteID != "" {
				targets[run.ID] = target{node: r, run: r, paragraph: p, kind: run.Object.Kind}
			}
			if !editable && r.child("drawing") != nil && targets[run.ID].kind == "" {
				targets[run.ID] = target{node: r, run: r, paragraph: p, kind: "picture"}
			}
			if editable {
				textNode := r
				if len(texts) == 1 && !r.has("br") || r.has("tab") {
					textNode = texts[0]
				}
				targets[run.ID] = target{node: textNode, text: text, kind: kind, paragraph: p, run: r, trackable: run.Trackable}
			} else {
				unsupported = true
			}
		}
		// A genuinely empty paragraph needs an insertion target even when Word
		// stored no run. Never synthesize one around unsupported child content.
		if allowed && len(out.Runs) == 0 && (len(p.children) == 0 || len(p.children) == 1 && p.child("pPr") != nil) {
			run := baseRun
			run.ID, run.Editable = id(), true
			out.Runs = append(out.Runs, run)
			targets[run.ID] = target{node: p, kind: kind, paragraph: p}
		}
		out.StructureEditable = allowed && structuralParagraph(p, root.source)
		for _, run := range out.Runs {
			out.StructureEditable = out.StructureEditable && run.Editable
		}
		if out.StructureEditable {
			targets[out.ID] = target{node: p, paragraph: p, text: textContent(p), kind: "replace_paragraphs"}
		} else if allowed && p.parent == body && (out.PageBreakBefore || props.child("pageBreakBefore") != nil) {
			targets[out.ID] = target{node: p, paragraph: p, text: textContent(p), kind: "update_boundary"}
		} else if allowed && p.parent == body && props.child("sectPr") != nil {
			targets[out.ID] = target{node: p, paragraph: p, kind: "update_section_boundary"}
		} else if allowed && p.parent == body && out.PageBreakAfter {
			targets[out.ID] = target{node: p, paragraph: p, kind: "update_trailing_boundary"}
		}
		return out
	}
	for _, child := range body.children {
		switch {
		case child.is("p"):
			p := paragraph(child, true, "replace_run_text")
			blocks = append(blocks, Block{ID: p.ID, Type: "paragraph", Paragraph: &p})
			bodyParagraphs[child] = &p
		case child.is("tbl"):
			block := Block{ID: id(), Type: "table", Rows: []Row{}}
			_, block.TableEditable = simpleTable(child)
			if block.TableEditable {
				targets[block.ID] = target{node: child, kind: "update_table"}
			}
			// Merged, nested or revision-marked tables are inspection-only.
			allowed := true
			for _, name := range []string{"gridSpan", "vMerge", "hMerge", "ins", "del", "sdt", "gridBefore", "gridAfter", "tblPrChange", "trPrChange", "tcPrChange"} {
				if child.has(name) {
					allowed = false
				}
			}
			for _, row := range child.children {
				if !row.is("tr") {
					if !row.is("tblPr") && !row.is("tblGrid") {
						allowed = false
					}
					continue
				}
				for _, cell := range row.children {
					if cell.is("tc") && cell.has("tbl") {
						allowed = false
					}
				}
			}
			if !allowed {
				unsupported = true
			}
			for _, row := range child.children {
				if !row.is("tr") {
					continue
				}
				r := Row{Cells: []Cell{}, Height: tableRowHeight(row)}
				for _, cell := range row.children {
					if !cell.is("tc") {
						continue
					}
					c := Cell{ID: id(), Paragraphs: []Paragraph{}}
					for _, p := range cell.children {
						if p.is("p") {
							c.Paragraphs = append(c.Paragraphs, paragraph(p, allowed, "replace_cell_text"))
						} else if !p.is("tcPr") {
							unsupported = true
						}
					}
					r.Cells = append(r.Cells, c)
				}
				block.Rows = append(block.Rows, r)
			}
			blocks = append(blocks, block)
		case child.is("sectPr"):
		default:
			unsupported = true
		}
		if counter > maxRuns {
			return nil, nil, false, fmt.Errorf("DOCX inspection structure limit exceeded")
		}
	}
	markBookmarkParagraphs(root, bodyParagraphs, protected, targets)
	markHyperlinkParagraphs(root, bodyParagraphs, protected, targets)
	return blocks, targets, unsupported, nil
}
