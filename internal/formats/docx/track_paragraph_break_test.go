package docx

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTrackParagraphBreakSavesAndReviewsNativeInsertion(t *testing.T) {
	ctx := context.Background()
	source := `<w:p xmlns:w14="` + word2010NS + `" w14:paraId="5A17ABCD"><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:b/></w:rPr><w:t>First Second</w:t></w:r></w:p>`
	neighbor := `<w:p><w:bookmarkStart w:id="0" w:name="Other"/><w:r><w:t>Unrelated</w:t></w:r><w:bookmarkEnd w:id="0"/></w:p>`
	s, path := withDocument(t, bodyXML(source+neighbor))
	original := fileBytes(t, path)
	v := mustSnapshot(t, s)
	p := v.Blocks[0].Paragraph
	if !p.Runs[0].BreakTrackable {
		t.Fatal("plain paragraph was not exposed for tracked breaking")
	}
	op := Operation{Type: "track_paragraph_break", TargetID: p.Runs[0].ID, ExpectedText: p.Runs[0].Text, Text: "First "}
	apply := func(base uint64, ops ...Operation) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: base, Operations: ops}, "ai")
	}
	if _, err := apply(v.Revision+1, op); !errors.Is(err, ErrRevisionConflict) {
		t.Fatal("old revision was accepted")
	}
	if _, err := apply(v.Revision, op, op); err == nil {
		t.Fatal("tracked split was accepted in a batch")
	}
	for _, prefix := range []string{"", "Second", "First Second"} {
		rejected := op
		rejected.Text = prefix
		if _, err := apply(v.Revision, rejected); err == nil || !bytes.Equal(original, fileBytes(t, path)) {
			t.Fatalf("invalid split prefix %q changed the file", prefix)
		}
	}
	if _, err := apply(v.Revision, op); err != nil {
		t.Fatal(err)
	}
	recorded := fileBytes(t, path)
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, s)
	left, right := v.Blocks[0].Paragraph, v.Blocks[1].Paragraph
	if len(v.Blocks) != 3 || left.Anchor != p.Anchor || right.Anchor == "" || right.Anchor == left.Anchor ||
		left.Layout.Align != "center" || right.Layout.Align != "center" ||
		left.Runs[0].Text != "First " || right.Runs[0].Text != "Second" ||
		!left.Runs[0].Bold || !right.Runs[0].Bold || !strings.Contains(string(s.pkg.document), neighbor) {
		t.Fatal("recorded split lost text or existing layout")
	}
	root, err := parseXML(s.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	mark := paragraphBreakMarker(root.child("body").children[0])
	if !v.TrackedChanges || left.BreakRevision == nil || !left.BreakRevision.Editable ||
		left.BreakRevision.Author != "AI" || left.BreakRevision.Date == "" ||
		mark == nil || mark.attr("id") == "" || mark.attr("id") == "0" {
		t.Fatal("missing unique, reviewable native change with author and time")
	}
	for _, decision := range []string{"accept_revision", "reject_revision"} {
		v = mustSnapshot(t, s)
		result, err := apply(v.Revision, Operation{Type: decision, TargetID: v.Blocks[0].ID})
		if err != nil {
			t.Fatal(err)
		}
		expectedBlocks := 3
		if decision == "reject_revision" {
			expectedBlocks = 2
			first := result.Blocks[0].Paragraph
			if len(first.Runs) != 2 || first.Runs[0].Text+first.Runs[1].Text != "First Second" ||
				!first.Runs[0].Bold || !first.Runs[1].Bold {
				t.Fatal("reject did not combine the original text and styling")
			}
		}
		if len(result.Blocks) != expectedBlocks || result.TrackedChanges || result.Blocks[0].Paragraph.BreakRevision != nil {
			t.Fatal("review did not clear the tracked break")
		}
		historyRestore(t, s, "undo")
		if !bytes.Equal(recorded, fileBytes(t, path)) {
			t.Fatal("undo did not restore recorded Word markup")
		}
	}
	historyRestore(t, s, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("undo did not restore the original paragraph")
	}
}

func TestTrackParagraphBreakRejectsProtectedBoundaries(t *testing.T) {
	const plain = `<w:p><w:r><w:t>First Second</w:t></w:r></w:p>`
	for name, content := range map[string]string{
		"multiple runs":    `<w:p><w:r><w:t>First Second</w:t></w:r><w:r><w:t>More</w:t></w:r></w:p>`,
		"field":            `<w:p><w:r><w:t>First Second</w:t></w:r><w:fldSimple w:instr="PAGE"><w:r><w:t>1</w:t></w:r></w:fldSimple></w:p>`,
		"page boundary":    `<w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:t>First Second</w:t></w:r></w:p>`,
		"section boundary": `<w:p><w:pPr><w:sectPr/></w:pPr><w:r><w:t>First Second</w:t></w:r></w:p>`,
		"after revision":   `<w:p><w:pPr>` + insertedBreakMarker + `</w:pPr><w:r><w:t>Earlier</w:t></w:r></w:p>` + plain,
		"spanning reference": `<w:p><w:bookmarkStart w:id="1" w:name="Range"/></w:p>` + plain +
			`<w:p><w:bookmarkEnd w:id="1"/></w:p>`,
	} {
		t.Run(name, func(t *testing.T) {
			s, path := withDocument(t, bodyXML(content))
			before := fileBytes(t, path)
			v := mustSnapshot(t, s)
			for _, block := range v.Blocks {
				for _, run := range block.Paragraph.Runs {
					if run.Text != "First Second" {
						continue
					}
					if run.BreakTrackable {
						t.Fatal("protected paragraph was exposed for tracking")
					}
					if _, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
						{Type: "track_paragraph_break", TargetID: run.ID, ExpectedText: run.Text, Text: "First "},
					}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, path)) {
						t.Fatal("tracking a protected paragraph changed its file")
					}
					return
				}
			}
			t.Fatal("fixture lost its source")
		})
	}
}
