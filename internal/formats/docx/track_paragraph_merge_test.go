package docx

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestTrackParagraphMergeSavesAndReviewsNativeDeletion(t *testing.T) {
	ctx := context.Background()
	source := `<w:p xmlns:w14="` + word2010NS + `" w14:paraId="5A17ABCD"><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">First </w:t></w:r></w:p>` +
		`<w:p xmlns:w14="` + word2010NS + `" w14:paraId="5A17ABCE"><w:pPr><w:jc w:val="right"/></w:pPr><w:r><w:rPr><w:i/></w:rPr><w:t>Second</w:t></w:r></w:p>`
	s, path := withDocument(t, bodyXML(source))
	original := fileBytes(t, path)
	before := mustSnapshot(t, s)
	left, right := before.Blocks[0].Paragraph, before.Blocks[1].Paragraph
	if !left.MergeTrackable || right.MergeTrackable {
		t.Fatal("only the first paragraph should expose the merge")
	}
	op := Operation{Type: "track_paragraph_merge", TargetID: left.ID}
	apply := func(base uint64, ops ...Operation) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: s.id, BaseRevision: base, Operations: ops}, "ai")
	}
	if _, err := apply(before.Revision+1, op); !errors.Is(err, ErrRevisionConflict) {
		t.Fatal("stale revision was accepted")
	}
	if _, err := apply(before.Revision, op, op); err == nil {
		t.Fatal("tracked merge was accepted in a batch")
	}
	wrongFields := op
	wrongFields.Text = "Unexpected"
	if _, err := apply(before.Revision, wrongFields); err == nil || !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("unexpected fields changed the file")
	}
	if _, err := apply(before.Revision, op); err != nil {
		t.Fatal(err)
	}
	recorded := fileBytes(t, path)
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	view := mustSnapshot(t, s)
	if len(view.Blocks) != 2 || !view.TrackedChanges ||
		view.Blocks[0].Paragraph.Anchor != left.Anchor || view.Blocks[1].Paragraph.Anchor != right.Anchor {
		t.Fatal("recording changed the paragraph identities or boundary")
	}
	mark := view.Blocks[0].Paragraph.BreakRevision
	if mark == nil || !mark.Editable || mark.Label != "문단 나눔 삭제" || mark.Author != "AI" || mark.Date == "" || mark.Text != "First \nSecond" {
		t.Fatalf("recording did not produce a native reviewable deletion: %+v", mark)
	}
	root, err := parseXML(s.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	if native := paragraphBreakMarker(root.child("body").children[0]); native == nil || !native.is("del") || native.attr("id") == "" {
		t.Fatal("native paragraph mark was not written")
	}
	for _, decision := range []string{"accept_revision", "reject_revision"} {
		view = mustSnapshot(t, s)
		result, err := apply(view.Revision, Operation{Type: decision, TargetID: view.Blocks[0].ID})
		if err != nil {
			t.Fatal(err)
		}
		first := result.Blocks[0].Paragraph
		if result.TrackedChanges || first.BreakRevision != nil || first.Runs[0].Text != "First " || !first.Runs[0].Bold {
			t.Fatal("review lost the first run or retained the revision")
		}
		if decision == "accept_revision" {
			if len(result.Blocks) != 1 || first.Anchor != right.Anchor || first.Layout.Align != "end" ||
				len(first.Runs) != 2 || first.Runs[1].Text != "Second" || !first.Runs[1].Italic {
				t.Fatal("accept must keep both runs and the following paragraph's properties")
			}
		} else if len(result.Blocks) != 2 || first.Anchor != left.Anchor || first.Layout.Align != "center" ||
			result.Blocks[1].Paragraph.Anchor != right.Anchor || result.Blocks[1].Paragraph.Layout.Align != "end" {
			t.Fatal("reject must keep the original paragraph identities and properties")
		}
		historyRestore(t, s, "undo")
		if !bytes.Equal(recorded, fileBytes(t, path)) {
			t.Fatal("undo did not restore the recorded deletion")
		}
	}
	historyRestore(t, s, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("undo did not restore the original file")
	}
}

func TestTrackParagraphMergeProtectsUnsafePairs(t *testing.T) {
	const left = `<w:p><w:r><w:t>First</w:t></w:r></w:p>`
	const right = `<w:p><w:r><w:t>Second</w:t></w:r></w:p>`
	for name, document := range map[string]string{
		"last paragraph":                       left,
		"field":                                left + `<w:p><w:fldSimple w:instr="PAGE"><w:r><w:t>1</w:t></w:r></w:fldSimple></w:p>`,
		"page boundary":                        left + `<w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:t>Second</w:t></w:r></w:p>`,
		"different paragraph properties":       left + `<w:p><w:pPr><w:spacing w:after="240"/></w:pPr><w:r><w:t>Second</w:t></w:r></w:p>`,
		"spanning reference":                   `<w:p><w:bookmarkStart w:id="1" w:name="Outer"/></w:p>` + left + right + `<w:p><w:bookmarkEnd w:id="1"/></w:p>`,
		"existing preceding boundary revision": `<w:p><w:pPr>` + insertedBreakMarker + `</w:pPr><w:r><w:t>Earlier</w:t></w:r></w:p>` + left + right,
	} {
		t.Run(name, func(t *testing.T) {
			s, path := withDocument(t, bodyXML(document))
			original := fileBytes(t, path)
			view := mustSnapshot(t, s)
			for _, block := range view.Blocks {
				paragraph := block.Paragraph
				if paragraph == nil || len(paragraph.Runs) == 0 || paragraph.Runs[0].Text != "First" {
					continue
				}
				if paragraph.MergeTrackable {
					t.Fatal("unsafe target was exposed")
				}
				rejected := Operation{Type: "track_paragraph_merge", TargetID: paragraph.ID}
				if _, err := s.Apply(context.Background(), ApplyInput{SessionID: view.ID, BaseRevision: view.Revision, Operations: []Operation{rejected}}, "human"); err == nil ||
					!bytes.Equal(original, fileBytes(t, path)) {
					t.Fatal("tracking an unsafe merge changed the package")
				}
				return
			}
			t.Fatal("missing first paragraph")
		})
	}
}
