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
