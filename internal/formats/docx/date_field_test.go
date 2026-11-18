package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestInsertDateFieldPersistsAndUndoes(t *testing.T) {
	ctx := context.Background()
	session, filename := withDocument(t, bodyXML(`<w:p/><w:p><w:r><w:t>Keep me</w:t></w:r></w:p>`))
	before := fileBytes(t, filename)
	v := mustSnapshot(t, session)
	insert := func(edit ReferenceEdit) (Snapshot, error) {
		return session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
			{Type: "update_reference", TargetID: v.Blocks[0].ID, Reference: &edit},
		}}, "human")
	}
	if _, err := insert(ReferenceEdit{Action: "insert_date_field", Text: "arbitrary code"}); err == nil ||
		!bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("unexpected data changed the document")
	}

	beforeDate := time.Now().UTC().Format(dateFieldLayout)
	if _, err := insert(ReferenceEdit{Action: "insert_date_field"}); err != nil {
		t.Fatal(err)
	}
	afterDate := time.Now().UTC().Format(dateFieldLayout)
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	saved := mustSnapshot(t, reopened)
	run := saved.Blocks[0].Paragraph.Runs[0]
	if run.Editable || run.Object == nil || run.Object.Kind != "date_field" || run.Object.Text != run.Text ||
		run.Text != beforeDate && run.Text != afterDate ||
		saved.Blocks[1].Paragraph.Runs[0].Text != "Keep me" ||
		!strings.Contains(string(reopened.pkg.document), escapeText(dateFieldInstruction)) {
		t.Fatalf("unexpected native date field: %+v", run)
	}

	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original source")
	}
}

func TestDateFieldFormatsUTC(t *testing.T) {
	now := time.Date(2026, time.November, 18, 1, 0, 0, 0, time.FixedZone("UTC+09", 9*60*60))
	if !strings.Contains(dateFieldXML(now), ">2026-11-17<") {
		t.Fatal("date field must use the server's UTC calendar date")
	}
}

func TestRefreshDateFieldKeepsInstructionAndFormatting(t *testing.T) {
	ctx := context.Background()
	field := `<w:fldSimple w:dirty="true" w:instr="  Date   \@ &quot;yyyy-MM-dd&quot;  "><!-- keep note --><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">2000-02-29</w:t></w:r></w:fldSimple>`
	body := bodyXML(`<w:p><w:r><w:t>Date: </w:t></w:r>` + field + `<w:r><w:t>.</w:t></w:r></w:p>`)
	session, filename := withDocument(t, body)
	before := fileBytes(t, filename)
	v := mustSnapshot(t, session)
	run := v.Blocks[0].Paragraph.Runs[1]
	if run.Editable || run.Object == nil || !run.Object.Editable || run.Object.Text != "2000-02-29" {
		t.Fatalf("unexpected date field: %+v", run)
	}
	refresh := func(v Snapshot, base uint64, edit ReferenceEdit) (Snapshot, error) {
		return session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: base, Operations: []Operation{
			{Type: "update_reference", TargetID: v.Blocks[0].Paragraph.Runs[1].ID, Reference: &edit},
		}}, "human")
	}
	edit := ReferenceEdit{Action: "refresh_date_field"}
	if _, err := refresh(v, v.Revision+1, edit); err == nil {
		t.Fatal("stale base revision was accepted")
	}
	if _, err := refresh(v, v.Revision, ReferenceEdit{Action: "refresh_date_field", URL: "https://example.com"}); err == nil {
		t.Fatal("unexpected input was accepted")
	}
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("rejected update changed the document")
	}
	beforeDate := time.Now().UTC().Format(dateFieldLayout)
	v, err := refresh(v, v.Revision, edit)
	if err != nil {
		t.Fatal(err)
	}
	afterDate := time.Now().UTC().Format(dateFieldLayout)
	value := v.Blocks[0].Paragraph.Runs[1].Text
	if value != beforeDate && value != afterDate {
		t.Fatalf("unexpected date: %q", value)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(reopened.pkg.document) != strings.Replace(body, "2000-02-29", value, 1) {
		t.Fatal("refresh changed XML outside the displayed date")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original source")
	}
}

func TestRefreshDateFieldRejectsUnsupportedFields(t *testing.T) {
	instruction := escapeText(dateFieldInstruction)
	for name, field := range map[string]string{
		"locked":           `<w:fldSimple w:fldLock="true" w:instr="` + instruction + `"><w:r><w:t>2000-02-29</w:t></w:r></w:fldSimple>`,
		"nested":           `<w:fldSimple w:instr="` + instruction + `"><w:r><w:t>2000-02-29</w:t><w:instrText>INCLUDETEXT</w:instrText></w:r></w:fldSimple>`,
		"external":         `<w:fldSimple w:instr="INCLUDETEXT &quot;https://example.com/file.docx&quot;"><w:r><w:t>2000-02-29</w:t></w:r></w:fldSimple>`,
		"different format": `<w:fldSimple w:instr="DATE \@ &quot;yyyy-mm-dd&quot;"><w:r><w:t>2000-02-29</w:t></w:r></w:fldSimple>`,
		"complex":          `<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText>` + instruction + `</w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>2000-02-29</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r>`,
	} {
		t.Run(name, func(t *testing.T) {
			session, filename := withDocument(t, bodyXML(`<w:p>`+field+`</w:p>`))
			before := fileBytes(t, filename)
			v := mustSnapshot(t, session)
			for _, run := range v.Blocks[0].Paragraph.Runs {
				if run.Object != nil && run.Object.Editable {
					t.Fatal("protected field was exposed as editable")
				}
				if _, err := session.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
					{Type: "update_reference", TargetID: run.ID, Reference: &ReferenceEdit{Action: "refresh_date_field"}},
				}}, "human"); err == nil {
					t.Fatal("protected field was changed")
				}
			}
			if !bytes.Equal(before, fileBytes(t, filename)) {
				t.Fatal("rejecting a protected field changed the document")
			}
		})
	}
}
