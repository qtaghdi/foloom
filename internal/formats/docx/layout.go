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

func applyParagraphPreview(layout *ParagraphLayout, props *node) {
	ind := props.child("ind")
	for _, pair := range []struct {
		name string
		dest **int
		min  int
	}{{"left", &layout.Left, -14400}, {"right", &layout.Right, -14400}, {"firstLine", &layout.FirstLine, 0}, {"hanging", &layout.Hanging, 0}} {
		if n := previewNumber(ind.attr(pair.name), pair.min, 14400); n != nil {
			*pair.dest = n
		}
	}
	if ind.attr("firstLine") != "" {
		layout.Hanging = nil
	}
	if ind.attr("hanging") != "" {
		layout.FirstLine = nil
	}
	if n := props.child("bidi"); n != nil {
		layout.Bidi = onOff(n.attr("val"), true)
	}
	if n := props.child("textDirection"); n != nil {
		layout.TextDirection = n.attr("val")
	}
	if spacing := props.child("spacing"); spacing.attr("line") != "" {
		rule := spacing.attr("lineRule")
		if rule == "" {
			rule = "auto"
		}
		lo, hi := 120, 2400
		if rule != "auto" {
			lo, hi = 20, 2880
		}
		if n := previewNumber(spacing.attr("line"), lo, hi); n != nil && (rule == "auto" || rule == "exact" || rule == "atLeast") {
			layout.Line = n
			layout.LineRule = rule
		}
	}

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
	if before := previewNumber(spacing.attr("before"), 0, 2880); before != nil {
		layout.Before = before
	}
	if after := previewNumber(spacing.attr("after"), 0, 2880); after != nil {
		layout.After = after
	}
}

func applyRunPreview(run *Run, props *node) {
	if n := props.child("u"); n != nil {
		run.Underline = n.attr("val") != "none"
	}
	if n := props.child("strike"); n != nil {
		run.Strike = onOff(n.attr("val"), true)
	}
	if n := props.child("dstrike"); n != nil && onOff(n.attr("val"), true) {
		run.Strike = true
	}
	if n := props.child("highlight"); n != nil {
		run.Highlight = n.attr("val")
	}
	if n := props.child("vertAlign"); n != nil {
		run.Vertical = n.attr("val")
	}

	if size := previewNumber(props.child("sz").attr("val"), 8, 288); size != nil {
		run.SizeHalfPoints = *size
	}
	color := props.child("color")
	value := color.attr("val")
	if color != nil {
		run.Color = ""
	}
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

// A break followed by text would split a paragraph mid-run. Only a trailing
// break can be represented at a block boundary without changing edit targets.
func trailingPageBreak(paragraph *node) *node {
	var last *node
	for _, run := range paragraph.children {
		switch {
		case run.is("r"):
			for _, item := range run.children {
				if !item.is("rPr") {
					last = item
				}
			}
		case !run.is("pPr"):
			last = run
		}
	}
	if last != nil && last.is("br") && last.attr("type") == "page" {
		return last
	}
	return nil
}
