package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestPlainContentControlUpdatesOnlyItsText(t *testing.T) {
	ctx := context.Background()
	control := `<w:sdt><w:sdtPr><w:id w:val="42"/><w:alias w:val="Company"/><w:text/></w:sdtPr><w:sdtContent><w:r><w:rPr><w:b/></w:rPr><w:t>before</w:t></w:r></w:sdtContent></w:sdt>`
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>prefix</w:t></w:r>`+control+`</w:p>`))
	original := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	run := v.Blocks[0].Paragraph.Runs[1]
	if run.Object == nil || !run.Object.Editable || run.Object.Label != "Company" {
		t.Fatal("plain control was not exposed")
	}
	op := Operation{Type: "update_control_text", TargetID: run.ID, ExpectedText: "stale", Text: " 새 값 & safe "}
	batch := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}
	if _, err := s.Apply(ctx, batch, "human"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("stale control edit was applied")
	}
	batch.Operations[0].ExpectedText = "before"
	if _, err := s.Apply(ctx, batch, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	saved := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[1]
	if saved.Object == nil || saved.Object.Text != op.Text ||
		!strings.Contains(string(reopened.pkg.document), `<w:id w:val="42"/><w:alias w:val="Company"/><w:text/>`) ||
		!strings.Contains(string(reopened.pkg.document), `<w:rPr><w:b/></w:rPr>`) {
		t.Fatal("the saved control lost its text or metadata")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo changed original package bytes")
	}
}

func TestContentControlRejectsLockedBoundAndPlaceholderText(t *testing.T) {
	for _, props := range []string{`<w:lock w:val="contentLocked"/>`, `<w:dataBinding w:xpath="/example"/>`, `<w:showingPlcHdr/>`} {
		ctx := context.Background()
		s, filename := withDocument(t, bodyXML(`<w:p><w:sdt><w:sdtPr>`+props+`<w:text/></w:sdtPr><w:sdtContent><w:r><w:t>original</w:t></w:r></w:sdtContent></w:sdt></w:p>`))
		original := fileBytes(t, filename)
		v := mustSnapshot(t, s)
		run := v.Blocks[0].Paragraph.Runs[0]
		if run.Object == nil || run.Object.Editable {
			t.Fatal("protected content control was editable")
		}
		if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
			{Type: "update_control_text", TargetID: run.ID, ExpectedText: "original", Text: "changed"},
		}}, "human"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
			t.Fatal("protected content control was modified")
		}
	}
}
