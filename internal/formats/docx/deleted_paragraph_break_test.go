package docx

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

const deletedBreakMarker = `<w:rPr><w:del w:id="73" w:author="Writer" w:date="2026-11-18T02:00:00Z"/></w:rPr>`

func TestReviewDeletedParagraphBreakKeepsFollowingParagraphProperties(t *testing.T) {
	ctx := context.Background()
	left := `<w:p xmlns:w14="` + word2010NS + `" xmlns:q="` + wordNS + `" w14:paraId="5A17ABCD"><w:pPr><w:jc w:val="center"/>` +
		deletedBreakMarker + `</w:pPr><q:r><q:rPr><q:b/></q:rPr><q:t xml:space="preserve">First </q:t></q:r></w:p>`
	right := `<w:p xmlns:w14="` + word2010NS + `" w14:paraId="1A17ABCD"><w:pPr><w:jc w:val="right"/></w:pPr><w:r><w:rPr><w:i/></w:rPr><w:t>Second</w:t></w:r></w:p>`
	for _, operation := range []string{"accept_revision", "reject_revision"} {
		t.Run(operation, func(t *testing.T) {
			s, path := withDocument(t, bodyXML(left+"\n"+right))
			prior := fileBytes(t, path)
			v := mustSnapshot(t, s)
			mark := v.Blocks[0].Paragraph.BreakRevision
			if !v.TrackedChanges || mark == nil || !mark.Editable || mark.Kind != "tracked_delete" ||
				mark.Author != "Writer" || mark.Date != "2026-11-18T02:00:00Z" || mark.Text != "First \nSecond" {
				t.Fatalf("deleted paragraph break was not exposed: %+v", mark)
			}
			op := Operation{Type: operation, TargetID: v.Blocks[0].ID}
			apply := func(base uint64, operations ...Operation) error {
				_, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: base, Operations: operations}, "human")
				return err
			}
			if err := apply(v.Revision+1, op); !errors.Is(err, ErrRevisionConflict) {
				t.Fatal("wrong revision was accepted")
			}
			if err := apply(v.Revision, op, op); err == nil || !bytes.Equal(prior, fileBytes(t, path)) {
				t.Fatal("batched review changed the document")
			}
			if err := apply(v.Revision, op); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			saved := mustSnapshot(t, reopened)
			first := saved.Blocks[0].Paragraph
			if saved.TrackedChanges || first.BreakRevision != nil || first.Runs[0].Text != "First " || !first.Runs[0].Bold {
				t.Fatal("review lost source text or left the marker")
			}
			if operation == "accept_revision" {
				if len(saved.Blocks) != 1 || first.Anchor != v.Blocks[1].Paragraph.Anchor || first.Layout.Align != "end" ||
					len(first.Runs) != 2 || first.Runs[1].Text != "Second" || !first.Runs[1].Italic {
					t.Fatal("accept did not preserve the following paragraph identity, properties and run styling")
				}
			} else if len(saved.Blocks) != 2 || first.Anchor != v.Blocks[0].Paragraph.Anchor || first.Layout.Align != "center" ||
				!strings.Contains(string(reopened.pkg.document), strings.Replace(left, deletedBreakMarker, "", 1)+"\n"+right) {
				t.Fatal("reject did not restore the two original paragraph boundaries and retain their properties")
			}
			historyRestore(t, reopened, "undo")
			if !bytes.Equal(prior, fileBytes(t, path)) {
				t.Fatal("undo did not restore the exact native deletion marker")
			}
		})
	}
}

func TestReviewDeletedParagraphBreakProtectsComplexPairs(t *testing.T) {
	left := `<w:p><w:pPr>` + deletedBreakMarker + `</w:pPr><w:r><w:t>First</w:t></w:r></w:p>`
	right := `<w:p><w:r><w:t>Second</w:t></w:r></w:p>`
	for name, xml := range map[string]string{
		"duplicate":          left + right + `<w:p><w:del w:id="73"><w:r><w:delText>Third</w:delText></w:r></w:del></w:p>`,
		"page":               left + `<w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:t>Second</w:t></w:r></w:p>`,
		"field":              left + `<w:p><w:fldSimple w:instr="PAGE"><w:r><w:t>1</w:t></w:r></w:fldSimple></w:p>`,
		"complex properties": left + `<w:p><w:pPr><w:ind w:left="50"/></w:pPr><w:r><w:t>Second</w:t></w:r></w:p>`,
		"invalid alignment":  left + `<w:p><w:pPr><w:jc w:val="unknown"/></w:pPr><w:r><w:t>Second</w:t></w:r></w:p>`,
		"range": left[:len(left)-len("</w:p>")] + `<w:bookmarkStart w:id="17" w:name="Protected"/></w:p>` +
			right[:len(right)-len("</w:p>")] + `<w:bookmarkEnd w:id="17"/></w:p>`,
	} {
		t.Run(name, func(t *testing.T) {
			s, path := withDocument(t, bodyXML(xml))
			prior := fileBytes(t, path)
			v := mustSnapshot(t, s)
			change := v.Blocks[0].Paragraph.BreakRevision
			if change == nil || change.Editable {
				t.Fatal("protected deletion not shown as read-only")
			}
			for _, op := range []string{"accept_revision", "reject_revision"} {
				if _, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
					{Type: op, TargetID: v.Blocks[0].ID},
				}}, "human"); err == nil || !bytes.Equal(prior, fileBytes(t, path)) {
					t.Fatal("protected deletion changed the original document")
				}
			}
		})
	}
}
