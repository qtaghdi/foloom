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

func TestImportedTabRunReplacesCompleteText(t *testing.T) {
	for _, test := range []struct {
		name, markup, before, after string
		cell                        bool
	}{
		{"body around tab", `<w:t>left</w:t><w:tab/><w:t>right</w:t>`, "left\tright", "new\ttail", false},
		{"cell with tabs and line break", `<w:t>left</w:t><w:tab/><w:tab/><w:t>right</w:t><w:br/><w:t>end</w:t>`, "left\t\tright\nend", "left\t\tright\nnew", true},
		{"tab only", `<w:tab/>`, "\t", "\ttext", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			paragraph := `<w:p><w:r><w:rPr><w:b/></w:rPr>` + test.markup + `</w:r><w:r><w:t>untouched</w:t></w:r></w:p>`
			operation := "replace_run_text"
			targetParagraph := func(snapshot Snapshot) Paragraph { return *snapshot.Blocks[0].Paragraph }
			if test.cell {
				paragraph = `<w:tbl><w:tr><w:tc>` + paragraph + `</w:tc></w:tr></w:tbl>`
				operation = "replace_cell_text"
				targetParagraph = func(snapshot Snapshot) Paragraph { return snapshot.Blocks[0].Rows[0].Cells[0].Paragraphs[0] }
			}
			session, path := withDocument(t, bodyXML(paragraph))
			original := fileBytes(t, path)
			snapshot := mustSnapshot(t, session)
			run := targetParagraph(snapshot).Runs[0]
			if run.Text != test.before || !run.Editable {
				t.Fatalf("wrong imported run: %+v", run)
			}
			updated, err := session.Apply(context.Background(), batch(snapshot, operation, run, test.after), "human")
			if err != nil {
				t.Fatal(err)
			}
			assertRuns := func(snapshot Snapshot) {
				t.Helper()
				runs := targetParagraph(snapshot).Runs
				if len(runs) != 2 || runs[0].Text != test.after || !runs[0].Bold || runs[1].Text != "untouched" {
					t.Fatalf("tab replacement changed unrelated content: %+v", runs)
				}
			}
			assertRuns(updated)
			reopened, err := Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			assertRuns(mustSnapshot(t, reopened))
			if strings.Count(string(reopened.pkg.document), "<dw:tab ") != strings.Count(test.after, "\t") {
				t.Fatal("native tab was duplicated or lost")
			}
			historyRestore(t, reopened, "undo")
			if !bytes.Equal(original, fileBytes(t, path)) {
				t.Fatal("undo changed the original document")
			}
		})
	}
}
