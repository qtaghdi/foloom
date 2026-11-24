package docx

import (
	"fmt"
	"strings"
)

type MathNested struct {
	Kind        string `json:"kind" jsonschema:"root_fraction, fraction_root_numerator, or fraction_root_denominator"`
	Numerator   string `json:"numerator"`
	Denominator string `json:"denominator"`
}

type MathNestedLeafEdit struct {
	Leaf         string `json:"leaf" jsonschema:"numerator or denominator"`
	ExpectedText string `json:"expectedText"`
	Text         string `json:"text"`
}

func nestedFractionNodes(fraction *node) (*node, *node) {
	if !plainMatrixElement(fraction, "f", 2) {
		return nil, nil
	}
	num, den := fraction.children[0], fraction.children[1]
	if !plainMatrixElement(num, "num", 1) || !plainMatrixElement(den, "den", 1) {
		return nil, nil
	}
	return num.children[0], den.children[0]
}

func nestedRootNode(rad *node) *node {
	if !plainMatrixElement(rad, "rad", 3) {
		return nil
	}
	props, degree, arg := rad.children[0], rad.children[1], rad.children[2]
	if !plainMatrixElement(props, "radPr", 1) || !plainMatrixElement(degree, "deg", 0) || !plainMatrixElement(arg, "e", 1) {
		return nil
	}
	hide := props.children[0]
	if hide.name.Space != mathNS || hide.name.Local != "degHide" || len(hide.children) != 0 || strings.TrimSpace(hide.text.String()) != "" {
		return nil
	}
	flag := attribute(hide, mathNS, "val")
	if flag != "1" && flag != "true" && flag != "on" && flag != "" {
		return nil
	}
	for _, attr := range hide.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != mathNS || attr.Name.Local != "val") {
			return nil
		}
	}
	return arg.children[0]
}

func nestedLeaf(run *node) *node {
	if !plainMatrixElement(run, "r", 1) {
		return nil
	}
	text := childNS(run, mathNS, "t")
	if text == nil || len(text.children) != 0 || !plainTextAttributes(text) || !validMathText(text.text.String()) {
		return nil
	}
	return text
}

func nestedMathNodes(math *node) (kind string, numerator, denominator *node) {
	if !plainMatrixElement(math, "oMath", 1) {
		return "", nil, nil
	}
	outer := math.children[0]
	if inner := nestedRootNode(outer); inner != nil {
		num, den := nestedFractionNodes(inner)
		return "root_fraction", nestedLeaf(num), nestedLeaf(den)
	}
	num, den := nestedFractionNodes(outer)
	if inner := nestedRootNode(num); inner != nil {
		return "fraction_root_numerator", nestedLeaf(inner), nestedLeaf(den)
	}
	if inner := nestedRootNode(den); inner != nil {
		return "fraction_root_denominator", nestedLeaf(num), nestedLeaf(inner)
	}
	return "", nil, nil
}

func nestedMathPreview(math *node) *MathNested {
	kind, num, den := nestedMathNodes(math)
	if num == nil || den == nil {
		return nil
	}
	return &MathNested{Kind: kind, Numerator: num.text.String(), Denominator: den.text.String()}
}

func nestedMathContents(value *MathNested) (string, error) {
	if value == nil || !validMathText(value.Numerator) || !validMathText(value.Denominator) {
		return "", fmt.Errorf("invalid DOCX nested math values")
	}
	root := func(expression string) string {
		return `<dwm:rad><dwm:radPr><dwm:degHide dwm:val="1"/></dwm:radPr><dwm:deg/><dwm:e>` + expression + `</dwm:e></dwm:rad>`
	}
	fraction := func(num, den string) string {
		return `<dwm:f><dwm:num>` + num + `</dwm:num><dwm:den>` + den + `</dwm:den></dwm:f>`
	}
	num, den := mathTextRun(value.Numerator), mathTextRun(value.Denominator)
	switch value.Kind {
	case "root_fraction":
		return root(fraction(num, den)), nil
	case "fraction_root_numerator":
		return fraction(root(num), den), nil
	case "fraction_root_denominator":
		return fraction(num, root(den)), nil
	}
	return "", fmt.Errorf("unsupported DOCX nested math kind")
}

func nestedLeafReplacement(data []byte, t target, edit *MathNestedLeafEdit) (replacement, error) {
	if t.kind != "math_object" || edit == nil || !validMathText(edit.Text) || edit.Text == edit.ExpectedText || strings.Contains(string(data[t.node.start:t.node.end]), "<!--") {
		return replacement{}, fmt.Errorf("invalid DOCX nested math edit")
	}
	_, num, den := nestedMathNodes(t.node)
	if num == nil || den == nil {
		return replacement{}, fmt.Errorf("protected DOCX nested math structure")
	}
	var leaf *node
	switch edit.Leaf {
	case "numerator":
		leaf = num
	case "denominator":
		leaf = den
	default:
		return replacement{}, fmt.Errorf("invalid DOCX nested math leaf")
	}
	if leaf.text.String() != edit.ExpectedText {
		return replacement{}, fmt.Errorf("stale DOCX nested math leaf")
	}
	return replaceTextNode(data, leaf, edit.Text)
}
