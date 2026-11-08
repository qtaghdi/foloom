package docx

import (
	"fmt"
	"strings"
)

const mathNS = "http://schemas.openxmlformats.org/officeDocument/2006/math"

type MathEdit struct {
	Action     string `json:"action" jsonschema:"insert, replace, or delete"`
	Expression string `json:"expression,omitempty"`
}

type ObjectInfo struct {
	Kind     string `json:"kind"`
	Text     string `json:"text,omitempty"`
	Editable bool   `json:"editable"`
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

func objectPreview(n *node) *ObjectInfo {
	switch {
	case n.name.Space == mathNS && (n.name.Local == "oMath" || n.name.Local == "oMathPara"):
		out := &ObjectInfo{Kind: "math", Editable: editableMath(n)}
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
	if edit.Action == "delete" {
		if t.kind != "math_object" || !editableMath(t.node) || strings.Contains(string(data[t.node.start:t.node.end]), "<!--") || edit.Expression != "" {
			return replacement{}, fmt.Errorf("invalid DOCX equation removal")
		}
		return replacement{t.node.start, t.node.end, nil}, nil
	}
	if !validText(edit.Expression) || len(edit.Expression) > 1024 || strings.TrimSpace(edit.Expression) == "" {
		return replacement{}, fmt.Errorf("invalid DOCX equation text")
	}
	raw := `<dwm:oMath xmlns:dwm="` + mathNS + `"><dwm:r><dwm:t xml:space="preserve">` + escapeText(edit.Expression) + `</dwm:t></dwm:r></dwm:oMath>`
	switch edit.Action {
	case "replace":
		if t.kind != "math_object" || !editableMath(t.node) || strings.Contains(string(data[t.node.start:t.node.end]), "<!--") {
			return replacement{}, fmt.Errorf("protected DOCX equation")
		}
		return replacement{t.node.start, t.node.end, []byte(raw)}, nil
	case "insert":
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
