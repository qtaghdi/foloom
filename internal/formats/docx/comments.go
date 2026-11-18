package docx

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type CommentEdit struct {
	Text        string `json:"text"`
	StartRunID  string `json:"startRunId,omitempty"`
	EndRunID    string `json:"endRunId,omitempty"`
	StartOffset *int   `json:"startOffset,omitempty" jsonschema:"UTF-16 start in the first run, inclusive"`
	EndOffset   *int   `json:"endOffset,omitempty" jsonschema:"UTF-16 end in the last run, exclusive"`
}

type ReviewComment struct {
	Resolved           bool   `json:"resolved,omitempty"`
	ResolutionEditable bool   `json:"resolutionEditable,omitempty"`
	ID                 string `json:"id"`
	Author             string `json:"author"`
	Date               string `json:"date,omitempty"`
	Text               string `json:"text"`
	ParagraphID        string `json:"paragraphId,omitempty"`
}

func (p *documentPackage) commentsPart(changes map[string][]byte) (string, []byte, *node, error) {
	rels := p.partData(documentRelationships, changes)
	if len(rels) == 0 {
		return "", nil, nil, nil
	}
	root, err := parseXML(rels)
	if err != nil {
		return "", nil, nil, err
	}
	filename := ""
	for _, rel := range root.children {
		if rel.name.Space != packageRelNS || rel.name.Local != "Relationship" || attribute(rel, "", "Type") != officeRelNS+"/comments" {
			continue
		}
		if filename != "" || attribute(rel, "", "TargetMode") == "External" {
			return "", nil, nil, fmt.Errorf("ambiguous DOCX comments relationship")
		}
		target := attribute(rel, "", "Target")
		if strings.HasPrefix(target, "/") {
			filename = strings.TrimPrefix(path.Clean(target), "/")
		} else {
			filename = path.Clean(path.Join("word", target))
		}
	}
	if filename == "" {
		return "", nil, nil, nil
	}
	data := p.partData(filename, changes)
	comments, err := parseXML(data)
	if err != nil || !comments.is("comments") {
		return "", nil, nil, fmt.Errorf("invalid DOCX comments part")
	}
	return filename, data, comments, nil
}

func reviewText(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	value = value[:limit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value + "…"
}

func (p *documentPackage) projectComments(root *node, blocks []Block) []ReviewComment {
	_, _, comments, err := p.commentsPart(nil)
	if err != nil || comments == nil {
		return nil
	}
	var resolutions map[string]*node
	if _, extended, err := p.commentExtensions(nil); err == nil && extended != nil {
		resolutions = commentResolutionNodes(comments, extended)
	}
	locations := map[string]string{}
	blockIndex := 0
	for _, block := range root.child("body").children {
		if !block.is("p") && !block.is("tbl") {
			continue
		}
		snapshot := blocks[blockIndex]
		blockIndex++
		if snapshot.Paragraph == nil {
			continue
		}
		for _, child := range block.children {
			if child.is("commentRangeStart") {
				id := attribute(child, wordNS, "id")
				if _, exists := locations[id]; exists {
					locations[id] = ""
				} else {
					locations[id] = snapshot.Paragraph.ID
				}
			}
		}
	}
	counts := map[string]int{}
	for _, comment := range comments.children {
		if comment.is("comment") {
			counts[attribute(comment, wordNS, "id")]++
		}
	}
	out := []ReviewComment{}
	for _, comment := range comments.children {
		id := attribute(comment, wordNS, "id")
		if !comment.is("comment") || id == "" || counts[id] != 1 {
			continue
		}
		var lines []string
		for _, child := range comment.children {
			if child.is("p") {
				lines = append(lines, textContent(child))
			}
		}
		date := attribute(comment, wordNS, "date")
		if _, err := time.Parse(time.RFC3339, date); err != nil {
			date = ""
		}
		resolved, resolutionEditable := false, false
		if entry := resolutions[id]; entry != nil {
			resolved, resolutionEditable = commentDone(entry)
		}
		out = append(out, ReviewComment{ID: reviewText(id, 40), Author: reviewText(attribute(comment, wordNS, "author"), 256), Date: date,
			Text: reviewText(strings.Join(lines, "\n"), 4000), ParagraphID: locations[id], Resolved: resolved, ResolutionEditable: resolutionEditable})
		if len(out) == 100 {
			break
		}
	}
	return out
}

func commentByteOffset(text string, units int) (int, bool) {
	for offset, r := range text {
		if units == 0 {
			return offset, true
		}
		units--
		if r > 0xffff {
			units--
		}
		if units < 0 {
			return 0, false
		}
	}
	return len(text), units == 0
}

func commentRunFragments(data []byte, t target, offsets []int, markers []string) (replacement, error) {
	run := t.run
	var body strings.Builder
	last := 0
	whole := offsets[0] == 0 && offsets[len(offsets)-1] == len(t.text)
	if len(offsets) == 1 && (offsets[0] == 0 || offsets[0] == len(t.text)) {
		if offsets[0] == 0 {
			return replacement{run.start, run.start, []byte(markers[0])}, nil
		}
		return replacement{run.end, run.end, []byte(markers[0])}, nil
	}
	if whole {
		body.WriteString(markers[0])
		body.Write(data[run.start:run.end])
		body.WriteString(markers[1])
		return replacement{run.start, run.end, []byte(body.String())}, nil
	}
	text := run.child("t")
	props := run.child("rPr")
	if text == nil || len(text.children) != 0 || countChild(run, "t") != 1 || len(run.children) > 2 ||
		len(run.children) == 2 && props == nil || !plainTextAttributes(text) || !onlyNamespaceAttributes(run) ||
		strings.ContainsAny(t.text, "\n\t") || strings.Contains(string(data[run.start:run.end]), "<!--") {
		return replacement{}, fmt.Errorf("DOCX comment boundary requires splittable plain text")
	}
	open, close := scopedOpening(run, data, false)
	writeFragment := func(value string) {
		if value == "" {
			return
		}
		body.WriteString(open)
		if props != nil {
			body.Write(data[props.start:props.end])
		}
		body.WriteString(inlineXML(value))
		body.WriteString(close)
	}
	for i, offset := range offsets {
		writeFragment(t.text[last:offset])
		body.WriteString(markers[i])
		last = offset
	}
	writeFragment(t.text[last:])
	return replacement{run.start, run.end, []byte(body.String())}, nil
}

func (p *documentPackage) appendComment(root *node, t target, edit CommentEdit, actor string, targets map[string]target, changes map[string][]byte) ([]replacement, error) {
	paragraph := t.paragraph
	if t.kind != "replace_paragraphs" || paragraph == nil || !paragraph.parent.is("body") || len(edit.Text) > 4000 ||
		strings.TrimSpace(edit.Text) == "" || !validInlineText(edit.Text) || strings.TrimSpace(textContent(paragraph)) == "" {
		return nil, fmt.Errorf("DOCX comment requires a non-empty body paragraph and bounded text")
	}
	first, last := paragraph.inner, paragraph.endStart
	if props := paragraph.child("pPr"); props != nil {
		first = props.end
	}
	var rangeStart, rangeEnd target
	var startByte, endByte int
	useOffsets := edit.StartOffset != nil || edit.EndOffset != nil
	if useOffsets || edit.StartRunID != "" || edit.EndRunID != "" {
		start, end := targets[edit.StartRunID], targets[edit.EndRunID]
		if start.kind != "replace_run_text" || end.kind != "replace_run_text" ||
			start.run == nil || end.run == nil || start.run.parent != paragraph ||
			end.run.parent != paragraph || start.run.start > end.run.start {
			return nil, fmt.Errorf("DOCX comment range requires ordered runs in the target paragraph")
		}
		first, last = start.run.start, end.run.end
		if useOffsets {
			if edit.StartOffset == nil || edit.EndOffset == nil {
				return nil, fmt.Errorf("DOCX comment requires both character offsets")
			}
			var okStart, okEnd bool
			startByte, okStart = commentByteOffset(start.text, *edit.StartOffset)
			endByte, okEnd = commentByteOffset(end.text, *edit.EndOffset)
			if !okStart || !okEnd || startByte == len(start.text) || endByte == 0 ||
				start.run == end.run && startByte >= endByte {
				return nil, fmt.Errorf("invalid DOCX comment character range")
			}
			rangeStart, rangeEnd = start, end
		}
	}
	filename, raw, comments, err := p.commentsPart(changes)
	if err != nil {
		return nil, err
	}
	highest := int64(-1)
	seen := map[int64]bool{}
	check := func(id string) error {
		n, err := strconv.ParseInt(id, 10, 32)
		if err != nil || n < 0 {
			return fmt.Errorf("unsupported DOCX comment identifier")
		}
		if n > highest {
			highest = n
		}
		return nil
	}
	if comments != nil {
		for _, comment := range comments.children {
			if !comment.is("comment") {
				continue
			}
			id := attribute(comment, wordNS, "id")
			if err := check(id); err != nil {
				return nil, err
			}
			number, _ := strconv.ParseInt(id, 10, 32)
			if seen[number] {
				return nil, fmt.Errorf("duplicate DOCX comment identifier")
			}
			seen[number] = true
		}
	}
	if len(seen) >= 100 {
		return nil, fmt.Errorf("DOCX comments limit exceeded")
	}
	var scan func(*node) error
	scan = func(n *node) error {
		if n.is("commentRangeStart") || n.is("commentRangeEnd") || n.is("commentReference") {
			if err := check(attribute(n, wordNS, "id")); err != nil {
				return err
			}
		}
		for _, child := range n.children {
			if err := scan(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := scan(root); err != nil {
		return nil, err
	}
	if highest >= 2147483647 {
		return nil, fmt.Errorf("DOCX comment identifiers exhausted")
	}
	id := strconv.FormatInt(highest+1, 10)
	if filename == "" {
		filename, err = p.addWordPart(changes, "comments", "rIdFoloomComments")
		if err != nil {
			return nil, err
		}
		raw = []byte(`<w:comments xmlns:w="` + wordNS + `"></w:comments>`)
	}
	author := "사용자"
	if actor == "ai" {
		author = "AI"
	}
	var xml strings.Builder
	xml.WriteString(`<dwc:comment xmlns:dwc="` + wordNS + `" dwc:id="` + id + `" dwc:author="` + author + `" dwc:date="` + time.Now().UTC().Format(time.RFC3339) + `">`)
	for _, line := range strings.Split(edit.Text, "\n") {
		xml.WriteString(`<dwc:p><dwc:r><dwc:t xml:space="preserve">` + escapeText(line) + `</dwc:t></dwc:r></dwc:p>`)
	}
	xml.WriteString(`</dwc:comment>`)
	next, err := appendXMLChild(raw, xml.String())
	if err != nil {
		return nil, err
	}
	changes[filename] = next
	start := `<dwc:commentRangeStart xmlns:dwc="` + wordNS + `" dwc:id="` + id + `"/>`
	end := `<dwc:commentRangeEnd xmlns:dwc="` + wordNS + `" dwc:id="` + id + `"/><dwc:r xmlns:dwc="` + wordNS + `"><dwc:commentReference dwc:id="` + id + `"/></dwc:r>`
	if useOffsets {
		if rangeStart.run == rangeEnd.run {
			edit, err := commentRunFragments(p.document, rangeStart, []int{startByte, endByte}, []string{start, end})
			return []replacement{edit}, err
		}
		first, err := commentRunFragments(p.document, rangeStart, []int{startByte}, []string{start})
		if err != nil {
			return nil, err
		}
		last, err := commentRunFragments(p.document, rangeEnd, []int{endByte}, []string{end})
		return []replacement{first, last}, err
	}
	return []replacement{{first, first, []byte(start)}, {last, last, []byte(end)}}, nil
}
