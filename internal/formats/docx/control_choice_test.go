package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

const sampleChoiceControl = `<w:sdt><w:sdtPr><w:alias w:val="Status"/><w:id w:val="42"/><w:dropDownList w:lastValue="todo"><w:listItem w:displayText="대기" w:value="todo"/><w:listItem w:displayText="완료 &amp; 검토" w:value="done"/></w:dropDownList></w:sdtPr><w:sdtContent><w:r><w:rPr><w:b/></w:rPr><w:t>대기</w:t></w:r></w:sdtContent></w:sdt>`

func TestDropdownChoicePreservesNativeListAndHistory(t *testing.T) {
	ctx := context.Background()
	s, file := withDocument(t, bodyXML(`<w:p><w:r><w:t>Prefix</w:t></w:r>`+sampleChoiceControl+`<w:r><w:t>Suffix</w:t></w:r></w:p>`))
	v := mustSnapshot(t, s)
	original := fileBytes(t, file)
	run := v.Blocks[0].Paragraph.Runs[1]
	if run.Object == nil || !run.Object.Editable || run.Object.ChoiceControl == nil || run.Object.ChoiceControl.Value != "대기" || len(run.Object.ChoiceControl.Options) != 2 {
		t.Fatal("missing supported dropdown projection")
	}
	op := Operation{Type: "update_control_choice", TargetID: run.ID, ExpectedText: "대기", Text: "완료 & 검토"}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}
	for _, bad := range []Operation{{Type: op.Type, TargetID: run.ID, ExpectedText: "stale", Text: op.Text}, {Type: op.Type, TargetID: run.ID, ExpectedText: op.ExpectedText, Text: "missing"}, {Type: "update_control_text", TargetID: run.ID, ExpectedText: op.ExpectedText, Text: op.Text}} {
		invalid := input
		invalid.Operations = []Operation{bad}
		if _, err := s.Apply(ctx, invalid, "human"); err == nil || !bytes.Equal(original, fileBytes(t, file)) {
			t.Fatal("invalid selection changed source")
		}
	}
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	out := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[1].Object
	if out == nil || out.ChoiceControl == nil || out.ChoiceControl.Value != op.Text || len(out.ChoiceControl.Options) != 2 || out.Label != "Status" {
		t.Fatal("selection did not survive reopening")
	}
	root, err := parseXML(reopened.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	control := root.child("body").child("p").child("sdt")
	info := choiceControlNodes(control, reopened.pkg.document)
	if info == nil || info.list.attr("lastValue") != "done" || info.display.parent.child("rPr").child("b") == nil || control.child("sdtPr").child("id").attr("val") != "42" {
		t.Fatal("native list ID, selection or styling changed incorrectly")
	}
	if !bytes.Contains(reopened.pkg.document, []byte(`<w:listItem w:displayText="대기" w:value="todo"/><w:listItem w:displayText="완료 &amp; 검토" w:value="done"/>`)) {
		t.Fatal("list items changed")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, file)) {
		t.Fatal("undo did not restore original bytes")
	}
}

func TestDropdownChoiceProtectsUnsupportedState(t *testing.T) {
	for _, control := range []string{
		strings.Replace(sampleChoiceControl, `<w:dropDownList`, `<w:lock w:val="sdtLocked"/><w:dropDownList`, 1),
		strings.Replace(sampleChoiceControl, `<w:dropDownList`, `<w:dataBinding w:xpath="/status"/><w:dropDownList`, 1),
		strings.Replace(sampleChoiceControl, `lastValue="todo"`, `lastValue="done"`, 1),
		strings.Replace(sampleChoiceControl, `w:value="done"`, `w:value="todo"`, 1),
	} {
		s, file := withDocument(t, bodyXML(`<w:p>`+control+`</w:p>`))
		v := mustSnapshot(t, s)
		original := fileBytes(t, file)
		run := v.Blocks[0].Paragraph.Runs[0]
		if run.Object == nil || run.Object.ChoiceControl != nil || run.Object.Editable {
			t.Fatal("unsupported dropdown was editable")
		}
		_, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "update_control_choice", TargetID: run.ID, ExpectedText: "대기", Text: "완료 & 검토"}}}, "human")
		if err == nil || !bytes.Equal(original, fileBytes(t, file)) {
			t.Fatal("unsupported dropdown changed")
		}
	}
}

func TestInsertDropdownChoiceUsesUniqueNativeIDAndExpectedValue(t *testing.T) {
	ctx := context.Background()
	s, file := withDocument(t, bodyXML(`<w:p><w:r><w:t>Prefix</w:t></w:r></w:p><w:p>`+strings.Replace(sampleChoiceControl, `w:val="42"`, `w:val="1"`, 1)+`</w:p>`))
	v := mustSnapshot(t, s)
	original := fileBytes(t, file)
	options := []ControlChoiceOption{{Label: "대기", Value: "a"}, {Label: "완료 & 검토", Value: `b"`}}
	op := Operation{Type: "insert_control_choice", TargetID: v.Blocks[0].ID, Text: options[1].Label, ChoiceOptions: options}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}
	for _, bad := range []Operation{{Type: op.Type, TargetID: op.TargetID, Text: "missing", ChoiceOptions: options}, {Type: op.Type, TargetID: op.TargetID, Text: options[0].Label, ChoiceOptions: []ControlChoiceOption{options[0], options[0]}}, {Type: "insert_control_date", TargetID: op.TargetID, Text: "2026-11-25", ChoiceOptions: options}} {
		invalid := input
		invalid.Operations = []Operation{bad}
		if _, err := s.Apply(ctx, invalid, "human"); err == nil || !bytes.Equal(original, fileBytes(t, file)) {
			t.Fatal("invalid dropdown creation changed source")
		}
	}
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	saved := mustSnapshot(t, reopened)
	run := saved.Blocks[0].Paragraph.Runs[1]
	if run.Object == nil || run.Object.ChoiceControl == nil || run.Object.ChoiceControl.Value != op.Text || !bytes.Contains(reopened.pkg.document, []byte(`dws:id dws:val="2"`)) {
		t.Fatal("missing unique native dropdown")
	}
	updated, err := reopened.Apply(ctx, ApplyInput{SessionID: saved.ID, BaseRevision: saved.Revision, Operations: []Operation{{Type: "update_control_choice", TargetID: run.ID, ExpectedText: op.Text, Text: options[0].Label}}}, "human")
	if err != nil || updated.Blocks[0].Paragraph.Runs[1].Object.ChoiceControl.Value != options[0].Label {
		t.Fatal("new dropdown was not editable", err)
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, file)) {
		t.Fatal("undo did not restore original bytes")
	}
}
