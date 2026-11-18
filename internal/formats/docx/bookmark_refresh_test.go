package docx

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRefreshBookmarkReferencePersistsCurrentSourceAndUndoes(t *testing.T) {
	ctx := context.Background()
	session, filename := withDocument(t, bodyXML(`<w:p><w:bookmarkStart w:id="1" w:name="Source"/><w:r><w:t>before</w:t></w:r><w:bookmarkEnd w:id="1"/></w:p>
<w:p><w:r><w:t>See </w:t></w:r><w:fldSimple w:instr="REF Source \h"><w:r><w:rPr><w:i/></w:rPr><w:t xml:space="preserve">before</w:t></w:r></w:fldSimple>
<w:fldSimple w:instr="PAGEREF Source \h"><w:r><w:t>7</w:t></w:r></w:fldSimple></w:p>`))
	v := mustSnapshot(t, session)
	run := v.Blocks[0].Paragraph.Runs[0]
	v, err := session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "replace_run_text", TargetID: run.ID, ExpectedText: run.Text, Text: "새 근거 & 범위"},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	ref := v.Blocks[1].Paragraph.Runs[1]
	if ref.Link == nil || !ref.Link.RefreshNeeded || ref.Text != "새 근거 & 범위" {
		t.Fatal("pending bookmark cache was not exposed")
	}
	page := v.Blocks[1].Paragraph.Runs[2]
	if page.Link == nil || page.Link.RefreshNeeded || page.Text != "7" {
		t.Fatal("page field was changed or exposed as a pending text reference")
	}
	before := fileBytes(t, filename)
	xml := string(session.pkg.document)
	apply := func(ops ...Operation) (Snapshot, error) {
		return session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: ops}, "human")
	}
	op := Operation{Type: "update_reference", TargetID: ref.ID, Reference: &ReferenceEdit{Action: "refresh_bookmark_ref"}}
	_, stale := session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision - 1, Operations: []Operation{op}}, "human")
	if !errors.Is(stale, ErrRevisionConflict) {
		t.Fatal("old base revision was accepted")
	}
	if _, err := apply(op, op); err == nil {
		t.Fatal("REF cache refresh was permitted in a batch")
	}
	wrong := op
	wrong.Reference = &ReferenceEdit{Action: "refresh_bookmark_ref", Bookmark: "source"}
	if _, err := apply(wrong); err == nil {
		t.Fatal("unexpected REF data was accepted")
	}
	wrong = op
	wrong.TargetID = page.ID
	if _, err := apply(wrong); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("REF refresh accepted a page field or affected source bytes")
	}

	updated, err := apply(op)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	after := mustSnapshot(t, reopened).Blocks[1].Paragraph.Runs[1]
	if updated.Revision != v.Revision+1 || after.Link == nil || after.Link.RefreshNeeded || after.Text != ref.Text {
		t.Fatal("REF cache refresh did not persist")
	}
	want := strings.Replace(xml, `xml:space="preserve">before`, `xml:space="preserve">새 근거 &amp; 범위`, 1)
	if string(reopened.pkg.document) != want {
		t.Fatal("refresh changed Word code, another field, source text or cache style")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) ||
		!mustSnapshot(t, reopened).Blocks[1].Paragraph.Runs[1].Link.RefreshNeeded {
		t.Fatal("undo did not restore the old cache and pending status")
	}
}

func TestRefreshBookmarkReferenceRejectsProtectedFields(t *testing.T) {
	body := `<w:p><w:bookmarkStart w:id="1" w:name="Source"/><w:r><w:t>current</w:t></w:r><w:bookmarkEnd w:id="1"/></w:p>`
	makeField := func(code, attrs, cache string) string {
		return `<w:p><w:fldSimple w:instr="` + escapeText(code) + `"` + attrs + `><w:r><w:t>` + cache + `</w:t></w:r></w:fldSimple></w:p>`
	}
	for name, xml := range map[string]string{
		"locked":    body + makeField(`REF Source \h`, ` w:fldLock="true"`, "stale"),
		"external":  body + makeField(`INCLUDETEXT "https://example.com/word.docx"`, "", "stale"),
		"comment":   body + makeField(`REF Source \h`, "", "stale<!-- keep -->"),
		"number":    body + makeField(`REF Source \n \h`, "", "stale"),
		"page":      body + makeField(`PAGEREF Source \h`, "", "stale"),
		"missing":   body + makeField(`REF Missing \h`, "", "stale"),
		"ambiguous": body + `<w:p><w:bookmarkStart w:id="2" w:name="source"/><w:bookmarkEnd w:id="2"/></w:p>` + makeField(`REF Source \h`, "", "stale"),
		"complex":   body + `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText>REF Source \h</w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>stale</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`,
	} {
		t.Run(name, func(t *testing.T) {
			session, filename := withDocument(t, bodyXML(xml))
			before := fileBytes(t, filename)
			v := mustSnapshot(t, session)
			for _, run := range v.Blocks[len(v.Blocks)-1].Paragraph.Runs {
				if run.Link != nil && run.Link.RefreshNeeded {
					t.Fatal("protected REF was exposed as pending")
				}
				if _, err := session.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
					{Type: "update_reference", TargetID: run.ID, Reference: &ReferenceEdit{Action: "refresh_bookmark_ref"}},
				}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
					t.Fatal("protected REF refresh changed the DOCX")
				}
			}
		})
	}
}
