package docx

import "strings"

func plainControlText(control *node, data []byte) *node {
	if !control.is("sdt") || strings.TrimSpace(control.text.String()) != "" ||
		strings.Contains(string(data[control.start:control.end]), "<!--") {
		return nil
	}
	props, content := control.child("sdtPr"), control.child("sdtContent")
	count := 2
	if control.child("sdtEndPr") != nil {
		count++
	}
	if props == nil || content == nil || len(control.children) != count || len(content.children) != 1 ||
		strings.TrimSpace(content.text.String()) != "" || countChild(props, "text") != 1 || countChild(props, "lock") > 1 {
		return nil
	}
	for _, item := range props.children {
		switch {
		case item.is("rPr") || item.is("alias") || item.is("tag") || item.is("id") || item.is("placeholder") || item.is("text"):
		case item.is("lock") && (item.attr("val") == "unlocked" || item.attr("val") == "sdtLocked"):
		case item.name.Space == "http://schemas.microsoft.com/office/word/2012/wordml" &&
			(item.name.Local == "appearance" || item.name.Local == "color"):
		default:
			return nil
		}
	}
	run := content.children[0]
	if !run.is("r") || strings.TrimSpace(run.text.String()) != "" || run.has("rPrChange") {
		return nil
	}
	text := run.child("t")
	if text == nil || len(text.children) != 0 || !plainTextAttributes(text) ||
		len(run.children) != 1 && (len(run.children) != 2 || run.child("rPr") == nil) ||
		len(text.text.String()) > 1024 || !validText(text.text.String()) {
		return nil
	}
	return text
}
