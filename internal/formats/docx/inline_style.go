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
	if style.StyleID != nil && !styleText(*style.StyleID, true) {
		return fmt.Errorf("invalid DOCX character style reference")
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
	for _, font := range []string{style.Font, style.EastAsiaFont} {
		if len(font) > 120 || !utf8.ValidString(font) || strings.ContainsAny(font, "\"\\") {
			return fmt.Errorf("invalid DOCX inline font")
		}
		for _, r := range font {
			if r < 0x20 || r == 0x7f {
				return fmt.Errorf("invalid DOCX inline font")
			}
		}
	}
	for _, tag := range []string{style.ProofingLang, style.EastAsiaLang} {
		if tag != "" && !validProofingLanguage(tag) {
			return fmt.Errorf("invalid DOCX proofing language")
		}
	}
	return nil
}

func inlineProperties(props *node, data []byte, style *InlineStyle, inheritedLang *node) (string, error) {
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
	if style.StyleID != nil {
		overrides["rStyle"] = true
		if *style.StyleID != "" {
			property("rStyle", *style.StyleID)
		}
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
	if style.Font != "" || style.EastAsiaFont != "" {
		if countChild(props, "rFonts") > 1 {
			return "", fmt.Errorf("duplicate DOCX font properties cannot be changed")
		}
		overrides["rFonts"] = true
		fonts := props.child("rFonts")
		values := map[string]string{}
		var themes []string
		if style.Reset || style.Font != "" {
			fonts = nil
		}
		if style.Font != "" {
			for _, script := range []string{"ascii", "hAnsi", "eastAsia", "cs"} {
				values[script] = style.Font
			}
		}
		if style.EastAsiaFont != "" {
			values["eastAsia"] = style.EastAsiaFont
			themes = []string{"eastAsiaTheme"}
		}
		fontsXML, err := patchProperty(fonts, data, "rFonts", values, themes)
		if err != nil {
			return "", err
		}
		additions.WriteString(fontsXML)
	}
	if style.ProofingLang != "" || style.EastAsiaLang != "" {
		if countChild(props, "lang") > 1 {
			return "", fmt.Errorf("duplicate DOCX proofing language properties cannot be changed")
		}
		overrides["lang"] = true
		source, sourceData := withProofingSource(props.child("lang"), inheritedLang, data)
		languages, err := patchProofingLanguages(source, sourceData, style)
		if err != nil {
			return "", err
		}
		additions.WriteString(languages)
	}
	// Insert named properties in schema order without reordering untouched XML.
	type chunk struct{ name, raw string }
	chunks := []chunk{}
	if props != nil {
		for _, child := range props.children {
			if child.name.Space == wordNS && overrides[child.name.Local] {
				continue
			}
			chunks = append(chunks, chunk{child.name.Local, string(data[child.start:child.end])})
		}
	}
	extra := []byte(`<dwx:rPr xmlns:dwx="` + wordNS + `">` + additions.String() + `</dwx:rPr>`)
	parsed, err := parseXML(extra)
	if err != nil {
		return "", err
	}
	order := strings.Fields("rStyle rFonts b bCs i iCs caps smallCaps strike dstrike outline shadow emboss imprint noProof snapToGrid vanish webHidden color spacing w kern position sz szCs highlight u effect bdr shd fitText vertAlign rtl cs em lang eastAsianLayout specVanish oMath rPrChange")
	rank := map[string]int{}
	for i, name := range order {
		rank[name] = i + 1
	}
	for _, child := range parsed.children {
		item := chunk{child.name.Local, string(extra[child.start:child.end])}
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
	var content strings.Builder
	for _, c := range chunks {
		content.WriteString(c.raw)
	}
	return opening + content.String() + closing, nil
}
