package docx

import (
	"fmt"
	"strings"
	"time"
)

const controlDateFormat = "yyyy-MM-dd"
const controlDateSuffix = "T00:00:00Z"

type ControlDatePreview struct {
	Value  string `json:"value"`
	Format string `json:"format"`
}

func validControlDate(value string) bool {
	if len(value) != len(time.DateOnly) {
		return false
	}
	date, err := time.Parse(time.DateOnly, value)
	return err == nil && date.Year() > 0 && date.Format(time.DateOnly) == value
}

func dateControlNodes(control *node, data []byte) (property, display *node) {
	if !control.is("sdt") || strings.TrimSpace(control.text.String()) != "" ||
		strings.Contains(string(data[control.start:control.end]), "<!--") {
		return nil, nil
	}
	props, content := control.child("sdtPr"), control.child("sdtContent")
	if props == nil || content == nil || countChild(control, "sdtPr") != 1 || countChild(control, "sdtContent") != 1 ||
		len(control.children) != 2+countChild(control, "sdtEndPr") || countChild(control, "sdtEndPr") > 1 ||
		strings.TrimSpace(content.text.String()) != "" || strings.TrimSpace(props.text.String()) != "" ||
		countChild(props, "date") != 1 || countChild(props, "lock") > 1 {
		return nil, nil
	}
	for _, item := range props.children {
		switch {
		case item.is("rPr") || item.is("alias") || item.is("tag") || item.is("id") || item.is("placeholder") || item.is("date"):
		case item.is("lock") && item.attr("val") == "unlocked":
		default:
			return nil, nil
		}
	}
	date := props.child("date")
	value, canonical := strings.CutSuffix(date.attr("fullDate"), controlDateSuffix)
	if !canonical || !validControlDate(value) || !supportedControlDateProperties(date) {
		return nil, nil
	}
	texts, cached := plainTextRunNodes(content.children)
	// LibreOffice may add a formatting-only run before the one value run.
	if len(texts) != 1 || cached != value {
		return nil, nil
	}
	return date, texts[0]
}

func supportedControlDateProperties(date *node) bool {
	if strings.TrimSpace(date.text.String()) != "" || countChild(date, "dateFormat") != 1 {
		return false
	}
	for _, attr := range date.attrs {
		if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || attr.Name.Local != "fullDate") {
			return false
		}
	}
	seen := map[string]bool{}
	for _, item := range date.children {
		key, value := item.name.Local, item.attr("val")
		if item.name.Space != wordNS || seen[key] || len(item.children) != 0 || strings.TrimSpace(item.text.String()) != "" {
			return false
		}
		for _, attr := range item.attrs {
			if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || attr.Name.Local != "val") {
				return false
			}
		}
		seen[key] = true
		switch {
		case key == "dateFormat" && value == controlDateFormat:
		case key == "lid" && value == "en-US":
		case key == "calendar" && value == "gregorian":
		case key == "storeMappedDataAs" && (value == "date" || value == "dateTime"):
		default:
			return false
		}
	}
	return true
}

func (p *documentPackage) updateControlDate(t target, expected, value string) ([]replacement, error) {
	if t.kind != "update_control_date" || expected != t.text || value == expected || !validControlDate(value) {
		return nil, fmt.Errorf("DOCX date control requires its current ISO date and a different valid date")
	}
	_, _, settings, err := p.pageSettings(nil)
	if !allowControlEditsInSettings(settings, err) {
		return nil, fmt.Errorf("protected DOCX date content control")
	}
	ranges, valid := contentControlRanges(t.paragraph.parent)
	if !valid || ranges[t.paragraph] {
		return nil, fmt.Errorf("protected DOCX date content control")
	}
	date, display := dateControlNodes(t.node, p.document)
	if date == nil {
		return nil, fmt.Errorf("unsupported DOCX date content control")
	}
	opening, _ := scopedOpening(date, p.document, false)
	opening, err = patchOpeningAttributes(opening, wordNS, map[string]string{"fullDate": value + controlDateSuffix}, nil)
	if err != nil {
		return nil, err
	}
	return []replacement{
		{start: date.start, end: date.inner, data: []byte(opening)},
		{start: display.inner, end: display.endStart, data: []byte(value)},
	}, nil
}
