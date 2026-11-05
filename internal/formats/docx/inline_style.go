package docx

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var inlineColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func validateInlineStyle(style *InlineStyle) error {
	if style == nil {
		return nil
	}
	if style.Highlight != "" && !strings.Contains(" none yellow green cyan magenta blue red darkBlue darkCyan darkGreen darkMagenta darkRed darkYellow darkGray lightGray black white ", " "+style.Highlight+" ") {
		return fmt.Errorf("invalid DOCX highlight")
	}
	if style.Vertical != "" && style.Vertical != "baseline" && style.Vertical != "superscript" && style.Vertical != "subscript" {
		return fmt.Errorf("invalid DOCX vertical alignment")
	}
	if style.Color != "" && !inlineColor.MatchString(style.Color) {
		return fmt.Errorf("invalid DOCX inline color")
	}
	if style.SizeHalfPoints != nil && (*style.SizeHalfPoints < 8 || *style.SizeHalfPoints > 288) {
		return fmt.Errorf("DOCX inline font size is outside preview bounds")
	}
	if len(style.Font) > 120 || !utf8.ValidString(style.Font) || strings.ContainsAny(style.Font, "\"\\") {
		return fmt.Errorf("invalid DOCX inline font")
	}
	for _, r := range style.Font {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("invalid DOCX inline font")
		}
	}
	return nil
}

func inlineProperties(props *node, data []byte, style *InlineStyle) (string, error) {
	if err := validateInlineStyle(style); err != nil {
		return "", err
	}
	if style == nil {
		if props == nil {
			return "", nil
		}
		return string(data[props.start:props.end]), nil
	}
	opening, closing := `<dw:rPr xmlns:dw="`+wordNS+`">`, `</dw:rPr>`
	if props != nil {
		opening, closing = scopedOpening(props, data, false)
	}
	overrides := map[string]bool{}
	if style.Reset {
		for _, name := range []string{"b", "bCs", "i", "iCs", "u", "strike", "dstrike", "color", "sz", "szCs", "rFonts", "highlight", "vertAlign"} {
			overrides[name] = true
		}
	}
	var additions strings.Builder
	flag := func(name string, value *bool) {
		if value == nil {
			return
		}
		overrides[name] = true
		val := "0"
		if *value {
			val = "1"
		}
		additions.WriteString(`<dw:` + name + ` xmlns:dw="` + wordNS + `" dw:val="` + val + `"/>`)
	}
	flag("b", style.Bold)
	flag("bCs", style.Bold)
	flag("i", style.Italic)
	flag("iCs", style.Italic)
	property := func(name, value string) {
		overrides[name] = true
		additions.WriteString(`<dw:` + name + ` xmlns:dw="` + wordNS + `" dw:val="` + escapeText(value) + `"/>`)
	}
	if style.Underline != nil {
		value := "none"
		if *style.Underline {
			value = "single"
		}
		property("u", value)
	}
	flag("strike", style.Strike)
	if style.Strike != nil {
		no := false
		flag("dstrike", &no)
	}
	if style.Highlight != "" {
		property("highlight", style.Highlight)
	}
	if style.Vertical != "" {
		property("vertAlign", style.Vertical)
	}
	if style.Color != "" {
		property("color", strings.TrimPrefix(style.Color, "#"))
	}
	if style.SizeHalfPoints != nil {
		property("sz", strconv.Itoa(*style.SizeHalfPoints))
		property("szCs", strconv.Itoa(*style.SizeHalfPoints))
	}
	if style.Font != "" {
		overrides["rFonts"] = true
		value := escapeText(style.Font)
		additions.WriteString(`<dw:rFonts xmlns:dw="` + wordNS + `" dw:ascii="` + value + `" dw:hAnsi="` + value + `" dw:eastAsia="` + value + `" dw:cs="` + value + `"/>`)
	}
	var content strings.Builder
	if props != nil {
		for _, child := range props.children {
			if child.name.Space == wordNS && overrides[child.name.Local] {
				continue
			}
			content.Write(data[child.start:child.end])
		}
	}
	content.WriteString(additions.String())
	return opening + content.String() + closing, nil
}
