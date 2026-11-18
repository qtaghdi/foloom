package docx

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

const insertedBreakMarker = `<w:rPr><w:ins w:id="73" w:author="Writer" w:date="2026-11-18T02:00:00Z"/></w:rPr>`

func TestReviewInsertedParagraphBreakMaintainsStyledRuns(t *testing.T) {
	ctx := context.Background()
	for _, operation := range []string{"accept_revision", "reject_revision"} {
		t.Run(operation, func(t *testing.T) {
			left := `<w:p xmlns:w14="` + word2010NS + `" w14:paraId="5A17ABCD"><w:pPr><w:jc w:val="center"/>` + insertedBreakMarker +
				`</w:pPr><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">First </w:t></w:r></w:p>`
			right := `<w:p xmlns:w14="` + word2010NS + `" xmlns:q="` + wordNS +
				`" w14:paraId="1A17ABCD"><w:pPr><w:jc w:val="center"/></w:pPr><q:r><q:rPr><q:i/></q:rPr><q:t>Second</q:t></q:r></w:p>`
			s, path := withDocument(t, bodyXML(left+"\n"+right))
			before := fileBytes(t, path)
			v := mustSnapshot(t, s)
			change := v.Blocks[0].Paragraph.BreakRevision
			if change == nil || !change.Editable || change.Author != "Writer" || change.Date != "2026-11-18T02:00:00Z" ||
				change.Text != "First \nSecond" || !v.TrackedChanges {
				t.Fatalf("paragraph revision missing: %+v", change)
			}
			op := Operation{Type: operation, TargetID: v.Blocks[0].ID}
			apply := func(base uint64, ops ...Operation) (Snapshot, error) {
				return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: base, Operations: ops}, "human")
			}
			if _, err := apply(v.Revision+1, op); !errors.Is(err, ErrRevisionConflict) {
				t.Fatal("wrong revision was accepted")
			}
			if _, err := apply(v.Revision, op, op); err == nil || !bytes.Equal(before, fileBytes(t, path)) {
				t.Fatal("batched review changed the document")
			}
			if _, err := apply(v.Revision, op); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			saved := mustSnapshot(t, reopened)
			first := saved.Blocks[0].Paragraph
			if saved.TrackedChanges || first.BreakRevision != nil || first.Anchor != v.Blocks[0].Paragraph.Anchor ||
				first.Layout == nil || first.Layout.Align != "center" ||
				first.Runs[0].Text != "First " || !first.Runs[0].Bold {
				t.Fatal("review lost the first paragraph or left a tracked marker")
			}
			if operation == "accept_revision" {
				if len(saved.Blocks) != 2 || saved.Blocks[1].Paragraph.Anchor != v.Blocks[1].Paragraph.Anchor ||
					saved.Blocks[1].Paragraph.Runs[0].Text != "Second" || !saved.Blocks[1].Paragraph.Runs[0].Italic ||
					!strings.Contains(string(reopened.pkg.document), right) {
					t.Fatal("accept changed the second paragraph")
				}
			} else if len(saved.Blocks) != 1 || len(first.Runs) != 2 || first.Runs[1].Text != "Second" || !first.Runs[1].Italic {
				t.Fatal("reject did not join both runs and styles")
			}
			historyRestore(t, reopened, "undo")
			if !bytes.Equal(before, fileBytes(t, path)) {
				t.Fatal("undo did not restore the exact review marker and paragraphs")
			}
		})
	}
}

func TestProtectedInsertedParagraphBreakRemainsReadOnly(t *testing.T) {
	left := `<w:p><w:pPr>` + insertedBreakMarker + `</w:pPr><w:r><w:t>First</w:t></w:r></w:p>`
	right := `<w:p><w:r><w:t>Second</w:t></w:r></w:p>`
	for name, xml := range map[string]string{
		"duplicate ID":         left + right + `<w:p><w:ins w:id="73"><w:r><w:t>Later</w:t></w:r></w:ins></w:p>`,
		"different properties": left + `<w:p><w:pPr><w:jc w:val="end"/></w:pPr><w:r><w:t>Second</w:t></w:r></w:p>`,
		"word field":           left + `<w:p><w:fldSimple w:instr="PAGE"><w:r><w:t>Second</w:t></w:r></w:fldSimple></w:p>`,
		"page boundary":        left + `<w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:t>Second</w:t></w:r></w:p>`,
		"opaque gap":           left + `<!-- retain note -->` + right,
		"no next paragraph":    left + `<w:sectPr/>`,
		"spanning range": `<w:p><w:bookmarkStart w:id="6" w:name="Range"/></w:p>` + left + right +
			`<w:p><w:bookmarkEnd w:id="6"/></w:p>`,
	} {
		t.Run(name, func(t *testing.T) {
			s, path := withDocument(t, bodyXML(xml))
			before := fileBytes(t, path)
			v := mustSnapshot(t, s)
			for _, block := range v.Blocks {
				if block.Paragraph == nil || block.Paragraph.BreakRevision == nil {
					continue
				}
				if block.Paragraph.BreakRevision.Editable {
					t.Fatal("protected marker exposed as reviewable")
				}
				for _, op := range []string{"accept_revision", "reject_revision"} {
					if _, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
						{Type: op, TargetID: block.ID},
					}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, path)) {
						t.Fatal("protected marker changed the file")
					}
				}
				return
			}
			t.Fatal("read-only marker was not shown")
		})
	}
}

func TestReviewInsertedParagraphBreakChecksInheritedPageBoundaries(t *testing.T) {
	ctx := context.Background()
	style := `<w:pStyle w:val="NewPage"/>`
	_, path := withDocument(t, bodyXML(`<w:p><w:pPr>`+style+insertedBreakMarker+`</w:pPr><w:r><w:t>First</w:t></w:r></w:p>`+
		`<w:p><w:pPr>`+style+`</w:pPr><w:r><w:t>Second</w:t></w:r></w:p>`))
	rewritePackage(t, path, func(_ string, data []byte) []byte { return data }, map[string][]byte{
		"word/design.xml":              []byte(`<w:styles xmlns:w="` + wordNS + `"><w:style w:type="paragraph" w:styleId="NewPage"><w:pPr><w:pageBreakBefore/></w:pPr></w:style></w:styles>`),
		"word/_rels/document.xml.rels": []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="style" Type="` + officeRelNS + `/styles" Target="design.xml"/></Relationships>`),
	})
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	before := fileBytes(t, path)
	v := mustSnapshot(t, s)
	change := v.Blocks[0].Paragraph.BreakRevision
	if change == nil || change.Editable || !v.Blocks[1].Paragraph.PageBreakBefore {
		t.Fatal("inherited page boundary not reflected in review")
	}
	if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "reject_revision", TargetID: v.Blocks[0].ID},
	}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, path)) {
		t.Fatal("applying review bypassed inherited page protection")
	}
}
