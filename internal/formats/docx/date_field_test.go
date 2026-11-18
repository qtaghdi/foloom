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
