package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDOCXReviewsIndividualTrackedTextWithoutChangingOtherRevisions(t *testing.T) {
	insert := `<w:ins w:id="1" w:author="Reviewer" xmlns:cx="urn:run-style"><w:r><w:rPr><w:b/><cx:option/></w:rPr><w:t>inserted</w:t></w:r></w:ins>`
	deleted := `<w:del w:id="2" w:author="Reviewer"><w:r><w:rPr><w:i/></w:rPr><w:delText xml:space="preserve"> deleted &amp; safe </w:delText></w:r></w:del>`
	for _, tc := range []struct {
		name, operation string
		index           int
		kept            string
		untouched       string
	}{
		{"accept insert", "accept_revision", 1, "inserted", deleted},
		{"reject insert", "reject_revision", 1, "", deleted},
		{"accept delete", "accept_revision", 2, "", insert},
		{"reject delete", "reject_revision", 2, " deleted & safe ", insert},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>original</w:t></w:r>`+insert+deleted+`</w:p>`))
			before := fileBytes(t, filename)
			v := mustSnapshot(t, s)
			runs := v.Blocks[0].Paragraph.Runs
			selected := runs[tc.index]
			if !v.TrackedChanges || selected.Object == nil || !selected.Object.Editable || selected.Object.Author != "Reviewer" {
				t.Fatal("tracked change not exposed for review")
			}
			if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
				{Type: tc.operation, TargetID: selected.ID},
			}}, "human"); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(ctx, filename)
			if err != nil {
				t.Fatal(err)
			}
			doc := string(reopened.pkg.document)
			if !strings.Contains(doc, tc.untouched) {
				t.Fatal("review changed another revision")
			}
			var plain []string
			for _, run := range mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs {
				if run.Object == nil {
					plain = append(plain, run.Text)
					if run.Text == "inserted" && (!run.Bold || !strings.Contains(doc, `xmlns:cx="urn:run-style"`)) || run.Text == " deleted & safe " && !run.Italic {
						t.Fatal("review lost text formatting or inherited namespace")
					}
				}
			}
			if strings.Join(plain, "") != "original"+tc.kept {
				t.Fatalf("incorrect reviewed text: %v", plain)
			}
			historyRestore(t, reopened, "undo")
			if !bytes.Equal(before, fileBytes(t, filename)) {
				t.Fatal("undo did not restore original revision XML")
			}
		})
	}
}

func TestDOCXReviewProtectsComplexTrackedRuns(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:del w:id="1"><w:r><w:delText>outer</w:delText></w:r><w:ins w:id="2"><w:r><w:t>nested</w:t></w:r></w:ins></w:del></w:p>`))
	before := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "accept_revision", TargetID: v.Blocks[0].Paragraph.Runs[0].ID},
	}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("nested tracked content could be silently removed")
	}
}

func TestDOCXTrackedRunPersistsAsNativeInsertAndDelete(t *testing.T) {
	ctx := context.Background()
	raw := `<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>팀 🙂</w:t></w:r><w:r><w:t> suffix</w:t></w:r></w:p>` +
		`<w:p><w:ins w:id="0" w:author="Reviewer"><w:r><w:t>existing</w:t></w:r></w:ins></w:p>`
	s, filename := withDocument(t, bodyXML(raw))
	before := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	run := v.Blocks[0].Paragraph.Runs[0]
	if !run.Trackable {
		t.Fatal("plain formatted run cannot be selected")
	}
	op := Operation{Type: "track_run_text", TargetID: run.ID, ExpectedText: "stale", Text: "연구팀 & 협력"}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}
	if _, err := s.Apply(ctx, input, "ai"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("stale tracked text committed")
	}
	input.Operations[0].ExpectedText = run.Text
	if _, err := s.Apply(ctx, input, "ai"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	view := mustSnapshot(t, reopened)
	runs := view.Blocks[0].Paragraph.Runs
	if !view.TrackedChanges || len(runs) != 3 || runs[0].Object == nil || runs[1].Object == nil ||
		runs[0].Object.Kind != "tracked_delete" || runs[0].Object.Text != run.Text ||
		runs[1].Object.Kind != "tracked_insert" || runs[1].Object.Text != op.Text ||
		runs[0].Object.Author != "AI" || runs[0].Object.Date == "" || runs[2].Text != " suffix" {
		t.Fatalf("tracked replacement after reopen: %+v", runs)
	}
	doc := string(reopened.pkg.document)
	if !strings.Contains(doc, `dwr:id="1"`) || !strings.Contains(doc, `dwr:id="2"`) ||
		!strings.Contains(doc, `<w:ins w:id="0" w:author="Reviewer">`) {
		t.Fatal("the new annotation IDs collide with existing revisions")
	}
	accepted, err := reopened.Apply(ctx, ApplyInput{SessionID: view.ID, BaseRevision: view.Revision, Operations: []Operation{
		{Type: "accept_revision", TargetID: runs[0].ID}, {Type: "accept_revision", TargetID: runs[1].ID},
	}}, "human")
	if err != nil || accepted.Blocks[0].Paragraph.Runs[0].Text != op.Text || !accepted.Blocks[0].Paragraph.Runs[0].Bold {
		t.Fatalf("accept changed the inserted text or its formatting: %v", err)
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore original package bytes")
	}
}
