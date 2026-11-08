package docx

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func paragraphPlan(snapshot Snapshot, paragraphs ...ParagraphContent) ApplyInput {
	first, last := snapshot.Blocks[0].Paragraph, snapshot.Blocks[len(snapshot.Blocks)-1].Paragraph
	texts := []string{}
	for _, block := range snapshot.Blocks {
		texts = append(texts, textContentForTest(*block.Paragraph))
	}
	return ApplyInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision, Operations: []Operation{{Type: "replace_paragraphs", TargetID: first.ID, EndTargetID: last.ID, ExpectedText: strings.Join(texts, "\n"), Paragraphs: paragraphs}}}
}
func textContentForTest(p Paragraph) string {
	var text string
	for _, r := range p.Runs {
		text += r.Text
	}
	return text
}
func TestParagraphPlanSplitMergeReorderAndReopen(t *testing.T) {
	s, path := withDocument(t, bodyXML(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:b/></w:rPr><w:t>한글 😀</w:t></w:r><w:r><w:rPr><w:i/></w:rPr><w:t>right</w:t></w:r></w:p><w:p><w:r><w:t>next</w:t></w:r></w:p>`))
	original, err := loadPackage(fileBytes(t, path))
	if err != nil {
		t.Fatal(err)
	}
	snap := mustSnapshot(t, s)
	p, q := snap.Blocks[0].Paragraph, snap.Blocks[1].Paragraph
	plan := paragraphPlan(snap,
		ParagraphContent{SourceID: q.ID, Runs: []RunContent{{SourceID: q.Runs[0].ID, Text: "next"}}},
		ParagraphContent{SourceID: p.ID, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "한글 "}}},
		ParagraphContent{SourceID: p.ID, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "😀"}, {SourceID: p.Runs[1].ID, Text: "right\nsoft"}}},
	)
	after, err := s.Apply(context.Background(), plan, "human")
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != 2 || after.Sequence != 1 || len(after.Blocks) != 3 || textContentForTest(*after.Blocks[2].Paragraph) != "😀right\nsoft" || !after.Blocks[2].Paragraph.Runs[0].Bold || !after.Blocks[2].Paragraph.Runs[1].Italic || after.Blocks[2].Paragraph.Layout.Align != "center" {
		t.Fatalf("incorrect structured commit: %+v", after)
	}
	for i, part := range original.parts {
		if part.header.Name != "word/document.xml" && !bytes.Equal(part.data, s.pkg.parts[i].data) {
			t.Fatal("unrelated part changed")
		}
	}
	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	round := mustSnapshot(t, reopened)
	if textContentForTest(*round.Blocks[2].Paragraph) != "😀right\nsoft" || !round.Blocks[2].Paragraph.StructureEditable {
		t.Fatal("split result did not reopen as editable text")
	}
	p, q = round.Blocks[1].Paragraph, round.Blocks[2].Paragraph
	input := ApplyInput{SessionID: round.ID, BaseRevision: round.Revision, Operations: []Operation{{Type: "replace_paragraphs", TargetID: p.ID, EndTargetID: q.ID, ExpectedText: "한글 \n😀right\nsoft", Paragraphs: []ParagraphContent{{SourceID: p.ID, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "한글 "}, {SourceID: q.Runs[0].ID, Text: "😀"}, {SourceID: q.Runs[1].ID, Text: "right\nsoft"}}}}}}}
	merged, err := reopened.Apply(context.Background(), input, "ai")
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Blocks) != 2 || textContentForTest(*merged.Blocks[1].Paragraph) != "한글 😀right\nsoft" {
		t.Fatal("merge failed")
	}
}
func TestParagraphPlanRejectedAtomically(t *testing.T) {
	for _, name := range []string{"target", "expected", "source", "run", "overlap", "empty", "invalid", "cross-table", "rename"} {
		t.Run(name, func(t *testing.T) {
			s, path := createTest(t)
			snap := mustSnapshot(t, s)
			p := snap.Blocks[0].Paragraph
			input := ApplyInput{SessionID: snap.ID, BaseRevision: 1, Operations: []Operation{{Type: "replace_paragraphs", TargetID: p.ID, EndTargetID: p.ID, ExpectedText: "hello", Paragraphs: []ParagraphContent{{SourceID: p.ID, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "new"}}}}}}}
			op := &input.Operations[0]
			switch name {
			case "target":
				op.TargetID = "stale"
			case "expected":
				op.ExpectedText = "wrong"
			case "source":
				op.Paragraphs[0].SourceID = "other"
			case "run":
				op.Paragraphs[0].Runs[0].SourceID = snap.Blocks[1].Rows[0].Cells[0].Paragraphs[0].Runs[0].ID
			case "overlap":
				input.Operations = append(input.Operations, Operation{Type: "replace_run_text", TargetID: p.Runs[0].ID, ExpectedText: "hello", Text: "other"})
			case "empty":
				op.Paragraphs = nil
			case "invalid":
				op.Paragraphs[0].Runs[0].Text = "bad\ttext"
			case "cross-table":
				op.EndTargetID = snap.Blocks[1].Rows[0].Cells[0].Paragraphs[0].ID
			case "rename":
				s.rename = func(string, string) error { return fmt.Errorf("injected failure") }
			}
			before := fileBytes(t, path)
			if _, err := s.Apply(context.Background(), input, "ai"); err == nil {
				t.Fatal("invalid plan accepted")
			}
			if !bytes.Equal(before, fileBytes(t, path)) || mustSnapshot(t, s).Revision != 1 || len(s.events) != 0 {
				t.Fatal("rejected paragraph plan mutated authoritative state")
			}
		})
	}
}
func TestParagraphPlanNamespaceAndIdentity(t *testing.T) {
	xml := `<x:document xmlns:x="` + wordNS + `" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"><x:body><x:p xmlns:local="` + wordNS + `" w14:paraId="12345678"><local:pPr><local:jc local:val="center"/></local:pPr><local:r><local:rPr><local:b/></local:rPr><local:t>original</local:t></local:r></x:p></x:body></x:document>`
	s, _ := withDocument(t, xml)
	snap := mustSnapshot(t, s)
	p := snap.Blocks[0].Paragraph
	if !p.StructureEditable {
		t.Fatal("namespace aliases disabled safe paragraph")
	}
	plan := paragraphPlan(snap, ParagraphContent{SourceID: p.ID, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "left"}}}, ParagraphContent{SourceID: p.ID, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "right"}}})
	after, err := s.Apply(context.Background(), plan, "human")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(s.pkg.document), `paraId="12345678"`) != 1 || !after.Blocks[1].Paragraph.Runs[0].Bold || after.Blocks[1].Paragraph.Layout.Align != "center" {
		t.Fatal("namespace scope or unique paragraph identity was not preserved")
	}
}
func TestParagraphStructureProtection(t *testing.T) {
	for _, body := range []string{`<w:p><w:r><w:t>x</w:t></w:r><w:r><w:br w:type="page"/></w:r></w:p>`, `<w:p><w:pPr><w:sectPr/></w:pPr><w:r><w:t>x</w:t></w:r></w:p>`, `<w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:t>x</w:t></w:r></w:p>`, `<w:p><!--opaque--><w:r><w:t>x</w:t></w:r></w:p>`, `<w:p><w:bookmarkStart w:id="1"/><w:r><w:t>x</w:t></w:r></w:p>`} {
		s, _ := withDocument(t, bodyXML(body))
		if mustSnapshot(t, s).Blocks[0].Paragraph.StructureEditable {
			t.Fatalf("protected paragraph exposed structure writes: %s", body)
		}
	}
}

func TestInlineBreakReplacementRemainsEditable(t *testing.T) {
	s, _ := createTest(t)
	snap := mustSnapshot(t, s)
	for _, text := range []string{"first\nsecond", "after\n\nthird", "single"} {
		run := snap.Blocks[0].Paragraph.Runs[0]
		var err error
		snap, err = s.Apply(context.Background(), batch(snap, "replace_run_text", run, text), "human")
		if err != nil {
			t.Fatal(err)
		}
		if snap.Blocks[0].Paragraph.Runs[0].Text != text || !snap.Blocks[0].Paragraph.Runs[0].Editable {
			t.Fatal("inline break stopped being editable")
		}
	}
}
func TestParagraphPlanInsideTableCell(t *testing.T) {
	s, _ := createTest(t)
	snap := mustSnapshot(t, s)
	p := snap.Blocks[1].Rows[0].Cells[0].Paragraphs[0]
	input := ApplyInput{SessionID: snap.ID, BaseRevision: snap.Revision, Operations: []Operation{{Type: "replace_paragraphs", TargetID: p.ID, EndTargetID: p.ID, ExpectedText: "A", Paragraphs: []ParagraphContent{
		{SourceID: p.ID, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "A"}}},
		{SourceID: p.ID, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "new cell paragraph"}}},
	}}}}
	after, err := s.Apply(context.Background(), input, "human")
	if err != nil {
		t.Fatal(err)
	}
	cell := after.Blocks[1].Rows[0].Cells[0]
	if len(cell.Paragraphs) != 2 || textContentForTest(cell.Paragraphs[1]) != "new cell paragraph" || after.Blocks[1].Rows[0].Cells[1].Paragraphs[0].Runs[0].Text != "B" {
		t.Fatal("cell structure edit escaped its cell")
	}
}
