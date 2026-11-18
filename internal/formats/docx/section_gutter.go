package docx

import "strings"

const maxSectionGutter = 31680

func sectionGutterSide(settings *node) string {
	for _, name := range []string{"mirrorMargins", "bookFoldPrinting", "bookFoldRevPrinting", "printTwoOnOne"} {
		if !bindingFlagDisabled(settings, name) {
			return ""
		}
	}
	flag := settings.child("gutterAtTop")
	if bindingFlagDisabled(settings, "gutterAtTop") {
		return "left"
	}
	if sectionChildCount(settings, "gutterAtTop") != 1 || len(flag.children) != 0 || strings.TrimSpace(flag.text.String()) != "" {
		return ""
	}
	switch flag.attr("val") {
	case "", "1", "true", "on":
		return "top"
	default:
		return ""
	}
}

// An absent value in a snapshot protects a gutter whose binding or column
// layout cannot be projected with the fixed left or top margin model.
func sectionGutter(props *node, supported bool) *int {
	if !supported || sectionChildCount(props, "pgMar") > 1 || sectionChildCount(props, "cols") > 1 {
		return nil
	}
	for _, name := range []string{"rtlGutter", "bidi"} {
		if !bindingFlagDisabled(props, name) {
			return nil
		}
	}
	if cols := props.child("cols"); cols != nil && (len(cols.children) > 0 || !onOff(cols.attr("equalWidth"), true)) {
		return nil
	}
	raw := props.child("pgMar").attr("gutter")
	if raw == "" {
		raw = "0"
	}
	return previewNumber(raw, 0, maxSectionGutter)
}

func bindingFlagDisabled(root *node, name string) bool {
	flag := root.child(name)
	if flag == nil {
		return true
	}
	if sectionChildCount(root, name) != 1 || len(flag.children) != 0 || strings.TrimSpace(flag.text.String()) != "" {
		return false
	}
	switch flag.attr("val") {
	case "0", "false", "off":
		return true
	default:
		return false
	}
}
