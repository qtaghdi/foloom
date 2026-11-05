package docx

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func validateParagraphFormat(f *ParagraphFormat) error {
	if f == nil {
		return nil
	}
	if f.Align != "" && f.Align != "start" && f.Align != "end" && f.Align != "center" && f.Align != "justify" {
		return fmt.Errorf("invalid DOCX paragraph alignment")
	}
	for _, v := range []*int{f.Left, f.Right} {
		if v != nil && (*v < -14400 || *v > 14400) {
			return fmt.Errorf("DOCX paragraph indent exceeds bounds")
		}
	}
	for _, v := range []*int{f.FirstLine, f.Hanging} {
		if v != nil && (*v < 0 || *v > 14400) {
			return fmt.Errorf("invalid DOCX first-line indent")
		}
	}
	if f.FirstLine != nil && f.Hanging != nil {
		return fmt.Errorf("first-line and hanging indent must be separate choices")
	}
	for _, v := range []*int{f.Before, f.After} {
		if v != nil && (*v < 0 || *v > 2880) {
			return fmt.Errorf("DOCX paragraph spacing exceeds bounds")
		}
	}
	if f.LineRule != "" && f.LineRule != "auto" && f.LineRule != "exact" && f.LineRule != "atLeast" {
		return fmt.Errorf("invalid DOCX line spacing rule")
	}
	if f.Line == nil && f.LineRule != "" {
		return fmt.Errorf("DOCX line rule requires line spacing")
	}
	if f.Line != nil {
		lo, hi := 120, 2400
		if f.LineRule != "" && f.LineRule != "auto" {
			lo, hi = 20, 2880
		}
		if *f.Line < lo || *f.Line > hi {
			return fmt.Errorf("DOCX line spacing exceeds bounds")
		}
	}
	if f.TextDirection != "" && f.TextDirection != "lrTb" && f.TextDirection != "tbRl" && f.TextDirection != "btLr" {
		return fmt.Errorf("invalid DOCX text direction")
	}
	return nil
}

// Rebuild only a named scalar property. RawToken retains namespace prefixes,
// including unknown attributes, after scopedOpening supplies inherited bindings.
func patchProperty(n *node, data []byte, name string, values map[string]string, remove []string) (string, error) {
	opening, closing := `<dwf:`+name+` xmlns:dwf="`+wordNS+`">`, `</dwf:`+name+`>`
	if n != nil {
		if len(n.children) != 0 || strings.TrimSpace(n.text.String()) != "" {
			return "", fmt.Errorf("opaque DOCX property cannot be changed")
		}
		opening, closing = scopedOpening(n, data, false)
	}
	token, err := xml.NewDecoder(strings.NewReader(opening)).RawToken()
	if err != nil {
		return "", err
	}
	start, ok := token.(xml.StartElement)
	if !ok {
		return "", fmt.Errorf("invalid DOCX property opening")
	}
	scope := map[string]string{}
	for _, a := range start.Attr {
		if a.Name.Space == "xmlns" {
			scope[a.Name.Local] = a.Value
		} else if a.Name.Local == "xmlns" && a.Name.Space == "" {
			scope[""] = a.Value
		}
	}
	prefix := "dwf"
	for scope[prefix] != "" && scope[prefix] != wordNS {
		prefix += "f"
	}
	qualified := func(n xml.Name) string {
		if n.Space != "" {
			return n.Space + ":" + n.Local
		}
		return n.Local
	}
	var out strings.Builder
	out.WriteString("<" + qualified(start.Name))
	removes := map[string]bool{}
	for _, k := range remove {
		removes[k] = true
	}
	for k := range values {
		removes[k] = true
	}
	for _, a := range start.Attr {
		if scope[a.Name.Space] == wordNS && a.Name.Space != "" && removes[a.Name.Local] {
			continue
		}
		out.WriteString(" " + qualified(a.Name) + `="` + escapeText(a.Value) + `"`)
	}
	if scope[prefix] != wordNS {
		out.WriteString(` xmlns:` + prefix + `="` + wordNS + `"`)
	}
	keys := []string{}
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out.WriteString(" " + prefix + ":" + k + `="` + escapeText(values[k]) + `"`)
	}
	out.WriteString(">" + closing)
	return out.String(), nil
}
func paragraphProperties(props *node, data []byte, f *ParagraphFormat) (string, error) {
	if err := validateParagraphFormat(f); err != nil {
		return "", err
	}
	if f == nil {
		if props == nil {
			return "", nil
		}
		return string(data[props.start:props.end]), nil
	}
	updates := map[string]string{}
	set := func(name string, values map[string]string, remove ...string) error {
		if props != nil {
			count := 0
			for _, child := range props.children {
				if child.is(name) {
					count++
				}
			}
			if count > 1 {
				return fmt.Errorf("duplicate DOCX paragraph property cannot be changed")
			}
		}
		value, err := patchProperty(props.child(name), data, name, values, remove)
		if err == nil {
			updates[name] = value
		}
		return err
	}
	if f.Align != "" {
		value := f.Align
		if value == "justify" {
			value = "both"
		}
		if err := set("jc", map[string]string{"val": value}); err != nil {
			return "", err
		}
	}
	ind := map[string]string{}
	remove := []string{}
	for _, pair := range []struct {
		name    string
		value   *int
		aliases []string
	}{{"left", f.Left, []string{"start", "startChars", "leftChars"}}, {"right", f.Right, []string{"end", "endChars", "rightChars"}}, {"firstLine", f.FirstLine, []string{"firstLineChars", "hanging", "hangingChars"}}, {"hanging", f.Hanging, []string{"hangingChars", "firstLine", "firstLineChars"}}} {
		if pair.value != nil {
			ind[pair.name] = strconv.Itoa(*pair.value)
			remove = append(remove, pair.aliases...)
		}
	}
	if len(ind) > 0 {
		if err := set("ind", ind, remove...); err != nil {
			return "", err
		}
	}
	spacing := map[string]string{}
	remove = nil
	if f.Before != nil {
		spacing["before"] = strconv.Itoa(*f.Before)
		remove = append(remove, "beforeLines", "beforeAutospacing")
	}
	if f.After != nil {
		spacing["after"] = strconv.Itoa(*f.After)
		remove = append(remove, "afterLines", "afterAutospacing")
	}
	if f.Line != nil {
		spacing["line"] = strconv.Itoa(*f.Line)
		rule := f.LineRule
		if rule == "" {
			rule = "auto"
		}
		spacing["lineRule"] = rule
	}
	if len(spacing) > 0 {
		if err := set("spacing", spacing, remove...); err != nil {
			return "", err
		}
	}
	if f.Bidi != nil {
		v := "0"
		if *f.Bidi {
			v = "1"
		}
		if err := set("bidi", map[string]string{"val": v}); err != nil {
			return "", err
		}
	}
	if f.TextDirection != "" {
		if err := set("textDirection", map[string]string{"val": f.TextDirection}); err != nil {
			return "", err
		}
	}
	if len(updates) == 0 {
		if props == nil {
			return "", nil
		}
		return string(data[props.start:props.end]), nil
	}
	opening, closing := `<dwf:pPr xmlns:dwf="`+wordNS+`">`, `</dwf:pPr>`
	if props != nil {
		opening, closing = scopedOpening(props, data, false)
	}
	type chunk struct{ name, raw string }
	chunks := []chunk{}
	if props != nil {
		for _, c := range props.children {
			if _, changed := updates[c.name.Local]; !changed {
				chunks = append(chunks, chunk{c.name.Local, string(data[c.start:c.end])})
			}
		}
	}
	for name, raw := range updates {
		chunks = append(chunks, chunk{name, raw})
	}
	order := strings.Fields("pStyle keepNext keepLines pageBreakBefore framePr widowControl numPr suppressLineNumbers pBdr shd tabs suppressAutoHyphens kinsoku wordWrap overflowPunct topLinePunct autoSpaceDE autoSpaceDN bidi adjustRightInd snapToGrid spacing ind contextualSpacing mirrorIndents suppressOverlap jc textDirection textAlignment textboxTightWrap outlineLvl divId cnfStyle rPr sectPr pPrChange")
	rank := map[string]int{}
	for i, n := range order {
		rank[n] = i
	}
	sort.SliceStable(chunks, func(i, j int) bool { return rank[chunks[i].name] < rank[chunks[j].name] })
	var content strings.Builder
	content.WriteString(opening)
	for _, c := range chunks {
		content.WriteString(c.raw)
	}
	content.WriteString(closing)
	return content.String(), nil
}
