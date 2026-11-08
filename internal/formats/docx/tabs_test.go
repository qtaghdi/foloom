package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestTabStopsAndCharactersRoundTrip(t *testing.T) {
	s, path := withDocument(t, bodyXML(`<w:p><w:pPr><w:tabs xmlns:c="urn:custom"><w:tab w:val="left" w:pos="720" c:keep="yes"/><w:tab w:val="right" w:pos="1440"/></w:tabs></w:pPr><w:r><w:t>A</w:t><w:tab/><w:t>B</w:t></w:r></w:p>`))
	original := fileBytes(t, path)
	v := mustSnapshot(t, s)
	p := v.Blocks[0].Paragraph
	if !p.StructureEditable || p.Runs[0].Text != "A\tB" || len(p.Layout.Tabs) != 2 {
		t.Fatalf("tab inspection: %+v", p)
	}
	next, err := s.Apply(context.Background(), paragraphPlan(v, ParagraphContent{SourceID: p.ID, Format: &ParagraphFormat{Tabs: []TabStop{{Position: 720, Align: "decimal", Leader: "dot"}, {Position: 1440, Align: "clear"}, {Position: 2160, Align: "center"}}}, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "A\t12.34\nC\tD"}}}), "human")
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Blocks[0].Paragraph.Layout.Tabs) != 2 || next.Blocks[0].Paragraph.Layout.Tabs[0].Align != "decimal" {
		t.Fatal("effective stops mismatch")
	}
	data := string(s.pkg.document)
	for _, want := range []string{`c:keep="yes"`, `val="clear"`, `<dw:tab `, `<dw:br `} {
		if !strings.Contains(data, want) {
			t.Fatalf("missing %s in %s", want, data)
		}
	}
	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	p = mustSnapshot(t, reopened).Blocks[0].Paragraph
	if p.Runs[0].Text != "A\t12.34\nC\tD" || !p.StructureEditable {
		t.Fatalf("reopen %+v", p)
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("undo changed original")
	}
}

func TestInvalidTabStopsAreAtomic(t *testing.T) {
	for _, stops := range [][]TabStop{{{Position: -1, Align: "left"}}, {{Position: 28801, Align: "left"}}, {{Position: 720, Align: "bad"}}, {{Position: 720, Align: "left", Leader: "bad"}}, {{Position: 720, Align: "clear", Leader: "dot"}}, {{Position: 720, Align: "left"}, {Position: 720, Align: "right"}}} {
		s, path := withDocument(t, bodyXML(`<w:p><w:r><w:t>hello</w:t></w:r></w:p>`))
		v := mustSnapshot(t, s)
		p := v.Blocks[0].Paragraph
		before := fileBytes(t, path)
		_, err := s.Apply(context.Background(), paragraphPlan(v, ParagraphContent{SourceID: p.ID, Format: &ParagraphFormat{Tabs: stops}, Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: "changed"}}}), "human")
		if err == nil || !bytes.Equal(before, fileBytes(t, path)) || mustSnapshot(t, s).Revision != v.Revision {
			t.Fatalf("invalid tabs mutated: %+v", stops)
		}
	}
}

func TestRunReplacementWritesTabElements(t *testing.T) {
	s, path := createTest(t)
	v := mustSnapshot(t, s)
	_, err := s.Apply(context.Background(), batch(v, "replace_run_text", v.Blocks[0].Paragraph.Runs[0], "left\tright\nend"), "human")
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[0].Text != "left\tright\nend" {
		t.Fatal("tab was not preserved")
	}
	if !strings.Contains(string(s.pkg.document), `<dw:tab `) {
		t.Fatal("tab stored as literal text")
	}
}
