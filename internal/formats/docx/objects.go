package docx

import (
	"fmt"
	"strings"
)

const mathNS = "http://schemas.openxmlformats.org/officeDocument/2006/math"

type MathEdit struct {
	Action     string        `json:"action" jsonschema:"insert, insert_fraction, replace, or delete"`
	Expression string        `json:"expression,omitempty"`
	Fraction   *MathFraction `json:"fraction,omitempty"`
}

type MathFraction struct {
	Numerator   string `json:"numerator"`
	Denominator string `json:"denominator"`
}

type ObjectInfo struct {
	Kind     string        `json:"kind"`
	Text     string        `json:"text,omitempty"`
	Editable bool          `json:"editable"`
	Fraction *MathFraction `json:"fraction,omitempty"`
}

func editableMath(math *node) bool {
	if math.name.Space != mathNS || math.name.Local != "oMath" || !onlyNamespaceAttributes(math) || len(math.children) != 1 ||
		strings.TrimSpace(math.text.String()) != "" {
		return false
	}
	run := childNS(math, mathNS, "r")
	if run == nil || !onlyNamespaceAttributes(run) || len(run.children) != 1 || strings.TrimSpace(run.text.String()) != "" {
		return false
	}
	text := childNS(run, mathNS, "t")
	if text == nil || len(text.children) != 0 {
		return false
	}
	for _, a := range text.attrs {
		if !isNamespaceAttribute(a) && (a.Name.Space != "http://www.w3.org/XML/1998/namespace" || a.Name.Local != "space") {
			return false
		}
	}
	return true
}

func previewFraction(math *node) *MathFraction {
	if math.name.Local != "oMath" || len(math.children) != 1 {
		return nil
	}
	fraction := childNS(math, mathNS, "f")
	if fraction == nil || len(fraction.children) != 2 {
		return nil
	}
	read := func(name string) string {
		arg := childNS(fraction, mathNS, name)
		if arg == nil || len(arg.children) != 1 {
			return ""
		}
		run := childNS(arg, mathNS, "r")
		if run == nil || len(run.children) != 1 {
			return ""
		}
		text := childNS(run, mathNS, "t")
		if text == nil || len(text.children) != 0 {
			return ""
		}
		return reviewText(text.text.String(), 1024)
	}
	numerator, denominator := read("num"), read("den")
	if strings.TrimSpace(numerator) == "" || strings.TrimSpace(denominator) == "" {
		return nil
	}
	return &MathFraction{Numerator: numerator, Denominator: denominator}
}

func objectPreview(n *node) *ObjectInfo {
	switch {
	case n.name.Space == mathNS && (n.name.Local == "oMath" || n.name.Local == "oMathPara"):
		out := &ObjectInfo{Kind: "math", Editable: editableMath(n), Fraction: previewFraction(n)}
		if out.Editable {
			out.Text = reviewText(childNS(childNS(n, mathNS, "r"), mathNS, "t").text.String(), 1024)
		}
		return out
	case n.is("sdt"):
		return &ObjectInfo{Kind: "control", Text: reviewText(textContent(n), 1024)}
	case n.is("object") || n.child("object") != nil:
		return &ObjectInfo{Kind: "embedded"}
	case n.is("pict") || n.child("pict") != nil:
		if n.has("txbxContent") {
			return &ObjectInfo{Kind: "textbox", Text: reviewText(textContent(n), 1024)}
		}
		return &ObjectInfo{Kind: "shape"}
	}
	drawing := n.child("drawing")
	if drawing == nil {
		return nil
	}
	container := childNS(drawing, drawingWordNS, "inline")
	if container == nil {
		container = childNS(drawing, drawingWordNS, "anchor")
	}
	graphic := childNS(childNS(container, drawingNS, "graphic"), drawingNS, "graphicData")
	switch attribute(graphic, "", "uri") {
	case drawingNS + "/chart", "http://schemas.openxmlformats.org/drawingml/2006/chart":
		return &ObjectInfo{Kind: "chart"}
	case drawingNS + "/diagram", "http://schemas.openxmlformats.org/drawingml/2006/diagram":
		return &ObjectInfo{Kind: "smartart"}
	case "http://schemas.microsoft.com/office/word/2010/wordprocessingShape", "http://schemas.microsoft.com/office/word/2010/wordprocessingGroup":
		if drawing.has("txbxContent") {
			return &ObjectInfo{Kind: "textbox", Text: reviewText(textContent(drawing), 1024)}
		}
		return &ObjectInfo{Kind: "shape"}
	case pictureNS:
		return nil
	default:
		return &ObjectInfo{Kind: "drawing"}
	}
}

func mathReplacement(data []byte, t target, edit MathEdit) (replacement, error) {
	if edit.Action != "insert_fraction" && edit.Fraction != nil {
		return replacement{}, fmt.Errorf("unexpected DOCX fraction fields")
	}
	if edit.Action == "delete" {
		if t.kind != "math_object" || !editableMath(t.node) || strings.Contains(string(data[t.node.start:t.node.end]), "<!--") || edit.Expression != "" {
			return replacement{}, fmt.Errorf("invalid DOCX equation removal")
		}
		return replacement{t.node.start, t.node.end, nil}, nil
	}
	mathRun := func(text string) string {
		return `<dwm:r><dwm:t xml:space="preserve">` + escapeText(text) + `</dwm:t></dwm:r>`
	}
	validExpression := func(text string) bool {
		return validText(text) && len(text) <= 1024 && strings.TrimSpace(text) != ""
	}
	var contents string
	if edit.Action == "insert_fraction" {
		fraction := edit.Fraction
		if edit.Expression != "" || fraction == nil || !validExpression(fraction.Numerator) || !validExpression(fraction.Denominator) {
			return replacement{}, fmt.Errorf("invalid DOCX fraction")
		}
		contents = `<dwm:f><dwm:num>` + mathRun(fraction.Numerator) + `</dwm:num><dwm:den>` + mathRun(fraction.Denominator) + `</dwm:den></dwm:f>`
	} else {
		if !validExpression(edit.Expression) {
			return replacement{}, fmt.Errorf("invalid DOCX equation text")
		}
		contents = mathRun(edit.Expression)
	}
	raw := `<dwm:oMath xmlns:dwm="` + mathNS + `">` + contents + `</dwm:oMath>`
	switch edit.Action {
	case "replace":
		if t.kind != "math_object" || !editableMath(t.node) || strings.Contains(string(data[t.node.start:t.node.end]), "<!--") {
			return replacement{}, fmt.Errorf("protected DOCX equation")
		}
		return replacement{t.node.start, t.node.end, []byte(raw)}, nil
	case "insert", "insert_fraction":
		p := t.paragraph
		if t.kind != "replace_paragraphs" || p == nil {
			return replacement{}, fmt.Errorf("DOCX equation requires an editable paragraph")
		}
		if strings.HasSuffix(string(data[p.start:p.inner]), "/>") {
			open, close := scopedOpening(p, data, false)
			return replacement{p.start, p.end, []byte(open + raw + close)}, nil
		}
		return replacement{p.endStart, p.endStart, []byte(raw)}, nil
	default:
		return replacement{}, fmt.Errorf("unsupported DOCX equation operation")
	}
}
