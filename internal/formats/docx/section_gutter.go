package docx

import "strings"

const maxSectionGutter = 31680

func fixedLeftGutters(settings *node) bool {
	for _, name := range []string{"mirrorMargins", "gutterAtTop", "bookFoldPrinting", "bookFoldRevPrinting", "printTwoOnOne"} {
		if !bindingFlagDisabled(settings, name) {
			return false
		}
	}
	return true
}

// An absent value in a snapshot protects a gutter whose binding or column
// layout cannot be projected with the fixed left margin model.
func sectionGutter(props *node, fixedLeft bool) *int {
	if !fixedLeft || sectionChildCount(props, "pgMar") > 1 || sectionChildCount(props, "cols") > 1 {
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
