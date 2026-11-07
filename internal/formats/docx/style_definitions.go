package docx

import (
	"archive/zip"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

func directInlineStyle(props *node) *InlineStyle {
	if props == nil {
		return nil
	}
	r := Run{}
	directRun(&r, props)
	s := &InlineStyle{Color: r.Color, Font: r.Font, Highlight: r.Highlight, Vertical: r.Vertical}
	if props.child("b") != nil {
		s.Bold = &r.Bold
	}
	if props.child("i") != nil {
		s.Italic = &r.Italic
	}
	if props.child("u") != nil {
		s.Underline = &r.Underline
	}
	if props.child("strike") != nil || props.child("dstrike") != nil {
		s.Strike = &r.Strike
	}
	if r.SizeHalfPoints != 0 {
		s.SizeHalfPoints = &r.SizeHalfPoints
	}
	return s
}
func directParagraphFormat(props *node) *ParagraphFormat {
	if props == nil {
		return nil
	}
	l := ParagraphLayout{}
	applyParagraphPreview(&l, props)
	f := &ParagraphFormat{Align: l.Align, Left: l.Left, Right: l.Right, FirstLine: l.FirstLine, Hanging: l.Hanging, Before: l.Before, After: l.After, Line: l.Line, LineRule: l.LineRule, TextDirection: l.TextDirection}
	if props.child("bidi") != nil {
		f.Bidi = &l.Bidi
	}
	return f
}
func styleText(value string, empty bool) bool {
	return (empty || strings.TrimSpace(value) != "") && len(value) <= 120 && utf8.ValidString(value) && strings.IndexFunc(value, unicode.IsControl) < 0
}
func editableStyle(n *node, data []byte) bool {
	if strings.Contains(string(data[n.start:n.end]), "<!--") {
		return false
	}
	for _, name := range []string{"name", "basedOn", "pPr", "rPr"} {
		count := 0
		for _, c := range n.children {
			if c.is(name) {
				count++
			}
		}
		if count > 1 {
			return false
		}
	}
	var safe func(*node) bool
	safe = func(n *node) bool {
		if n == nil {
			return true
		}
		if n.name.Space != wordNS || strings.TrimSpace(n.text.String()) != "" {
			return false
		}
		for _, a := range n.attrs {
			if a.Name.Local == "id" && a.Name.Space != "xmlns" {
				return false
			}
		}
		for _, c := range n.children {
			if c.is("pPrChange") || c.is("rPrChange") || !safe(c) {
				return false
			}
		}
		return true
	}
	return safe(n.child("pPr")) && safe(n.child("rPr"))
}
func (p *documentPackage) styleCatalog() []StyleInfo {
	result := []StyleInfo{}
	ids := []string{}
	for id := range p.styles.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !styleText(id, false) {
			continue
		}
		n := p.styles.byID[id]
		kind := n.attr("type")
		if kind != "paragraph" && kind != "character" {
			continue
		}
		name := n.child("name").attr("val")
		if name == "" {
			name = id
		}
		if len([]rune(name)) > 120 {
			name = string([]rune(name)[:120]) + "…"
		}
		result = append(result, StyleInfo{ID: id, Name: name, Kind: kind, BasedOn: n.child("basedOn").attr("val"), Editable: editableStyle(n, p.styleData()), Run: directInlineStyle(n.child("rPr")), Paragraph: directParagraphFormat(n.child("pPr"))})
		if len(result) >= 500 {
			break
		}
	}
	return result
}
func (p *documentPackage) styleData() []byte {
	for _, part := range p.parts {
		if part.header.Name == p.stylesPath {
			return part.data
		}
	}
	return nil
}
func patchStyle(root *node, data []byte, d StyleDefinition) ([]byte, error) {
	if !root.is("styles") {
		return nil, fmt.Errorf("unsupported DOCX styles root")
	}
	if !styleText(d.ID, false) || d.Name != "" && !styleText(d.Name, false) || (d.Kind != "paragraph" && d.Kind != "character") {
		return nil, fmt.Errorf("invalid DOCX style identity, name or kind")
	}
	if d.BasedOn != nil && !styleText(*d.BasedOn, true) {
		return nil, fmt.Errorf("invalid DOCX style base")
	}
	if d.Run != nil && d.Run.StyleID != nil || d.Paragraph != nil && d.Paragraph.StyleID != nil {
		return nil, fmt.Errorf("style definitions use basedOn rather than style references")
	}
	if err := validateInlineStyle(d.Run); err != nil {
		return nil, err
	}
	if err := validateParagraphFormat(d.Paragraph); err != nil {
		return nil, err
	}
	if d.Kind == "character" && d.Paragraph != nil {
		return nil, fmt.Errorf("character styles cannot contain paragraph formatting")
	}
	var old *node
	for _, n := range root.children {
		if n.is("style") && n.attr("styleId") == d.ID {
			if old != nil {
				return nil, fmt.Errorf("duplicate DOCX style identity")
			}
			old = n
		}
	}
	opening, closing := `<dws:style xmlns:dws="`+wordNS+`" dws:type="`+d.Kind+`" dws:styleId="`+escapeText(d.ID)+`">`, `</dws:style>`
	if old != nil {
		if old.attr("type") != d.Kind || !editableStyle(old, data) {
			return nil, fmt.Errorf("DOCX style is protected or has another kind")
		}
		opening, closing = scopedOpening(old, data, false)
	} else if d.Name == "" {
		return nil, fmt.Errorf("new DOCX style requires a name")
	}
	replacements := map[string]string{}
	scalar := func(name, value string) string {
		return `<dws:` + name + ` xmlns:dws="` + wordNS + `" dws:val="` + escapeText(value) + `"/>`
	}
	if d.Name != "" {
		var n *node
		if old != nil {
			n = old.child("name")
		}
		patched, err := patchProperty(n, data, "name", map[string]string{"val": d.Name}, nil)
		if err != nil {
			return nil, err
		}
		replacements["name"] = patched
	}
	if d.BasedOn != nil {
		replacements["basedOn"] = ""
		if *d.BasedOn != "" {
			replacements["basedOn"] = scalar("basedOn", *d.BasedOn)
		}
	}
	if d.Run != nil {
		var n *node
		if old != nil {
			n = old.child("rPr")
		}
		v, err := inlineProperties(n, data, d.Run)
		if err != nil {
			return nil, err
		}
		replacements["rPr"] = v
	}
	if d.Paragraph != nil {
		var n *node
		if old != nil {
			n = old.child("pPr")
		}
		v, err := paragraphProperties(n, data, d.Paragraph)
		if err != nil {
			return nil, err
		}
		replacements["pPr"] = v
	}
	type chunk struct{ name, raw string }
	chunks := []chunk{}
	if old != nil {
		for _, c := range old.children {
			if _, changed := replacements[c.name.Local]; !changed {
				chunks = append(chunks, chunk{c.name.Local, string(data[c.start:c.end])})
			}
		}
	}
	rank := map[string]int{}
	for i, n := range strings.Fields("name aliases basedOn next link autoRedefine hidden uiPriority semiHidden unhideWhenUsed qFormat locked personal personalCompose personalReply rsid pPr rPr tblPr trPr tcPr tblStylePr") {
		rank[n] = i + 1
	}
	names := []string{}
	for name := range replacements {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return rank[names[i]] < rank[names[j]] })
	for _, name := range names {
		item := chunk{name, replacements[name]}
		index := len(chunks)
		for i, old := range chunks {
			if rank[old.name] > rank[item.name] {
				index = i
				break
			}
		}
		chunks = append(chunks, chunk{})
		copy(chunks[index+1:], chunks[index:])
		chunks[index] = item
	}
	var b strings.Builder
	b.WriteString(opening)
	for _, c := range chunks {
		b.WriteString(c.raw)
	}
	b.WriteString(closing)
	if old != nil {
		return append(append(append([]byte(nil), data[:old.start]...), []byte(b.String())...), data[old.end:]...), nil
	}
	if strings.HasSuffix(string(data[root.start:root.inner]), "/>") {
		open, close := scopedOpening(root, data, false)
		return append(append(append([]byte(nil), data[:root.start]...), []byte(open+b.String()+close)...), data[root.end:]...), nil
	}
	return append(append(append([]byte(nil), data[:root.endStart]...), []byte(b.String())...), data[root.endStart:]...), nil
}
func (p *documentPackage) prepareStyles(operations []Operation) (map[string][]byte, previewStyles, error) {
	changes := map[string][]byte{}
	defs := []StyleDefinition{}
	seen := map[string]bool{}
	for _, op := range operations {
		if op.Type != "define_style" {
			if op.StyleDefinition != nil {
				return nil, previewStyles{}, fmt.Errorf("style definition requires define_style")
			}
			continue
		}
		if op.StyleDefinition == nil || op.TargetID != "" || op.EndTargetID != "" || op.Text != "" || op.ExpectedText != "" || op.Paragraphs != nil || seen[op.StyleDefinition.ID] {
			return nil, previewStyles{}, fmt.Errorf("invalid or duplicate DOCX style definition")
		}
		seen[op.StyleDefinition.ID] = true
		defs = append(defs, *op.StyleDefinition)
	}
	if len(defs) == 0 {
		return changes, p.styles, nil
	}
	if len(defs) > 32 {
		return nil, previewStyles{}, fmt.Errorf("DOCX batch permits at most 32 style definitions")
	}
	filename, data := p.stylesPath, p.styleData()
	if filename == "" {
		var err error
		filename, err = p.addWordPart(changes, "styles", "rIdDocxStyles")
		if err != nil {
			return nil, previewStyles{}, err
		}
		data = []byte(`<w:styles xmlns:w="` + wordNS + `"></w:styles>`)
	}

	for _, d := range defs {
		root, err := parseXML(data)
		if err != nil {
			return nil, previewStyles{}, err
		}
		data, err = patchStyle(root, data, d)
		if err != nil {
			return nil, previewStyles{}, err
		}
	}
	if len(data) > maxPartBytes {
		return nil, previewStyles{}, fmt.Errorf("DOCX styles part exceeds limit")
	}
	root, err := parseXML(data)
	if err != nil {
		return nil, previewStyles{}, err
	}
	styles := readPreviewStyles(root)
	// Validate touched inheritance, including existing descendants affected by edits.
	for id, n := range styles.byID {
		visited := map[string]bool{}
		kind := n.attr("type")
		touches := false
		current := id
		chainErr := false
		for count := 0; current != ""; count++ {
			if seen[current] {
				touches = true
			}
			next := styles.byID[current]
			if count >= 32 || visited[current] || next == nil || next.attr("type") != kind {
				chainErr = true
				break
			}
			visited[current] = true
			current = next.child("basedOn").attr("val")
		}
		if touches && chainErr {
			return nil, previewStyles{}, fmt.Errorf("invalid, cyclic or excessive DOCX style inheritance")
		}
	}
	changes[filename] = data
	return changes, styles, nil
}

// serializeParts preserves original ZIP headers and untouched part payloads.
func (p *documentPackage) serializeParts(document []byte, changes map[string][]byte) ([]byte, error) {
	if len(changes) == 0 {
		return p.serialize(document)
	}
	clone := *p
	clone.parts = append([]part(nil), p.parts...)
	seen := map[string]bool{}
	for i, part := range clone.parts {
		seen[part.header.Name] = true
		if data, ok := changes[part.header.Name]; ok {
			clone.parts[i].data = data
		}
	}
	names := []string{}
	for name := range changes {
		if !seen[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		clone.parts = append(clone.parts, part{header: zip.FileHeader{Name: name, Method: zip.Deflate}, data: changes[name]})
	}
	return clone.serialize(document)
}

func validateStyleReference(styles previewStyles, id *string, kind string) error {
	if id == nil || *id == "" {
		return nil
	}
	n := styles.byID[*id]
	if n == nil || n.attr("type") != kind {
		return fmt.Errorf("unknown or mismatched DOCX style reference")
	}
	return nil
}
