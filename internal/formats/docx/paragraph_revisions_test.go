package docx

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTrackedParagraphAlignmentCanBeReviewedAfterReopen(t *testing.T) {
	for _, review := range []string{"accept_revision", "reject_revision"} {
		t.Run(review, func(t *testing.T) {
			ctx := context.Background()
			const properties = `<w:spacing w:after="120"/><w:jc w:val="right"/>`
			const run = `<w:r><w:rPr><w:b/></w:rPr><w:t>Keep original text</w:t></w:r>`
			const second = `<w:p><w:r><w:t>Other paragraph</w:t></w:r></w:p>`
			s, filename := withDocument(t, bodyXML(`<w:p><w:pPr>`+properties+`</w:pPr>`+run+`</w:p>`+second))
			original := fileBytes(t, filename)
			baseline := s.pkg
			v := mustSnapshot(t, s)
			if !v.Blocks[0].Paragraph.AlignmentTrackable {
				t.Fatal("plain paragraph was not advertised for tracked alignment")
			}
			input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
				{Type: "track_paragraph_alignment", TargetID: v.Blocks[0].ID, Alignment: "center"},
			}}
			after, err := s.Apply(ctx, input, "ai")
			if err != nil {
				t.Fatal(err)
			}
			if after.Revision != v.Revision+1 || after.Sequence != v.Sequence+1 || s.events[0].TextOnly {
				t.Fatal("alignment change did not advance one structural revision")
			}
			if _, err := s.Apply(ctx, input, "ai"); !errors.Is(err, ErrRevisionConflict) {
				t.Fatalf("stale alignment write returned %v", err)
			}
			tracked := fileBytes(t, filename)
			reopened, err := Open(ctx, filename)
			if err != nil {
				t.Fatal(err)
			}
			snapshot := mustSnapshot(t, reopened)
			p := snapshot.Blocks[0].Paragraph
			if p.Layout.Align != "center" || !snapshot.TrackedChanges || p.AlignmentRevision == nil ||
				!p.AlignmentRevision.Editable || p.AlignmentRevision.Author != "AI" || p.AlignmentRevision.Date == "" ||
				p.AlignmentRevision.Label != "정렬 변경 끝 → 가운데" || p.AlignmentTrackable {
				t.Fatalf("tracked alignment was not reviewable: %+v", p)
			}
			if !strings.Contains(string(reopened.pkg.document), properties) || !strings.Contains(string(reopened.pkg.document), run+"</w:p>"+second) {
				t.Fatal("writing tracked alignment changed source properties or text")
			}
			result, err := reopened.Apply(ctx, ApplyInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision, Operations: []Operation{
				{Type: review, TargetID: p.ID},
			}}, "human")
			if err != nil {
				t.Fatal(err)
			}
			expected := "center"
			if review == "reject_revision" {
				expected = "end"
			}
			p = result.Blocks[0].Paragraph
			if result.TrackedChanges || p.AlignmentRevision != nil || p.Layout.Align != expected ||
				p.Layout.After == nil || *p.Layout.After != 120 {
				t.Fatalf("unexpected alignment or retained revision after %s: %+v", review, p)
			}
			for i, part := range baseline.parts {
				if part.header.Name != "word/document.xml" && !bytes.Equal(part.data, reopened.pkg.parts[i].data) {
					t.Fatalf("unrelated OOXML part changed: %s", part.header.Name)
				}
			}
			historyRestore(t, reopened, "undo")
			if !bytes.Equal(tracked, fileBytes(t, filename)) {
				t.Fatal("undo did not restore tracked file")
			}
			historyRestore(t, reopened, "undo")
			if !bytes.Equal(original, fileBytes(t, filename)) {
				t.Fatal("undo did not restore original file")
			}
		})
	}
}

func TestRejectTrackedParagraphAlignmentRestoresInheritance(t *testing.T) {
	ctx := context.Background()
	s, _ := withDocument(t, bodyXML(`<w:p/><w:p><w:r><w:t>A</w:t></w:r></w:p>`))
	v := mustSnapshot(t, s)
	next, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "track_paragraph_alignment", TargetID: v.Blocks[0].ID, Alignment: "end"},
		{Type: "track_paragraph_alignment", TargetID: v.Blocks[1].ID, Alignment: "justify"},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseXML(s.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	a, b := root.child("body").children[0].child("pPr").child("pPrChange"), root.child("body").children[1].child("pPr").child("pPrChange")
	if a == nil || b == nil || a.attr("id") == b.attr("id") {
		t.Fatal("tracking two paragraphs did not allocate unique revision IDs")
	}
	restored, err := s.Apply(ctx, ApplyInput{SessionID: next.ID, BaseRevision: next.Revision, Operations: []Operation{
		{Type: "reject_revision", TargetID: next.Blocks[0].ID},
		{Type: "reject_revision", TargetID: next.Blocks[1].ID},
	}}, "human")
	if err != nil || restored.TrackedChanges || restored.Blocks[0].Paragraph.Layout != nil || restored.Blocks[1].Paragraph.Layout != nil {
		t.Fatalf("did not restore alignment inheritance: %v", err)
	}
}

func TestComplexParagraphAlignmentRevisionRemainsReadOnly(t *testing.T) {
	for _, markup := range []string{
		`<w:pPr><w:spacing w:after="60"/><w:jc w:val="center"/><w:pPrChange w:id="12"><w:pPr><w:spacing w:after="120"/></w:pPr></w:pPrChange></w:pPr>`,
		`<w:pPr><w:jc w:val="center"/><w:pPrChange w:id="12"><w:pPr><w:jc w:val="right"/></w:pPr></w:pPrChange></w:pPr><w:fldSimple w:instr="DATE"/>`,
	} {
		ctx := context.Background()
		s, filename := withDocument(t, bodyXML(`<w:p>`+markup+`<w:r><w:t>Keep me</w:t></w:r></w:p>`))
		original := fileBytes(t, filename)
		v := mustSnapshot(t, s)
		p := v.Blocks[0].Paragraph
		if p.AlignmentRevision == nil || p.AlignmentRevision.Editable || p.AlignmentTrackable {
			t.Fatal("complex revision was not identified as read-only")
		}
		for _, op := range []Operation{
			{Type: "track_paragraph_alignment", Alignment: "start"},
			{Type: "accept_revision"},
			{Type: "reject_revision"},
		} {
			op.TargetID = p.ID
			_, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}, "human")
			if err == nil || !bytes.Equal(original, fileBytes(t, filename)) || mustSnapshot(t, s).Revision != v.Revision {
				t.Fatal("protected alignment operation changed the document")
			}
		}
	}
}
