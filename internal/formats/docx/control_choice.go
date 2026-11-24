package docx

import (
	"fmt"
	"strings"
)

type ControlChoicePreview struct {
	Value   string   `json:"value"`
	Options []string `json:"options"`
}

type controlChoiceInfo struct {
	ControlChoicePreview
	values        map[string]string
	list, display *node
}

func validControlChoiceText(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= 256 && validText(value)
}

func choiceControlNodes(control *node, data []byte) *controlChoiceInfo {
	if !control.is("sdt") || strings.TrimSpace(control.text.String()) != "" || strings.Contains(string(data[control.start:control.end]), "<!--") {
		return nil
	}
	props, content := control.child("sdtPr"), control.child("sdtContent")
	if props == nil || content == nil || countChild(control, "sdtPr") != 1 || countChild(control, "sdtContent") != 1 ||
		len(control.children) != 2+countChild(control, "sdtEndPr") || countChild(control, "sdtEndPr") > 1 ||
		strings.TrimSpace(content.text.String()) != "" || strings.TrimSpace(props.text.String()) != "" ||
		countChild(props, "dropDownList") != 1 || countChild(props, "lock") > 1 {
		return nil
	}
	for _, item := range props.children {
		switch {
		case item.is("rPr") || item.is("alias") || item.is("tag") || item.is("id") || item.is("placeholder") || item.is("dropDownList"):
		case item.is("lock") && item.attr("val") == "unlocked":
		default:
			return nil
		}
	}
	list := props.child("dropDownList")
	if strings.TrimSpace(list.text.String()) != "" || len(list.children) < 2 || len(list.children) > 32 {
		return nil
	}
	lastValuePresent := false
	for _, attr := range list.attrs {
		if isNamespaceAttribute(attr) {
			continue
		}
		if attr.Name.Space != wordNS || attr.Name.Local != "lastValue" || lastValuePresent {
			return nil
		}
		lastValuePresent = true
	}
	texts, cached := plainTextRunNodes(content.children)
	if len(texts) != 1 {
		return nil
	}
	info := &controlChoiceInfo{ControlChoicePreview: ControlChoicePreview{Value: cached}, values: map[string]string{}, list: list, display: texts[0]}
	seenValues := map[string]bool{}
	for _, item := range list.children {
		if !item.is("listItem") || len(item.children) != 0 || strings.TrimSpace(item.text.String()) != "" {
			return nil
		}
		label, value := item.attr("displayText"), item.attr("value")
		if !validControlChoiceText(label) || !validControlChoiceText(value) || info.values[label] != "" || seenValues[value] {
			return nil
		}
		for _, attr := range item.attrs {
			if !isNamespaceAttribute(attr) && (attr.Name.Space != wordNS || attr.Name.Local != "displayText" && attr.Name.Local != "value") {
				return nil
			}
		}
		info.values[label] = value
		seenValues[value] = true
		info.Options = append(info.Options, label)
	}
	if info.values[cached] == "" || lastValuePresent && list.attr("lastValue") != info.values[cached] {
		return nil
	}
	return info
}

func (p *documentPackage) updateControlChoice(t target, expected, value string) ([]replacement, error) {
	if t.kind != "update_control_choice" || expected != t.text || expected == value {
		return nil, fmt.Errorf("DOCX dropdown requires its current display value and a different option")
	}
	_, _, settings, settingsErr := p.pageSettings(nil)
	ranges, valid := contentControlRanges(t.paragraph.parent)
	if !allowControlEditsInSettings(settings, settingsErr) || !valid || ranges[t.paragraph] {
		return nil, fmt.Errorf("protected DOCX dropdown content control")
	}
	info := choiceControlNodes(t.node, p.document)
	if info == nil || info.values[value] == "" {
		return nil, fmt.Errorf("unsupported DOCX dropdown choice")
	}
	opening, _ := scopedOpening(info.list, p.document, false)
	opening, err := patchOpeningAttributes(opening, wordNS, map[string]string{"lastValue": info.values[value]}, nil)
	if err != nil {
		return nil, err
	}
	edit, err := replaceTextNode(p.document, info.display, value)
	if err != nil {
		return nil, err
	}
	return []replacement{{start: info.list.start, end: info.list.inner, data: []byte(opening)}, edit}, nil
}
