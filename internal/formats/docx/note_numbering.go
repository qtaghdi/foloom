package docx

import "strconv"

// Only project a numbering sequence when every reference in the body can be
// placed in document order and matched to its own automatic note marker.
func continuousNoteLabels(kind string, body, settings *node, notes map[string]bool) map[string]string {
	property := kind + "Pr"
	section := body.child("sectPr")
	if sectionChildCount(body, "sectPr") > 1 || sectionChildCount(section, property) > 1 || sectionChildCount(settings, property) > 1 {
		return nil
	}
	documentProperties, sectionProperties := settings.child(property), section.child(property)
	for _, properties := range []*node{documentProperties, sectionProperties} {
		if sectionChildCount(properties, "numFmt") > 1 || sectionChildCount(properties, "numStart") > 1 || sectionChildCount(properties, "numRestart") > 1 {
			return nil
		}
	}
	option := func(name string) *node {
		if local := sectionProperties.child(name); local != nil {
			return local
		}
		return documentProperties.child(name)
	}
	format := "decimal"
	if kind == "endnote" {
		format = "lowerRoman"
	}
	if n := option("numFmt"); n != nil {
		format = attribute(n, wordNS, "val")
		if attribute(n, wordNS, "format") != "" {
			return nil
		}
	}
	if n := option("numRestart"); n != nil && attribute(n, wordNS, "val") != "continuous" {
		return nil
	}
	const maxNoteNumber = 2147483647
	for id := range notes {
		value, err := strconv.ParseInt(id, 10, 32)
		if err != nil || value < 1 || id != strconv.FormatInt(value, 10) {
			return nil
		}
	}
	number := 1
	if n := option("numStart"); n != nil {
		start := previewNumber(attribute(n, wordNS, "val"), 1, maxNoteNumber)
		if start == nil {
			return nil
		}
		number = *start
	}
	labels := map[string]string{}
	usedIDs := map[int64]bool{}
	var visit func(*node) bool
	visit = func(n *node) bool {
		if n.is(kind + "Reference") {
			run := n.parent
			id := attribute(n, wordNS, "id")
			numericID, err := strconv.ParseInt(id, 10, 32)
			custom := attribute(n, wordNS, "customMarkFollows")
			if err != nil || numericID < 1 || usedIDs[numericID] || !notes[id] ||
				!run.is("r") || !run.parent.is("p") || run.parent.parent != body ||
				!(len(run.children) == 1 || len(run.children) == 2 && run.child("rPr") != nil) ||
				(custom != "" && custom != "0" && custom != "false" && custom != "off") || number > maxNoteNumber {
				return false
			}
			label := formatListNumber(number, format)
			if label == "" || format == "bullet" {
				return false
			}
			labels[id] = label
			usedIDs[numericID] = true
			number++
		}
		for _, child := range n.children {
			if !visit(child) {
				return false
			}
		}
		return true
	}
	if !visit(body) {
		return nil
	}
	return labels
}
