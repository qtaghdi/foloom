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
			} else {
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
	for _, a := range n.attrs {
		if a.Name.Space == wordNS && a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}
func textContent(n *node) string {
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
	node       *node
	text, kind string
}

func inspect(data []byte, sessionID string, revision uint64) ([]Block, map[string]target, bool, error) {
	root, err := parseXML(data)
	if err != nil {
		return nil, nil, false, err
	}
	if !root.is("document") || root.child("body") == nil {
		return nil, nil, false, fmt.Errorf("DOCX requires a transitional Word document body")
	}
	body := root.child("body")
	protected := map[*node]bool{}
	fieldDepth := 0
	var markFields func(*node)
	markFields = func(n *node) {
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
	targets := map[string]target{}
	blocks := []Block{}
	unsupported := false
	counter := 0
	id := func() string {
		counter++
		return sessionID + ":" + strconv.FormatUint(revision, 10) + ":" + strconv.Itoa(counter)
	}
	var paragraph func(*node, bool, string) Paragraph
	paragraph = func(p *node, allowed bool, kind string) Paragraph {
		out := Paragraph{ID: id(), Runs: []Run{}}
		// Fields and tracked changes can span runs. Fail closed for the paragraph.
		for _, name := range []string{"fldChar", "instrText", "fldSimple", "ins", "del", "moveFrom", "moveTo", "sdt", "rPrChange", "pPrChange"} {
			if p.has(name) {
				allowed = false
			}
		}
		for _, r := range p.children {
			if r.is("pPr") {
				continue
			}
			if !r.is("r") {
				unsupported = true
				out.Runs = append(out.Runs, Run{ID: id(), Text: textContent(r)})
				continue
			}
			texts := []*node{}
			plain := true
			for _, c := range r.children {
				if c.is("t") {
					texts = append(texts, c)
				} else if !c.is("rPr") {
					plain = false
				}
			}
			editable := allowed && !protected[r] && plain && len(texts) == 1 && len(texts[0].children) == 0
			text := textContent(r)
			if !validText(text) {
				editable = false
			}
			run := Run{ID: id(), Text: text, Editable: editable}
			if props := r.child("rPr"); props != nil {
				if b := props.child("b"); b != nil {
					run.Bold = b.attr("val") != "0" && b.attr("val") != "false" && b.attr("val") != "off"
				}
				if i := props.child("i"); i != nil {
					run.Italic = i.attr("val") != "0" && i.attr("val") != "false" && i.attr("val") != "off"
				}
			}
			out.Runs = append(out.Runs, run)
			if editable {
				targets[run.ID] = target{node: texts[0], text: text, kind: kind}
			} else {
				unsupported = true
			}
		}
		return out
	}
	for _, child := range body.children {
		switch {
		case child.is("p"):
			p := paragraph(child, true, "replace_run_text")
			blocks = append(blocks, Block{ID: p.ID, Type: "paragraph", Paragraph: &p})
		case child.is("tbl"):
			block := Block{ID: id(), Type: "table", Rows: []Row{}}
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
				r := Row{Cells: []Cell{}}
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
	return blocks, targets, unsupported, nil
}
