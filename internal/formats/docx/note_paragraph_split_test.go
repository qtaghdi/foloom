package docx

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSplitNoteParagraphPreservesTerminalReferences(t *testing.T) {
	ctx := context.Background()
	source := `<w:p xmlns:w14="` + word2010NS + `" w14:paraId="5A17ABCD"><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:b/></w:rPr><w:t>처음 끝</w:t></w:r></w:p>`
	s, path := withDocument(t, bodyXML(source+`<w:p/>`))
	v := mustSnapshot(t, s)
	v, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: v.Blocks[0].ID, Reference: &ReferenceEdit{Action: "insert_footnote", Text: "각주 원본"}},
		{Type: "update_reference", TargetID: v.Blocks[1].ID, Reference: &ReferenceEdit{Action: "insert_endnote", Text: "미주 원본"}},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an imported paragraph with both references after its text.
	rewritePackage(t, path, func(name string, data []byte) []byte {
		if name != "word/document.xml" {
			return data
		}
		root, err := parseXML(data)
		if err != nil {
			t.Fatal(err)
		}
		body := root.child("body")
		first, second := body.children[0], body.children[1]
		marker := second.children[0]
		return []byte(string(data[:first.endStart]) + string(data[marker.start:marker.end]) +
			string(data[first.endStart:first.end]) + string(data[second.end:]))
	}, nil)
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, s)
	original := v.Blocks[0].Paragraph
	if !original.Runs[0].SplitNoteParagraph || original.MoveNoteUp || original.MoveNoteDown {
		t.Fatal("note split incorrectly depends on an adjacent paragraph")
	}
	noteParts := map[string][]byte{documentRelationships: s.pkg.partData(documentRelationships, nil)}
	for _, kind := range []string{"footnote", "endnote"} {
		name, note, err := s.pkg.notesPart(kind, nil)
		if err != nil || note == nil {
			t.Fatalf("%s part unavailable: %v", kind, err)
		}
		noteParts[name] = note.source
	}
	before := fileBytes(t, path)
	op := Operation{Type: "split_note_paragraph", TargetID: original.Runs[0].ID, ExpectedText: "처음 끝", Text: "처음 "}
	app := func(base uint64, ops ...Operation) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: base, Operations: ops}, "human")
	}
	if _, err := app(v.Revision-1, op); !errors.Is(err, ErrRevisionConflict) {
		t.Fatal("old revision was accepted")
	}
	if _, err := app(v.Revision, op, op); err == nil {
		t.Fatal("note split was accepted in a batch")
	}
	for _, prefix := range []string{"", "처음 끝", "끝", "\xec"} {
		invalid := op
		invalid.Text = prefix
		if _, err := app(v.Revision, invalid); err == nil || !bytes.Equal(before, fileBytes(t, path)) {
			t.Fatalf("unsafe prefix %q changed the document", prefix)
		}
	}
	if _, err := app(v.Revision, op); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := mustSnapshot(t, reopened)
	if len(snapshot.Blocks) != 2 {
		t.Fatal("chapter did not split into separate body paragraphs")
	}
	left, right := snapshot.Blocks[0].Paragraph, snapshot.Blocks[1].Paragraph
	if left.Anchor != original.Anchor || right.Anchor == "" || right.Anchor == left.Anchor ||
		left.Layout == nil || right.Layout == nil || left.Layout.Align != "center" || right.Layout.Align != "center" ||
		len(left.Runs) != 1 || len(right.Runs) != 3 || left.Runs[0].Text != "처음 " || right.Runs[0].Text != "끝" ||
		!left.Runs[0].Bold || !right.Runs[0].Bold {
		t.Fatal("paragraph identities, formatting, or text not preserved after split")
	}
	labels := map[string]string{}
	for _, run := range right.Runs[1:] {
		if run.Object == nil || run.Object.noteID != "1" {
			t.Fatal("right paragraph lost a note marker")
		}
		labels[run.Object.Kind] = run.Object.Text
	}
	if labels["footnote"] != "각주 원본" || labels["endnote"] != "미주 원본" {
		t.Fatal("a note was detached from its source")
	}
	for name, part := range noteParts {
		if !bytes.Equal(part, reopened.pkg.partData(name, nil)) {
			t.Fatalf("split modified %s", name)
		}
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, path)) {
		t.Fatal("undo did not restore the original note paragraph")
	}
}

func TestSplitNoteParagraphRejectsUnsafeStructures(t *testing.T) {
	ctx := context.Background()
	for name, modify := range map[string]func(string) string{
		"page boundary": func(raw string) string {
			return strings.Replace(raw, `<w:p>`, `<w:p><w:pPr><w:pageBreakBefore/></w:pPr>`, 1)
		},
		"tracked field": func(raw string) string {
			return strings.Replace(raw, `<w:r><w:t>original`, `<w:fldSimple w:instr="PAGE"><w:r><w:t>1</w:t></w:r></w:fldSimple><w:r><w:t>original`, 1)
		},
		"two text runs": func(raw string) string {
			return strings.Replace(raw, `</w:t></w:r>`, `</w:t></w:r><w:r><w:t>second</w:t></w:r>`, 1)
		},
		"dangling reference": func(raw string) string {
			return strings.Replace(raw, `footnoteReference dwn:id="1"`, `footnoteReference dwn:id="2"`, 1)
		},
		"shared reference": func(raw string) string {
			return strings.Replace(raw, `</w:body>`, `<w:p><w:r><w:footnoteReference w:id="1"/></w:r></w:p></w:body>`, 1)
		},
		"spanning range": func(raw string) string {
			raw = strings.Replace(raw, `<w:body>`, `<w:body><w:p><w:bookmarkStart w:id="5" w:name="Range"/></w:p>`, 1)
			return strings.Replace(raw, `</w:body>`, `<w:p><w:bookmarkEnd w:id="5"/></w:p></w:body>`, 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, path := withDocument(t, bodyXML(`<w:p><w:r><w:t>original</w:t></w:r></w:p>`))
			v := mustSnapshot(t, s)
			if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
				{Type: "update_reference", TargetID: v.Blocks[0].ID, Reference: &ReferenceEdit{Action: "insert_footnote", Text: "source"}},
			}}, "human"); err != nil {
				t.Fatal(err)
			}
			rewritePackage(t, path, func(name string, content []byte) []byte {
				if name == "word/document.xml" {
					return []byte(modify(string(content)))
				}
				return content
			}, nil)
			before := fileBytes(t, path)
			s, err := Open(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			v = mustSnapshot(t, s)
			for _, block := range v.Blocks {
				for _, run := range block.Paragraph.Runs {
					if run.Text != "original" {
						continue
					}
					if run.SplitNoteParagraph {
						t.Fatal("protected paragraph exposed as splittable")
					}
					if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
						{Type: "split_note_paragraph", TargetID: run.ID, ExpectedText: run.Text, Text: "orig"},
					}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, path)) {
						t.Fatal("protected paragraph changed")
					}
					return
				}
			}
			t.Fatal("expected source text not found")
		})
	}
}
