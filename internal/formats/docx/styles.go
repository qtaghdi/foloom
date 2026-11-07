package docx

type previewStyles struct {
	defaults  *node
	byID      map[string]*node
	paragraph string
}

func readPreviewStyles(root *node) previewStyles {
	if root == nil || !root.is("styles") {
		return previewStyles{}
	}
	s := previewStyles{defaults: root.child("docDefaults"), byID: map[string]*node{}}
	for _, style := range root.children {
		if id := style.attr("styleId"); style.is("style") && id != "" {
			s.byID[id] = style
			if style.attr("type") == "paragraph" && onOff(style.attr("default"), false) {
				s.paragraph = id
			}
		}
	}
	return s
}

func onOff(value string, empty bool) bool {
	switch value {
	case "1", "true", "on":
		return true
	case "":
		return empty
	default:
		return false
	}
}

// Bound inheritance even for malformed files with cycles or very long chains.
func (s previewStyles) chain(id, kind string) []*node {
	var chain []*node
	for len(chain) < 32 {
		style := s.byID[id]
		if style == nil || style.attr("type") != kind {
			return chain
		}
		for _, ancestor := range chain {
			if ancestor == style {
				return nil
			}
		}
		chain = append(chain, style)
		id = style.child("basedOn").attr("val")
	}
	return chain
}

func styleRun(run *Run, props *node) {
	strike := run.Strike
	applyRunPreview(run, props)
	if props.child("strike") != nil || props.child("dstrike") != nil {
		run.Strike = strike
		for _, name := range []string{"strike", "dstrike"} {
			if n := props.child(name); n != nil && onOff(n.attr("val"), true) {
				run.Strike = !run.Strike
			}
		}
	}
	if b := props.child("b"); b != nil && onOff(b.attr("val"), true) {
		run.Bold = !run.Bold
	}
	if i := props.child("i"); i != nil && onOff(i.attr("val"), true) {
		run.Italic = !run.Italic
	}
}

func directRun(run *Run, props *node) {
	applyRunPreview(run, props)
	if b := props.child("b"); b != nil {
		run.Bold = onOff(b.attr("val"), true)
	}
	if i := props.child("i"); i != nil {
		run.Italic = onOff(i.attr("val"), true)
	}
}
