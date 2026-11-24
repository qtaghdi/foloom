package docx

import (
	"bytes"
	"fmt"
	"strings"
)

func allowControlEditsInSettings(settings *node, err error) bool {
	if err != nil || countChild(settings, "documentProtection") > 1 || settings.child("writeProtection") != nil {
		return false
	}
	lock := settings.child("documentProtection")
	if lock == nil {
		return true
	}
	switch lock.attr("enforcement") {
	case "0", "false", "off":
		return true
	default:
		return false
	}
}

func contentControlRanges(body *node) (map[*node]bool, bool) {
	if body.has("permStart") || body.has("permEnd") {
		return nil, false
	}
	return bodyRangeBlocks(body)
}

func canUnwrapControl(t target, ranges map[*node]bool) bool {
	if t.kind != "update_control_text" || ranges[t.paragraph] {
		return false
	}
	control := t.node
	props, content := control.child("sdtPr"), control.child("sdtContent")
	if !onlyNamespaceAttributes(control) || !onlyNamespaceAttributes(content) {
		return false
	}
	if lock := props.child("lock"); lock != nil && lock.attr("val") != "unlocked" {
		return false
	}
	// Rule out styling inherited through a wrapper rather than stored on the
	// runs themselves. These properties have no equivalent on a plain run.
	for _, property := range []*node{props.child("rPr"), control.child("sdtEndPr")} {
		if property != nil && (len(property.children) != 0 || !onlyNamespaceAttributes(property) || strings.TrimSpace(property.text.String()) != "") {
			return false
		}
	}
	return countChild(props, "rPr") <= 1
}

func projectRestrictedControlEdits(root *node, blocks []Block, targets map[string]target, settings *node, settingsErr error) {
	body := root.child("body")
	if !body.has("sdt") || !allowControlEditsInSettings(settings, settingsErr) {
		return
	}
	ranges, valid := contentControlRanges(body)
	if !valid {
		return
	}
	for _, block := range blocks {
		if block.Paragraph == nil {
			continue
		}
		for _, run := range block.Paragraph.Runs {
			t := targets[run.ID]
			if canUnwrapControl(t, ranges) {
				run.Object.UnwrapEditable = true
			}
			if t.kind == "update_control_date" && !ranges[t.paragraph] {
				run.Object.Editable = true
				run.Object.DateControl = &ControlDatePreview{Value: t.text, Format: controlDateFormat}
				run.Object.Label = reviewText(t.node.child("sdtPr").child("alias").attr("val"), 160)
			}
			if t.kind == "update_control_choice" && !ranges[t.paragraph] {
				if choice := choiceControlNodes(t.node, root.source); choice != nil {
					run.Object.Editable = true
					run.Object.ChoiceControl = &choice.ControlChoicePreview
					run.Object.Label = reviewText(t.node.child("sdtPr").child("alias").attr("val"), 160)
				}
			}
		}
	}
}

func (p *documentPackage) unwrapControl(t target) (replacement, error) {
	if t.kind != "update_control_text" {
		return replacement{}, fmt.Errorf("protected or unsupported DOCX content control")
	}
	_, _, settings, err := p.pageSettings(nil)
	if !allowControlEditsInSettings(settings, err) {
		return replacement{}, fmt.Errorf("protected or unsupported DOCX content control")
	}
	ranges, valid := contentControlRanges(t.paragraph.parent)
	if !valid || !canUnwrapControl(t, ranges) {
		return replacement{}, fmt.Errorf("protected or unsupported DOCX content control")
	}
	control := t.node
	content := control.child("sdtContent")
	payload := p.document[content.inner:content.endStart]
	if len(control.attrs) != 0 || len(content.attrs) != 0 {
		// Scope lost with either wrapper must follow the existing runs. The
		// original paragraph remains the parent and retains its own bindings.
		var lifted bytes.Buffer
		at := content.inner
		for _, run := range content.children {
			lifted.Write(p.document[at:run.start])
			open, close := scopedOpeningWithin(run, t.paragraph, p.document, false)
			if lifted.Len()+len(open)+len(close)+run.endStart-run.inner > maxPartBytes {
				return replacement{}, fmt.Errorf("DOCX content control replacement exceeds 16 MiB")
			}
			if run.inner == run.end {
				lifted.WriteString(strings.TrimSuffix(open, ">") + "/>")
			} else {
				lifted.WriteString(open)
				lifted.Write(p.document[run.inner:run.endStart])
				lifted.WriteString(close)
			}
			at = run.end
		}
		lifted.Write(p.document[at:content.endStart])
		payload = lifted.Bytes()
	}
	return replacement{start: control.start, end: control.end, data: payload}, nil
}
