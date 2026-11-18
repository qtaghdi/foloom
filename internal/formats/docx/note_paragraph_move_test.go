package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestMoveNoteParagraphPreservesBothNoteParts(t *testing.T) {
	ctx := context.Background()
	session, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>Before</w:t></w:r></w:p>
<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>Footnote target</w:t></w:r></w:p>
<w:p><w:r><w:t>Between</w:t></w:r></w:p>
<w:p><w:r><w:t>Endnote target</w:t></w:r></w:p>
<w:p><w:r><w:t>After</w:t></w:r></w:p>`))
	v := mustSnapshot(t, session)
	v, err := session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: v.Blocks[1].ID, Reference: &ReferenceEdit{Action: "insert_footnote", Text: "Footnote source"}},
		{Type: "update_reference", TargetID: v.Blocks[3].ID, Reference: &ReferenceEdit{Action: "insert_endnote", Text: "Endnote source"}},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocks[1].Paragraph.MoveNoteUp || !v.Blocks[3].Paragraph.MoveNoteDown {
		t.Fatal("safe note-bearing paragraphs are not movable")
	}
	before := fileBytes(t, filename)
	parts := map[string][]byte{documentRelationships: session.pkg.partData(documentRelationships, nil)}
	for _, kind := range []string{"footnote", "endnote"} {
		path, notes, err := session.pkg.notesPart(kind, nil)
		if err != nil || notes == nil {
			t.Fatalf("%s part unavailable: %v", kind, err)
		}
		parts[path] = notes.source
	}
	ops := []Operation{
		{Type: "move_note_up", TargetID: v.Blocks[1].ID},
		{Type: "move_note_down", TargetID: v.Blocks[3].ID},
	}
	if _, err := session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision - 1, Operations: ops}, "human"); err == nil ||
		!bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("stale request changed the DOCX")
	}
	invalid := append([]Operation(nil), ops...)
	invalid[1].Text = "unexpected"
	if _, err := session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: invalid}, "human"); err == nil ||
		!bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("invalid request partially moved the note paragraphs")
	}
	if _, err := session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: ops}, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	after := mustSnapshot(t, reopened)
	for i, label := range []string{"Footnote target", "Before", "Between", "After", "Endnote target"} {
		if after.Blocks[i].Paragraph.Runs[0].Text != label {
			t.Fatalf("unexpected paragraph order at %d", i)
		}
	}
	foot := after.Blocks[0].Paragraph.Runs
	end := after.Blocks[4].Paragraph.Runs
	if !foot[0].Bold || foot[1].Object == nil || foot[1].Object.Kind != "footnote" || foot[1].Object.noteID != "1" || foot[1].Object.Text != "Footnote source" ||
		end[1].Object == nil || end[1].Object.Kind != "endnote" || end[1].Object.noteID != "1" || end[1].Object.Text != "Endnote source" {
		t.Fatal("note marker, content, or source formatting changed")
	}
	for path, data := range parts {
		if !bytes.Equal(data, reopened.pkg.partData(path, nil)) {
			t.Fatalf("moving a note-bearing paragraph changed %s", path)
		}
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the DOCX exactly")
	}
}

func TestMoveNoteParagraphRejectsBlockedBoundariesAndReferences(t *testing.T) {
	ctx := context.Background()
	neighbor := `<w:p><w:r><w:t>Neighbor</w:t></w:r></w:p>`
	for name, modify := range map[string]func(string) string{
		"page": func(body string) string {
			return strings.Replace(body, neighbor, `<w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:t>Neighbor</w:t></w:r></w:p>`, 1)
		},
		"section": func(body string) string {
			return strings.Replace(body, neighbor, `<w:p><w:pPr><w:sectPr/></w:pPr><w:r><w:t>Neighbor</w:t></w:r></w:p>`, 1)
		},
		"table": func(body string) string {
			return strings.Replace(body, neighbor, `<w:tbl><w:tr><w:tc>`+neighbor+`</w:tc></w:tr></w:tbl>`, 1)
		},
		"shared reference": func(body string) string {
			return strings.Replace(body, neighbor, neighbor+`<w:p><w:r><w:footnoteReference w:id="1"/></w:r></w:p>`, 1)
		},
		"dangling reference": func(body string) string {
			return strings.Replace(body, `footnoteReference dwn:id="1"`, `footnoteReference dwn:id="2"`, 1)
		},
		"custom mark": func(body string) string {
			return strings.Replace(body, `footnoteReference dwn:id="1"`, `footnoteReference dwn:id="1" dwn:customMarkFollows="true"`, 1)
		},
		"field": func(body string) string {
			return strings.Replace(body, `<w:t>Footnote target</w:t></w:r>`, `<w:t>Footnote target</w:t></w:r><w:fldSimple w:instr="PAGE"><w:r><w:t>1</w:t></w:r></w:fldSimple>`, 1)
		},
		"tracked change": func(body string) string {
			return strings.Replace(body, `<w:t>Footnote target</w:t></w:r>`, `<w:t>Footnote target</w:t></w:r><w:ins w:id="1"><w:r><w:t>revision</w:t></w:r></w:ins>`, 1)
		},
		"comment range": func(body string) string {
			body = strings.Replace(body, "<w:body>", `<w:body><w:p><w:commentRangeStart w:id="2"/></w:p>`, 1)
			return strings.Replace(body, "</w:body>", `<w:p><w:commentRangeEnd w:id="2"/></w:p></w:body>`, 1)
		},
		"bookmark range": func(body string) string {
			body = strings.Replace(body, "<w:body>", `<w:body><w:p><w:bookmarkStart w:id="2" w:name="Range"/></w:p>`, 1)
			return strings.Replace(body, "</w:body>", `<w:p><w:bookmarkEnd w:id="2"/></w:p></w:body>`, 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			session, filename := withDocument(t, bodyXML(neighbor+`<w:p><w:r><w:t>Footnote target</w:t></w:r></w:p>`))
			v := mustSnapshot(t, session)
			if _, err := session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
				{Type: "update_reference", TargetID: v.Blocks[1].ID, Reference: &ReferenceEdit{Action: "insert_footnote", Text: "Source"}},
			}}, "human"); err != nil {
				t.Fatal(err)
			}
			rewritePackage(t, filename, func(part string, data []byte) []byte {
				if part == "word/document.xml" {
					return []byte(modify(string(data)))
				}
				return data
			}, nil)
			before := fileBytes(t, filename)
			reopened, err := Open(ctx, filename)
			if err != nil {
				t.Fatal(err)
			}
			v = mustSnapshot(t, reopened)
			for _, block := range v.Blocks {
				p := block.Paragraph
				if p == nil || len(p.Runs) == 0 || p.Runs[0].Text != "Footnote target" {
					continue
				}
				if p.MoveNoteUp || p.MoveNoteDown {
					t.Fatal("protected note-bearing paragraph was exposed as movable")
				}
				if _, err := reopened.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
					{Type: "move_note_up", TargetID: block.ID},
				}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
					t.Fatal("protected note-bearing paragraph changed")
				}
				return
			}
			t.Fatal("fixture lost its note-bearing paragraph")
		})
	}
}
