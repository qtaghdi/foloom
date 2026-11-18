package docx

import (
	"bytes"
	"context"
	"testing"
)

func TestReviewImportedRunFormatKeepsTextAndOtherRuns(t *testing.T) {
	const changed = `<w:r><w:rPr><w:b/><w:i/><w:rPrChange xmlns:old="` + wordNS + `" w:id="11" w:date="2025-01-02T03:04:05Z" w:author="Reviewer"><old:rPr><old:i/></old:rPr></w:rPrChange></w:rPr><w:t>서식 검토</w:t></w:r>`
	const other = `<w:r><w:rPr><w:color w:val="002244"/></w:rPr><w:t>unrelated</w:t></w:r>`
	for _, test := range []struct {
		action string
		bold   bool
	}{
		{"accept_revision", true},
		{"reject_revision", false},
	} {
		t.Run(test.action, func(t *testing.T) {
			ctx := context.Background()
			s, filename := withDocument(t, bodyXML(`<w:p>`+changed+other+`</w:p>`))
			before := fileBytes(t, filename)
			v := mustSnapshot(t, s)
			run := v.Blocks[0].Paragraph.Runs[0]
			if run.Object == nil || run.Object.Kind != "tracked_format" || !run.Object.Editable ||
				run.Object.Author != "Reviewer" || run.Object.Date != "2025-01-02T03:04:05Z" || !run.Bold || !run.Italic || run.Editable {
				t.Fatalf("imported run format change not reviewable: %+v", run)
			}
			if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{
				Type: test.action, TargetID: run.ID,
			}}}, "human"); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(ctx, filename)
			if err != nil {
				t.Fatal(err)
			}
			run = mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[0]
			if run.Object != nil || run.Bold != test.bold || !run.Italic || run.Text != "서식 검토" ||
				!bytes.Contains(reopened.pkg.document, []byte(other)) || bytes.Contains(reopened.pkg.document, []byte("rPrChange")) {
				t.Fatalf("review did not preserve text or restore the correct properties: %+v", run)
			}
			historyRestore(t, reopened, "undo")
			if !bytes.Equal(before, fileBytes(t, filename)) {
				t.Fatal("undo did not restore the original DOCX")
			}
		})
	}
}

func TestRunFormatReviewProtectsCommentRangesAndUnsupportedProperties(t *testing.T) {
	const text = `<w:r><w:rPr><w:b/><w:rPrChange w:id="5"><w:rPr/></w:rPrChange></w:rPr><w:t>review</w:t></w:r>`
	s, filename := withDocument(t, bodyXML(`<w:p><w:commentRangeStart w:id="0"/>`+text+`<w:commentRangeEnd w:id="0"/></w:p>`))
	v := mustSnapshot(t, s)
	run := v.Blocks[0].Paragraph.Runs[0]
	if run.Object == nil || run.Object.Editable {
		t.Fatal("a format revision within a comment range should be read-only")
	}
	before := fileBytes(t, filename)
	if _, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{
		Type: "reject_revision", TargetID: run.ID,
	}}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("a protected format revision was applied")
	}
	s, _ = withDocument(t, bodyXML(`<w:p><w:r><w:rPr><w:b/><w:rPrChange w:id="1"><w:rPr><w:vanish/></w:rPr></w:rPrChange></w:rPr><w:t>complex</w:t></w:r></w:p>`))
	if run := mustSnapshot(t, s).Blocks[0].Paragraph.Runs[0]; run.Object != nil && run.Object.Editable {
		t.Fatal("unsupported former properties must not be replaced")
	}
}

func TestTrackBoldWritesNativePreviousPropertiesAndSupportsReject(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:rPr><w:i/><w:color w:val="002244"/></w:rPr><w:t>sample</w:t></w:r></w:p>`))
	before := fileBytes(t, filename)
	apply := func(op Operation) Snapshot {
		t.Helper()
		v := mustSnapshot(t, s)
		out, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}, "ai")
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	v := mustSnapshot(t, s)
	run := v.Blocks[0].Paragraph.Runs[0]
	on := true
	op := Operation{Type: "track_run_bold", TargetID: run.ID, ExpectedText: "stale", Bold: &on}
	if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}, "ai"); err == nil ||
		!bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("a stale tracked formatting operation changed the file")
	}
	op.ExpectedText = run.Text
	v = apply(op)
	run = v.Blocks[0].Paragraph.Runs[0]
	if !run.Bold || !run.Italic || run.Color != "#002244" || run.Text != "sample" ||
		run.Object == nil || run.Object.Author != "AI" || !run.Object.Editable ||
		!bytes.Contains(s.pkg.document, []byte("rPrChange")) {
		t.Fatalf("tracked bold changed prior text/properties or cannot be reviewed: %+v", run)
	}
	s, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, s)
	v = apply(Operation{Type: "reject_revision", TargetID: v.Blocks[0].Paragraph.Runs[0].ID})
	run = v.Blocks[0].Paragraph.Runs[0]
	if run.Bold || !run.Italic || run.Color != "#002244" || run.Object != nil {
		t.Fatalf("rejecting bold did not restore formatting: %+v", run)
	}
	historyRestore(t, s, "undo")
	historyRestore(t, s, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original DOCX")
	}
}

func TestTrackBoldOffWithEmptyAndExplicitPropertySets(t *testing.T) {
	for _, prior := range []string{"", `<w:rPr><w:b/></w:rPr>`} {
		s, _ := withDocument(t, bodyXML(`<w:p><w:r>`+prior+`<w:t>sample</w:t></w:r></w:p>`))
		v := mustSnapshot(t, s)
		run := v.Blocks[0].Paragraph.Runs[0]
		off := false
		v, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{
			Type: "track_run_bold", TargetID: run.ID, ExpectedText: run.Text, Bold: &off,
		}}}, "human")
		if err != nil || v.Blocks[0].Paragraph.Runs[0].Bold {
			t.Fatal("tracked bold-off did not write the direct value", err)
		}
	}
}
