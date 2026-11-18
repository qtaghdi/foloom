package docx

import "fmt"

func markTrackableParagraphBreaks(root *node, paragraphs map[*node]*Paragraph, targets map[string]target) {
	body := root.child("body")
	var ranged map[*node]bool
	for index, source := range body.children {
		paragraph := paragraphs[source]
		if paragraph == nil || !paragraph.StructureEditable || paragraph.PageBreakBefore || paragraph.PageBreakAfter ||
			paragraph.List != nil || len(paragraph.Runs) != 1 || !paragraph.Runs[0].Trackable {
			continue
		}
		if index > 0 {
			previous := body.children[index-1].child("pPr").child("rPr")
			if previous.child("ins") != nil || previous.child("del") != nil || previous.child("moveFrom") != nil || previous.child("moveTo") != nil {
				continue
			}
		}
		run := &paragraph.Runs[0]
		t := targets[run.ID]
		if t.run == nil || !splittableTextRun(t.run) {
			continue
		}
		if props := source.child("pPr"); props != nil && (source.children[0] != props || !onlyNamespaceAttributes(props) || props.child("rPr") != nil) {
			continue
		}
		if ranged == nil {
			var valid bool
			ranged, valid = bodyRangeBlocks(body)
			if !valid || body.has("permStart") || body.has("permEnd") {
				return
			}
		}
		if !ranged[source] {
			run.BreakTrackable, t.trackableParagraphBreak = true, true
			targets[run.ID] = t
		}
	}
}

func trackParagraphBreak(data []byte, t target, op Operation, ids paragraphIDs, author, date string, revisionIDs map[int64]bool, nextID *int64) (replacement, error) {
	if !t.trackableParagraphBreak {
		return replacement{}, fmt.Errorf("protected DOCX tracked paragraph split")
	}
	marker := formatRevisionMarkup("ins", "", author, date, revisionIDs, nextID)
	runProps := `<dwr:rPr xmlns:dwr="` + wordNS + `">` + marker + `</dwr:rPr>`
	var leftProps []byte
	if props := t.paragraph.child("pPr"); props != nil {
		open, close := scopedOpening(props, data, false)
		content := ""
		if props.inner < props.endStart {
			content = string(data[props.inner:props.endStart])
		}
		leftProps = []byte(open + content + runProps + close)
	} else {
		leftProps = []byte(`<dwr:pPr xmlns:dwr="` + wordNS + `">` + runProps + `</dwr:pPr>`)
	}
	return splitParagraphText(data, t, op, ids, leftProps)
}
