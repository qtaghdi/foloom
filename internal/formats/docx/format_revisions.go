package docx

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func safeRunRevisionProperties(props, change *node) bool {
	if !onlyNamespaceAttributes(props) || strings.TrimSpace(props.text.String()) != "" {
		return false
	}
	seen := map[string]bool{}
	for _, child := range props.children {
		if child == change {
			continue
		}
		switch child.name.Local {
		case "rStyle", "rFonts", "b", "bCs", "i", "iCs", "strike", "dstrike", "color", "sz", "szCs",
			"highlight", "u", "vertAlign", "lang", "rtl", "cs":
		default:
			return false
		}
		if child.name.Space != wordNS || seen[child.name.Local] || len(child.children) != 0 || strings.TrimSpace(child.text.String()) != "" {
			return false
		}
		seen[child.name.Local] = true
		for _, attr := range child.attrs {
			if !isNamespaceAttribute(attr) && attr.Name.Space != wordNS {
				return false
			}
		}
	}
	return true
}

func runFormatRevision(run *node, data []byte) (change, previous *node) {
	props := run.child("rPr")
	change = props.child("rPrChange")
	if change == nil || len(run.children) != 2 || run.children[0] != props || !run.children[1].is("t") ||
		!trackableRun(run, run.children[1:], data) || props.children[len(props.children)-1] != change ||
		len(change.children) != 1 || strings.TrimSpace(change.text.String()) != "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(change.attr("id"), 10, 32)
	if err != nil || id < 0 {
		return nil, nil
	}
	for _, attr := range change.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS ||
			attr.Name.Local != "id" && attr.Name.Local != "author" && attr.Name.Local != "date") {
			return nil, nil
		}
	}
	previous = change.children[0]
	if !previous.is("rPr") || !safeRunRevisionProperties(props, change) || !safeRunRevisionProperties(previous, nil) {
		return nil, nil
	}
	return change, previous
}

func formatRevisionReplacement(data []byte, t target, operation string) (replacement, error) {
	change, previous := runFormatRevision(t.run, data)
	if change == nil {
		return replacement{}, fmt.Errorf("protected DOCX run format revision")
	}
	if operation == "accept_revision" {
		return replacement{start: change.start, end: change.end}, nil
	}
	open, close := scopedOpening(previous, data, false)
	inner := ""
	if previous.inner < previous.endStart {
		inner = string(data[previous.inner:previous.endStart])
	}
	props := change.parent
	return replacement{start: props.start, end: props.end, data: []byte(open + inner + close)}, nil
}

func formatRevisionPreview(change *node, text string, editable bool) *ObjectInfo {
	date := ""
	if value, err := time.Parse(time.RFC3339Nano, change.attr("date")); err == nil {
		date = value.UTC().Format(time.RFC3339)
	}
	return &ObjectInfo{Kind: "tracked_format", Text: reviewText(text, 1024), Author: reviewText(change.attr("author"), 160), Date: date, Editable: editable}
}
