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
	return splitParagraphText(data, t, op, ids, trackedParagraphMarkProperties(data, t.paragraph, marker))
}

func trackParagraphMerge(data []byte, t target, op Operation, author, date string, revisionIDs map[int64]bool, nextID *int64) (replacement, error) {
	if !t.trackableParagraphMerge || op.ExpectedText != "" || op.Text != "" {
		return replacement{}, fmt.Errorf("protected DOCX tracked paragraph merge or unexpected text")
	}
	marker := formatRevisionMarkup("del", "", author, date, revisionIDs, nextID)
	props := t.paragraph.child("pPr")
	start, end := t.paragraph.inner, t.paragraph.inner
	if props != nil {
		start, end = props.start, props.end
	}
	return replacement{start: start, end: end, data: trackedParagraphMarkProperties(data, t.paragraph, marker)}, nil
}

func trackedParagraphMarkProperties(data []byte, paragraph *node, marker string) []byte {
	props := paragraph.child("pPr")
	if props == nil {
		return []byte(`<dwr:pPr xmlns:dwr="` + wordNS + `"><dwr:rPr>` + marker + `</dwr:rPr></dwr:pPr>`)
	}
	if runProps := props.child("rPr"); runProps != nil {
		open, close := scopedOpening(runProps, data, false)
		return []byte(string(data[props.start:runProps.start]) + open + string(data[runProps.inner:runProps.endStart]) + marker + close + string(data[runProps.end:props.end]))
	}
	open, close := scopedOpening(props, data, false)
	runProps := `<dwr:rPr xmlns:dwr="` + wordNS + `">` + marker + `</dwr:rPr>`
	return []byte(open + string(data[props.inner:props.endStart]) + runProps + close)
}
