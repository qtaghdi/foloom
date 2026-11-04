package docx

import (
	"encoding/hex"
	"strconv"
	"strings"
	"unicode"
)

func previewNumber(value string, min, max int) *int {
	n, err := strconv.Atoi(value)
	if err != nil || n < min || n > max {
		return nil
	}
	return &n
}

// Page and paragraph lengths are twips. Mixed section sizes cannot describe
// one continuous paper surface, so they use the browser's default preview.
func pageLayout(body *node) *PageLayout {
	for _, block := range body.children {
		if block.child("pPr").child("sectPr") != nil {
			return nil
		}
	}
	section := body.child("sectPr")
	size := section.child("pgSz")
	width := previewNumber(size.attr("w"), 2000, 50000)
	height := previewNumber(size.attr("h"), 2000, 50000)
	if width == nil || height == nil {
		return nil
	}
	page := &PageLayout{Width: *width, Height: *height, Top: 1440, Right: 1440, Bottom: 1440, Left: 1440}
	margins := section.child("pgMar")
	for _, side := range []struct {
		name  string
		value *int
		max   int
	}{{"top", &page.Top, page.Height}, {"right", &page.Right, page.Width},
		{"bottom", &page.Bottom, page.Height}, {"left", &page.Left, page.Width}} {
		if n := previewNumber(margins.attr(side.name), 0, side.max); n != nil {
			*side.value = *n
		}
	}
	if page.Left+page.Right > page.Width-1200 || page.Top+page.Bottom > page.Height-1200 {
		return nil
	}
	return page
}

func paragraphLayout(props *node) *ParagraphLayout {
	var layout ParagraphLayout
	switch props.child("jc").attr("val") {
	case "left", "start":
		layout.Align = "start"
	case "right", "end":
		layout.Align = "end"
	case "center":
		layout.Align = "center"
	case "both":
		layout.Align = "justify"
	}
	spacing := props.child("spacing")
	layout.Before = previewNumber(spacing.attr("before"), 0, 2880)
	layout.After = previewNumber(spacing.attr("after"), 0, 2880)
	if layout.Align == "" && layout.Before == nil && layout.After == nil {
		return nil
	}
	return &layout
}

func applyRunPreview(run *Run, props *node) {
	if size := previewNumber(props.child("sz").attr("val"), 8, 288); size != nil {
		run.SizeHalfPoints = *size
	}
	color := props.child("color")
	value := color.attr("val")
	if len(value) == 6 && color.attr("themeColor") == "" {
		if _, err := hex.DecodeString(value); err == nil {
			run.Color = "#" + value
		}
	}
	fonts := props.child("rFonts")
	for _, key := range []string{"ascii", "hAnsi", "eastAsia"} {
		font := strings.TrimSpace(fonts.attr(key))
		if font != "" && len(font) <= 120 && strings.IndexFunc(font, func(r rune) bool {
			return unicode.IsControl(r) || strings.ContainsRune("\"\\", r)
		}) == -1 {
			run.Font = font
			break
		}
	}
}
