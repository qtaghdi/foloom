package docx

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const numberingRelationship = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering"

var listKey = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

type listLevel struct {
	kind  string
	text  string
	start int
}

type numberingCatalog struct {
	levels             map[int]map[int]listLevel
	maxID, maxAbstract int
}

func listInt(raw string, fallback int) int {
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || n < 0 {
		return fallback
	}
	return int(n)
}

func readListLevel(n *node) listLevel {
	start := listInt(n.child("start").attr("val"), 1)
	return listLevel{kind: n.child("numFmt").attr("val"), text: n.child("lvlText").attr("val"), start: start}
}

func readNumbering(root *node) numberingCatalog {
	c := numberingCatalog{levels: map[int]map[int]listLevel{}, maxAbstract: -1}
	if root == nil || !root.is("numbering") {
		return c
	}
	abstracts := map[int]map[int]listLevel{}
	for _, n := range root.children {
		if !n.is("abstractNum") {
			continue
		}
		id := listInt(n.attr("abstractNumId"), -1)
		if id < 0 {
			continue
		}
		c.maxAbstract = max(c.maxAbstract, id)
		levels := map[int]listLevel{}
		for _, level := range n.children {
			index := listInt(level.attr("ilvl"), -1)
			if level.is("lvl") && index >= 0 && index <= 8 {
				levels[index] = readListLevel(level)
			}
		}
		abstracts[id] = levels
	}
	for _, n := range root.children {
		if !n.is("num") {
			continue
		}
		id := listInt(n.attr("numId"), 0)
		if id <= 0 {
			continue
		}
		c.maxID = max(c.maxID, id)
		levels := map[int]listLevel{}
		for k, v := range abstracts[listInt(n.child("abstractNumId").attr("val"), -1)] {
			levels[k] = v
		}
		for _, override := range n.children {
			index := listInt(override.attr("ilvl"), -1)
			if !override.is("lvlOverride") || index < 0 || index > 8 {
				continue
			}
			level, ok := levels[index]
			if child := override.child("lvl"); child != nil {
				level, ok = readListLevel(child), true
			}
			if ok {
				level.start = listInt(override.child("startOverride").attr("val"), level.start)
				levels[index] = level
			}
		}
		c.levels[id] = levels
	}
	return c
}

// Numbering instances are append-only. Starting a new list never mutates the
// shared instance or abstract definition of an unrelated paragraph.
func (p *documentPackage) prepareNumbering(ops []Operation, changes map[string][]byte) (map[string]int, numberingCatalog, error) {
	var defs []ListDefinition
	keys := map[string]int{}
	for _, op := range ops {
		d := op.ListDefinition
		if op.Type != "define_list" {
			if d != nil {
				return nil, numberingCatalog{}, fmt.Errorf("list definition requires define_list")
			}
			continue
		}
		if d == nil || op.TargetID != "" || op.EndTargetID != "" || op.ExpectedText != "" || op.Text != "" || op.Paragraphs != nil ||
			!listKey.MatchString(d.Key) || d.Kind != "bullet" && d.Kind != "decimal" || d.Start < 0 || d.Start > 9999 || d.Kind == "bullet" && d.Start > 1 {
			return nil, numberingCatalog{}, fmt.Errorf("invalid DOCX list definition")
		}
		defs = append(defs, *d)
	}
	if len(defs) > 32 {
		return nil, numberingCatalog{}, fmt.Errorf("DOCX batch permits at most 32 new lists")
	}
	filename := p.numberingPath
	data := p.partData(filename, changes)
	var root *node
	var err error
	if data != nil {
		root, err = parseXML(data)
		if err != nil || !root.is("numbering") {
			return nil, numberingCatalog{}, fmt.Errorf("invalid DOCX numbering part")
		}
	}
	catalog := readNumbering(root)
	if len(defs) == 0 {
		return keys, catalog, nil
	}
	if root == nil {
		filename, err = p.addWordPart(changes, "numbering", "rIdDocxNumbering")
		if err != nil {
			return nil, numberingCatalog{}, err
		}
		data = []byte(`<w:numbering xmlns:w="` + wordNS + `"></w:numbering>`)
		root, _ = parseXML(data)
	}
	// A new definition must not give a previously dangling body or style
	// reference a meaning. Numbering IDs are shared across stories.
	for _, part := range p.parts {
		if !strings.HasSuffix(part.header.Name, ".xml") || !strings.Contains(string(part.data), "numId") {
			continue
		}
		n, e := parseXML(part.data)
		if e != nil {
			return nil, numberingCatalog{}, e
		}
		var visit func(*node)
		visit = func(n *node) {
			if n.is("numId") {
				catalog.maxID = max(catalog.maxID, listInt(n.attr("val"), 0))
			}
			for _, child := range n.children {
				visit(child)
			}
		}
		visit(n)
	}
	if catalog.maxID > 2147483647-len(defs) || catalog.maxAbstract > 2147483647-len(defs) {
		return nil, numberingCatalog{}, fmt.Errorf("DOCX numbering identifiers exhausted")
	}
	var abstractXML, instanceXML strings.Builder
	for _, d := range defs {
		if keys[d.Key] != 0 {
			return nil, numberingCatalog{}, fmt.Errorf("duplicate DOCX list key")
		}
		catalog.maxID++
		catalog.maxAbstract++
		id, abstract := catalog.maxID, catalog.maxAbstract
		keys[d.Key] = id
		levels := map[int]listLevel{}
		fmt.Fprintf(&abstractXML, `<w:abstractNum xmlns:w="%s" w:abstractNumId="%d"><w:multiLevelType w:val="multilevel"/>`, wordNS, abstract)
		for level := 0; level < 9; level++ {
			label := fmt.Sprintf("%%%d.", level+1)
			if d.Kind == "bullet" {
				label = []string{"•", "◦", "▪"}[level%3]
			}
			start := 1
			if level == 0 {
				start = max(1, d.Start)
			}
			levels[level] = listLevel{kind: d.Kind, text: label, start: start}
			left := 720 + level*360
			fmt.Fprintf(&abstractXML, `<w:lvl w:ilvl="%d"><w:start w:val="%d"/><w:numFmt w:val="%s"/><w:lvlText w:val="%s"/><w:lvlJc w:val="left"/><w:pPr><w:tabs><w:tab w:val="num" w:pos="%d"/></w:tabs><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl>`, level, start, d.Kind, label, left, left)
		}
		abstractXML.WriteString("</w:abstractNum>")
		fmt.Fprintf(&instanceXML, `<w:num xmlns:w="%s" w:numId="%d"><w:abstractNumId w:val="%d"/></w:num>`, wordNS, id, abstract)
		catalog.levels[id] = levels
	}
	firstNum, lastNum := root.endStart, root.endStart
	for _, n := range root.children {
		if n.is("num") || n.is("numIdMacAtCleanup") {
			firstNum = min(firstNum, n.start)
		}
		if n.is("numIdMacAtCleanup") {
			lastNum = min(lastNum, n.start)
		}
	}
	if strings.HasSuffix(string(data[root.start:root.inner]), "/>") {
		data, err = appendXMLChild(data, abstractXML.String()+instanceXML.String())
		if err != nil {
			return nil, numberingCatalog{}, err
		}
	} else {
		data = []byte(string(data[:firstNum]) + abstractXML.String() + string(data[firstNum:lastNum]) + instanceXML.String() + string(data[lastNum:]))
	}
	if len(data) > maxPartBytes {
		return nil, numberingCatalog{}, fmt.Errorf("DOCX numbering part exceeds limit")
	}
	changes[filename] = data
	return keys, catalog, nil
}

func resolveParagraphLists(op Operation, keys map[string]int, catalog numberingCatalog) (Operation, error) {
	var copied bool
	for i, paragraph := range op.Paragraphs {
		if paragraph.Format == nil || paragraph.Format.List == nil {
			continue
		}
		list := *paragraph.Format.List
		if list.Level < 0 || list.Level > 8 || list.ID < 0 || list.Key == "" && list.ID == 0 && list.Level != 0 {
			return op, fmt.Errorf("invalid DOCX list reference")
		}
		if list.Key != "" {
			if list.ID != 0 || keys[list.Key] == 0 {
				return op, fmt.Errorf("unknown or ambiguous DOCX list key")
			}
			list.ID, list.Key = keys[list.Key], ""
		}
		if _, ok := catalog.levels[list.ID][list.Level]; list.ID > 0 && !ok {
			return op, fmt.Errorf("unknown DOCX list ID or level")
		}
		if !copied {
			op.Paragraphs = append([]ParagraphContent(nil), op.Paragraphs...)
			copied = true
		}
		f := *paragraph.Format
		f.List = &list
		op.Paragraphs[i].Format = &f
	}
	return op, nil
}

func patchParagraphList(existing *node, data []byte, list *ParagraphList) (string, error) {
	if list.Key != "" {
		return "", fmt.Errorf("unresolved DOCX list key")
	}
	open, close := `<dwf:numPr xmlns:dwf="`+wordNS+`">`, `</dwf:numPr>`
	if existing != nil {
		open, close = scopedOpening(existing, data, false)
		for _, child := range existing.children {
			if !child.is("numId") && !child.is("ilvl") {
				return "", fmt.Errorf("protected DOCX numbering cannot be changed")
			}
		}
		if countChild(existing, "numId") > 1 || countChild(existing, "ilvl") > 1 {
			return "", fmt.Errorf("duplicate DOCX list reference")
		}
	}
	level := ""
	if list.ID != 0 {
		patched, err := patchProperty(existing.child("ilvl"), data, "ilvl", map[string]string{"val": strconv.Itoa(list.Level)}, nil)
		if err != nil {
			return "", err
		}
		level = patched
	}
	id, err := patchProperty(existing.child("numId"), data, "numId", map[string]string{"val": strconv.Itoa(list.ID)}, nil)
	if err != nil {
		return "", err
	}
	return open + level + id + close, nil
}

// Imported formats without a supported counter algorithm remain visible as
// labels with their format token, rather than inventing a decimal equivalent.
func listCatalog(c numberingCatalog) []ListInfo {
	ids := make([]int, 0, len(c.levels))
	for id := range c.levels {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if len(ids) > 500 {
		ids = ids[:500]
	}
	result := make([]ListInfo, 0, len(ids))
	for _, id := range ids {
		info := ListInfo{ID: id, Levels: []ListLevelInfo{}}
		for level := 0; level <= 8; level++ {
			if definition, ok := c.levels[id][level]; ok {
				info.Levels = append(info.Levels, ListLevelInfo{Level: level, Kind: definition.kind, Text: definition.text, Start: definition.start})
			}
		}
		result = append(result, info)
	}
	return result
}

func applyListLabels(blocks []Block, c numberingCatalog) {
	counters := map[int][9]int{}
	visit := func(p *Paragraph) {
		if p.List == nil || p.List.ID == 0 || p.List.Level < 0 || p.List.Level > 8 {
			return
		}
		id, level := p.List.ID, p.List.Level
		definition, ok := c.levels[id][level]
		if !ok {
			return
		}
		values := counters[id]
		if values[level] == 0 {
			values[level] = max(1, definition.start)
		} else {
			values[level]++
		}
		for i := level + 1; i < 9; i++ {
			values[i] = 0
		}
		counters[id] = values
		label := definition.text
		for i := 0; i <= 8; i++ {
			token := "%" + strconv.Itoa(i+1)
			if !strings.Contains(label, token) {
				continue
			}
			format := c.levels[id][i].kind
			n := values[i]
			if n == 0 {
				n = max(1, c.levels[id][i].start)
			}
			if format != "decimal" && format != "bullet" {
				p.ListLabel = "[" + definition.kind + "]"
				return
			}
			label = strings.ReplaceAll(label, token, strconv.Itoa(n))
		}
		if len(label) <= 240 {
			p.ListLabel = label
		}
	}
	for i := range blocks {
		b := &blocks[i]
		if b.Paragraph != nil {
			visit(b.Paragraph)
		}
		for r := range b.Rows {
			for cell := range b.Rows[r].Cells {
				for p := range b.Rows[r].Cells[cell].Paragraphs {
					visit(&b.Rows[r].Cells[cell].Paragraphs[p])
				}
			}
		}
	}
}
