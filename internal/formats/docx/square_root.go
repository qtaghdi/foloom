package docx

import "strings"

func squareRootText(math *node) *node {
	plain := func(n *node) bool {
		return n != nil && onlyNamespaceAttributes(n) && strings.TrimSpace(n.text.String()) == ""
	}
	if !plain(math) || math.name.Space != mathNS || math.name.Local != "oMath" || len(math.children) != 1 {
		return nil
	}
	rad := childNS(math, mathNS, "rad")
	if !plain(rad) || len(rad.children) != 3 {
		return nil
	}
	props, degree, arg := childNS(rad, mathNS, "radPr"), childNS(rad, mathNS, "deg"), childNS(rad, mathNS, "e")
	if !plain(props) || !plain(degree) || !plain(arg) || len(props.children) != 1 ||
		len(degree.children) != 0 || len(arg.children) != 1 || rad.children[0] != props || rad.children[1] != degree || rad.children[2] != arg {
		return nil
	}
	hide := childNS(props, mathNS, "degHide")
	flag := attribute(hide, mathNS, "val")
	if hide == nil || len(hide.children) != 0 || strings.TrimSpace(hide.text.String()) != "" ||
		flag != "" && flag != "1" && flag != "true" && flag != "on" {
		return nil
	}
	for _, attr := range hide.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != mathNS || attr.Name.Local != "val") {
			return nil
		}
	}
	run := childNS(arg, mathNS, "r")
	text := childNS(run, mathNS, "t")
	if !plain(run) || text == nil || len(text.children) != 0 || !plainTextAttributes(text) || !validMathText(text.text.String()) {
		return nil
	}
	for _, child := range run.children {
		if child == text || child.name.Space == mathNS && child.name.Local == "rPr" || child.is("rPr") && !child.has("rPrChange") {
			continue
		}
		return nil
	}
	return text
}
