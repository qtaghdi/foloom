package docx

import (
	"fmt"
	"sort"
	"strings"
)

// Structure editing is deliberately narrower than text replacement. Moving
// opaque anchors, section boundaries or extension metadata requires a contract
// that the current source-reference plan does not yet provide.
func structuralParagraph(p *node, data []byte) bool {
	return structuralParagraphContents(p, data, false, nil)
}

func structuralParagraphContents(p *node, data []byte, allowBookmarks bool, ignoredProperty *node) bool {
	if strings.Contains(string(data[p.start:p.end]), "<!--") {
		return false
	}
	for _, a := range p.attrs {
		if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" {
			continue
		}
		if a.Name.Space == wordNS && strings.HasPrefix(a.Name.Local, "rsid") {
			continue
		}
		if a.Name.Space == "http://schemas.microsoft.com/office/word/2010/wordml" && (a.Name.Local == "paraId" || a.Name.Local == "textId") {
			continue
		}
		return false
	}

	if countChild(p, "pPr") > 1 {
		return false
	}
	allowedProps := " pStyle keepNext keepLines widowControl numPr suppressLineNumbers pBdr shd tabs suppressAutoHyphens contextualSpacing mirrorIndents suppressOverlap bidi adjustRightInd snapToGrid spacing ind jc textDirection textAlignment textboxTightWrap outlineLvl cnfStyle rPr "
	var safeProperties func(*node) bool
	safeProperties = func(n *node) bool {
		if n == ignoredProperty {
			return true
		}
		if strings.TrimSpace(n.text.String()) != "" {
			return false
		}
		if n.name.Space != wordNS {
			return false
		}
		for _, a := range n.attrs {
			if a.Name.Local == "id" && a.Name.Space != "xmlns" {
				return false
			}
		}
		for _, name := range []string{"sectPr", "pPrChange", "rPrChange", "numberingChange", "drawing", "object", "sdt"} {
			if n.is(name) {
				return false
			}
		}
		for _, c := range n.children {
			if !safeProperties(c) {
				return false
			}
		}
		return true
	}
	if props := p.child("pPr"); props != nil && !safeProperties(props) {
		return false
	}
	for _, child := range p.children {
		if child.is("pPr") {
			for _, prop := range child.children {
				if prop == ignoredProperty {
					continue
				}
				// An explicit off value can travel with normal paragraph formatting.
				if prop.is("pageBreakBefore") && sectionChildCount(child, "pageBreakBefore") == 1 {
					switch attribute(prop, wordNS, "val") {
					case "0", "false", "off":
						continue
					}
				}
				if prop.name.Space != wordNS || !strings.Contains(allowedProps, " "+prop.name.Local+" ") {
					return false
				}
			}
		} else if !child.is("r") && !(allowBookmarks && (child.is("bookmarkStart") || child.is("bookmarkEnd"))) {
			return false
		}
		if child.is("r") {
			if countChild(child, "rPr") > 1 {
				return false
			}
			if props := child.child("rPr"); props != nil && !safeProperties(props) {
				return false
			}
			for _, part := range child.children {
				if !part.is("t") && !part.is("rPr") && !plainLineBreak(part) && !plainTab(part) {
					return false
				}
				if part.is("t") {
					for _, attr := range part.attrs {
						if attr.Name.Space != "xmlns" && attr.Name.Local != "xmlns" && !(attr.Name.Space == "http://www.w3.org/XML/1998/namespace" && attr.Name.Local == "space") {
							return false
						}
					}
				}
			}
			for _, a := range child.attrs {
				if a.Name.Space != "xmlns" && a.Name.Local != "xmlns" && !(a.Name.Space == wordNS && strings.HasPrefix(a.Name.Local, "rsid")) {
					return false
				}
			}
		}
	}
	return true
}

func paragraphReplacement(data []byte, targets map[string]target, op Operation) (replacement, error) {
	fail := func(message string) (replacement, error) {
		return replacement{}, fmt.Errorf("DOCX paragraph plan: %s", message)
	}
	first, ok := targets[op.TargetID]
	if !ok || first.kind != "replace_paragraphs" {
		return fail("unsupported or stale first paragraph")
	}
	last, ok := targets[op.EndTargetID]
	if !ok || last.kind != "replace_paragraphs" || last.node.parent != first.node.parent || last.node.start < first.node.start {
		return fail("range must contain ordered sibling paragraphs")
	}
	if op.Text != "" || len(op.Paragraphs) == 0 || len(op.Paragraphs) > 1000 {
		return fail("requires 1 to 1000 replacement paragraphs and no text field")
	}
	paragraphs := map[string]target{}
	byNode := map[*node]string{}
	for id, target := range targets {
		if target.kind == "replace_paragraphs" {
			byNode[target.node] = id
		}
	}
	originals := []*node{}
	texts := []string{}
	for _, n := range first.node.parent.children {
		if n.start < first.node.start || n.start > last.node.start {
			continue
		}
		id, supported := byNode[n]
		if !supported {
			return fail("range crosses a protected structure")
		}
		paragraphs[id] = targets[id]
		originals = append(originals, n)
		texts = append(texts, targets[id].text)
	}
	if strings.Join(texts, "\n") != op.ExpectedText {
		return fail("expected paragraph text mismatch")
	}
	// Never remove interstitial XML that the semantic model cannot represent.
	for i := 1; i < len(originals); i++ {
		if strings.TrimSpace(string(data[originals[i-1].end:originals[i].start])) != "" {
			return fail("range contains opaque interstitial XML")
		}
	}
	used := map[string]bool{}
	var result strings.Builder
	textBytes, runCount := 0, 0
	for _, paragraph := range op.Paragraphs {
		source, ok := paragraphs[paragraph.SourceID]
		if !ok || len(paragraph.Runs) == 0 {
			return fail("paragraph formatting source is outside the range or has no runs")
		}
		var content strings.Builder
		props, err := paragraphProperties(source.node.child("pPr"), data, paragraph.Format)
		if err != nil {
			return fail(err.Error())
		}
		content.WriteString(props)
		for _, run := range paragraph.Runs {
			runCount++
			textBytes += len(run.Text)
			if runCount > maxRuns || textBytes > 4<<20 || !validInlineText(run.Text) {
				return fail("run count, text size or supported-character limit exceeded")
			}
			target, exists := targets[run.SourceID]
			if !exists || target.kind == "replace_paragraphs" || paragraphs[byNode[target.paragraph]].node == nil {
				return fail("run formatting source is outside the range")
			}
			if err := validateInlineStyle(run.Style); err != nil {
				return fail(err.Error())
			}
			var runXML string
			if target.run == nil {
				props, err := inlineProperties(nil, data, run.Style)
				if err != nil {
					return fail(err.Error())
				}
				runXML = `<dw:r xmlns:dw="` + wordNS + `">` + props + inlineXML(run.Text) + `</dw:r>`
			} else if target.text == run.Text && run.Style == nil {
				opening, closing := scopedOpening(target.run, data, false)
				runXML = opening + string(data[target.run.inner:target.run.endStart]) + closing
			} else {
				opening, closing := scopedOpening(target.run, data, false)
				props, err := inlineProperties(target.run.child("rPr"), data, run.Style)
				if err != nil {
					return fail(err.Error())
				}
				runXML = opening + props + inlineXML(run.Text) + closing
			}
			if content.Len()+len(runXML) > maxPartBytes {
				return fail("paragraph XML exceeds limit")
			}
			content.WriteString(runXML)
		}
		opening, closing := scopedOpening(source.node, data, used[paragraph.SourceID])
		used[paragraph.SourceID] = true
		if result.Len()+len(opening)+content.Len()+len(closing) > maxPartBytes {
			return fail("replacement XML exceeds limit")
		}
		result.WriteString(opening)
		result.WriteString(content.String())
		result.WriteString(closing)
	}
	return replacement{first.node.start, last.node.end, []byte(result.String())}, nil
}

// A fragment may move from a paragraph that declared its own prefixes. Carry
// the original in-scope bindings with it, including the default namespace.
// New paragraph identities carry only namespace declarations, never source IDs.
func scopedOpening(n *node, data []byte, fresh bool) (string, string) {
	scope := map[string]string{}
	chain := []*node{}
	for current := n; current != nil; current = current.parent {
		chain = append(chain, current)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		for _, a := range chain[i].attrs {
			if a.Name.Space == "xmlns" {
				scope[a.Name.Local] = a.Value
			} else if a.Name.Space == "" && a.Name.Local == "xmlns" {
				scope[""] = a.Value
			}
		}
	}
	raw := string(data[n.start:n.inner])
	name := strings.TrimSuffix(strings.TrimSuffix(strings.Fields(strings.TrimPrefix(raw, "<"))[0], "/>"), ">")
	opening := strings.TrimSuffix(strings.TrimSuffix(raw, "/>"), ">")
	declared := map[string]bool{}
	for _, a := range n.attrs {
		if a.Name.Space == "xmlns" {
			declared[a.Name.Local] = true
		} else if a.Name.Space == "" && a.Name.Local == "xmlns" {
			declared[""] = true
		}
	}
	if fresh {
		prefix := "dw"
		for scope[prefix] != "" {
			prefix += "p"
		}
		scope[prefix] = wordNS
		name = prefix + ":p"
		opening = "<" + name
		declared = map[string]bool{}
	}
	prefixes := []string{}
	for prefix := range scope {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	for _, prefix := range prefixes {
		if !declared[prefix] {
			attr := "xmlns"
			if prefix != "" {
				attr += ":" + prefix
			}
			opening += " " + attr + `="` + escapeText(scope[prefix]) + `"`
		}
	}
	return opening + ">", "</" + name + ">"
}

// LF represents a Word line break inside a run. Paragraph boundaries are
// represented by ParagraphContent, never by a textarea's visual wrapping.
func validInlineText(text string) bool {
	return len(text) <= maxTextBytes && validText(strings.NewReplacer("\n", "", "\t", "").Replace(text))
}
func inlineXML(text string) string {
	var out strings.Builder
	for {
		i := strings.IndexAny(text, "\n\t")
		part := text
		if i >= 0 {
			part = text[:i]
		}
		out.WriteString(`<dw:t xmlns:dw="` + wordNS + `" xml:space="preserve">` + escapeText(part) + `</dw:t>`)
		if i < 0 {
			break
		}
		if text[i] == '\t' {
			out.WriteString(`<dw:tab xmlns:dw="` + wordNS + `"/>`)
		} else {
			out.WriteString(`<dw:br xmlns:dw="` + wordNS + `"/>`)
		}
		text = text[i+1:]
	}
	return out.String()
}

func plainTab(n *node) bool {
	if !n.is("tab") || len(n.children) != 0 || strings.TrimSpace(n.text.String()) != "" {
		return false
	}
	for _, attr := range n.attrs {
		if attr.Name.Space != "xmlns" && !(attr.Name.Space == "" && attr.Name.Local == "xmlns") {
			return false
		}
	}
	return true
}

func plainLineBreak(n *node) bool {
	if !n.is("br") || len(n.children) != 0 || strings.TrimSpace(n.text.String()) != "" {
		return false
	}
	for _, attr := range n.attrs {
		if attr.Name.Space == "xmlns" || attr.Name.Space == "" && attr.Name.Local == "xmlns" {
			continue
		}
		if attr.Name.Space == wordNS && attr.Name.Local == "type" && attr.Value == "textWrapping" {
			continue
		}
		return false
	}
	return true
}

func plainTextAttributes(n *node) bool {
	for _, a := range n.attrs {
		if a.Name.Space == "xmlns" || a.Name.Space == "" && a.Name.Local == "xmlns" || a.Name.Space == "http://www.w3.org/XML/1998/namespace" && a.Name.Local == "space" {
			continue
		}
		return false
	}
	return true
}

func countChild(n *node, name string) int {
	if n == nil {
		return 0
	}
	count := 0
	for _, child := range n.children {
		if child.is(name) {
			count++
		}
	}
	return count
}
