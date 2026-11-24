package docx

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

const sampleDateControl = `<w:sdt><w:sdtPr><w:alias w:val="Delivery date"/><w:tag w:val="delivery"/><w:id w:val="42"/><w:date w:fullDate="2026-11-18T00:00:00Z"><w:dateFormat w:val="yyyy-MM-dd"/><w:lid w:val="en-US"/><w:storeMappedDataAs w:val="dateTime"/><w:calendar w:val="gregorian"/></w:date></w:sdtPr><w:sdtContent><w:r><w:rPr/></w:r><w:r><w:rPr><w:b/></w:rPr><w:t>2026-11-18</w:t></w:r></w:sdtContent></w:sdt>`

func TestDateContentControlInsertion(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>Prefix</w:t></w:r></w:p><w:p>`+sampleDateControl+`</w:p>`))
	original := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	op := Operation{Type: "insert_control_date", TargetID: v.Blocks[0].ID, Text: "2032-02-29"}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}
	for _, invalid := range []string{"2031-02-29", "2026-01-01T00:00:00Z", "", "0000-01-01"} {
		bad := op
		bad.Text = invalid
		batch := input
		batch.Operations = []Operation{bad}
		if _, err := s.Apply(ctx, batch, "human"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
			t.Fatal("invalid date insertion changed the document")
		}
	}
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	saved := mustSnapshot(t, reopened)
	object := saved.Blocks[0].Paragraph.Runs[1].Object
	if object == nil || object.DateControl == nil || object.DateControl.Value != op.Text || !object.Editable || !bytes.Contains(reopened.pkg.document, []byte(sampleDateControl)) {
		t.Fatal("date insertion did not preserve native representation")
	}
	if !bytes.Contains(reopened.pkg.document, []byte(`dws:id dws:val="1"`)) {
		t.Fatal("new control ID missing")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore original bytes")
	}
}

func TestDateContentControlInsertionRejectsActiveReference(t *testing.T) {
	s, filename := withDocument(t, bodyXML(`<w:p><w:bookmarkStart w:id="8" w:name="date_range"/></w:p><w:p><w:r><w:t>Protected</w:t></w:r></w:p><w:p><w:bookmarkEnd w:id="8"/></w:p>`))
	v := mustSnapshot(t, s)
	original := fileBytes(t, filename)
	_, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "insert_control_date", TargetID: v.Blocks[1].ID, Text: "2032-02-29"}}}, "human")
	if err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("date insertion modified an active reference range")
	}
}

func TestDateContentControlUpdatesNativeDateAndDisplayTogether(t *testing.T) {
	ctx := context.Background()
	const old = "2026-11-18"
	const next = "2032-02-29"
	prefix := `<w:p><w:r><w:t xml:space="preserve">Delivery: </w:t></w:r>`
	suffix := `<w:r><w:t> after the date</w:t></w:r></w:p><w:p>` + chartDeleteDrawing + `</w:p>`
	s, filename := chartDocument(t, prefix+sampleDateControl+suffix, chartTitlePart, embeddedChartData)
	baseline, original := s.pkg, fileBytes(t, filename)
	v := mustSnapshot(t, s)
	run := v.Blocks[0].Paragraph.Runs[1]
	if run.Object == nil || !run.Object.Editable || run.Object.UnwrapEditable || run.Object.Text != old ||
		run.Object.Label != "Delivery date" || run.Object.DateControl == nil || *run.Object.DateControl != (ControlDatePreview{old, controlDateFormat}) {
		t.Fatalf("unexpected date control projection: %+v", run.Object)
	}
	op := Operation{Type: "update_control_date", TargetID: run.ID, ExpectedText: old, Text: next}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}
	stale, invalid, plainEdit := op, op, op
	stale.ExpectedText = "2027-12-31"
	invalid.Text = "2031-02-29"
	plainEdit.Type = "update_control_text"
	for _, ops := range [][]Operation{{stale}, {invalid}, {plainEdit}, {op, op}, {{Type: "unwrap_control", TargetID: run.ID}}} {
		batch := input
		batch.Operations = ops
		_, err := s.Apply(ctx, batch, "ai")
		if err == nil || !bytes.Equal(original, fileBytes(t, filename)) || mustSnapshot(t, s).Revision != v.Revision {
			t.Fatalf("invalid date edit changed bytes or revision: %+v", ops)
		}
	}
	after, err := s.Apply(ctx, input, "human")
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != v.Revision+1 || after.Sequence != v.Sequence+1 || s.events[0].TextOnly {
		t.Fatal("date and display were not committed in a single structural revision")
	}
	if _, err := s.Apply(ctx, input, "ai"); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale date update returned %v", err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	out := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[1].Object
	if out == nil || out.DateControl == nil || out.DateControl.Value != next || out.Text != next || out.Label != run.Object.Label {
		t.Fatalf("date was not readable after reopening: %+v", out)
	}
	root, err := parseXML(reopened.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	date, display := dateControlNodes(root.child("body").child("p").child("sdt"), reopened.pkg.document)
	if date == nil || date.attr("fullDate") != next+controlDateSuffix || display.parent.child("rPr").child("b") == nil {
		t.Fatal("native date or run formatting was lost")
	}
	withoutOpening := func(data []byte) string {
		tree, err := parseXML(data)
		if err != nil {
			t.Fatal(err)
		}
		current := tree.child("body").child("p").child("sdt").child("sdtPr").child("date")
		return string(data[:current.start]) + string(data[current.inner:])
	}
	if withoutOpening(reopened.pkg.document) != strings.Replace(withoutOpening(baseline.document), old, next, 1) ||
		len(reopened.pkg.parts) != len(baseline.parts) {
		t.Fatal("date change modified unrelated document XML")
	}
	for _, part := range baseline.parts {
		if part.header.Name != "word/document.xml" && !bytes.Equal(reopened.pkg.partData(part.header.Name, nil), part.data) {
			t.Fatalf("date edit changed %s", part.header.Name)
		}
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original package bytes")
	}
}

func TestDateContentControlRejectsUnsupportedRepresentations(t *testing.T) {
	for _, tc := range []struct {
		name, old, replacement string
	}{
		{"control locked", "<w:date ", `<w:lock w:val="sdtLocked"/><w:date `},
		{"content locked", "<w:date ", `<w:lock w:val="contentLocked"/><w:date `},
		{"binding", "<w:date ", `<w:dataBinding w:xpath="/date"/><w:date `},
		{"placeholder", "<w:date ", "<w:showingPlcHdr/><w:date "},
		{"mixed content type", "<w:date ", "<w:text/><w:date "},
		{"different calendar", "gregorian", "hebrew"},
		{"different locale", "en-US", "ar-SA"},
		{"different format", "yyyy-MM-dd", "dd/MM/yyyy"},
		{"full date time", "T00:00:00Z", "T03:00:00Z"},
		{"inconsistent cache", "<w:t>2026-11-18", "<w:t>2026-11-17"},
		{"multiple text runs", "</w:sdtContent>", "<w:r><w:t>extra</w:t></w:r></w:sdtContent>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			control := strings.Replace(sampleDateControl, tc.old, tc.replacement, 1)
			s, filename := withDocument(t, bodyXML("<w:p>"+control+"</w:p>"))
			assertDateControlRejected(t, s, filename, 0)
		})
	}
	ranged := `<w:p><w:bookmarkStart w:id="8" w:name="date_range"/></w:p><w:p>` + sampleDateControl + `</w:p><w:p><w:bookmarkEnd w:id="8"/></w:p>`
	s, rangedFile := withDocument(t, bodyXML(ranged))
	assertDateControlRejected(t, s, rangedFile, 1)
	_, filename := withDocument(t, bodyXML("<w:p>"+sampleDateControl+"</w:p>"))
	rewritePackage(t, filename, func(_ string, data []byte) []byte { return data }, map[string][]byte{
		documentRelationships:         []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="settings" Type="` + officeRelNS + `/settings" Target="protected-settings.xml"/></Relationships>`),
		"word/protected-settings.xml": []byte(`<w:settings xmlns:w="` + wordNS + `"><w:documentProtection w:edit="readOnly" w:enforcement="true"/></w:settings>`),
	})
	protected, err := Open(context.Background(), filename)
	if err != nil {
		t.Fatal(err)
	}
	assertDateControlRejected(t, protected, filename, 0)
}

func assertDateControlRejected(t *testing.T, s *Session, filename string, paragraph int) {
	t.Helper()
	initial, original := mustSnapshot(t, s), fileBytes(t, filename)
	run := initial.Blocks[paragraph].Paragraph.Runs[0]
	if run.Object == nil || run.Object.Kind != "control" || run.Object.Editable || run.Object.DateControl != nil {
		t.Fatalf("expected opaque date control, got %+v", run.Object)
	}
	_, err := s.Apply(context.Background(), ApplyInput{SessionID: initial.ID, BaseRevision: initial.Revision, Operations: []Operation{
		{Type: "update_control_date", TargetID: run.ID, ExpectedText: "2026-11-18", Text: "2032-02-29"},
	}}, "ai")
	if err == nil || !bytes.Equal(original, fileBytes(t, filename)) || mustSnapshot(t, s).Revision != initial.Revision {
		t.Fatal("protected date control operation changed bytes or revision")
	}
}
