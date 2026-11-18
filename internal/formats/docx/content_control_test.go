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

func TestPlainContentControlRetainsUnaffectedRunFormatting(t *testing.T) {
	ctx := context.Background()
	control := `<w:sdt><w:sdtPr><w:id w:val="42"/><w:text/></w:sdtPr><w:sdtContent>` +
		`<w:r><w:rPr/></w:r><w:r><w:rPr><w:b/></w:rPr><w:t>팀🙂 이름</w:t></w:r>` +
		`<w:r><w:rPr><w:i/></w:rPr><w:t xml:space="preserve"> 변경 확인</w:t></w:r>` +
		`<w:r><w:t>!</w:t></w:r></w:sdtContent></w:sdt>`
	s, filename := withDocument(t, bodyXML(`<w:p>`+control+`</w:p>`))
	original := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	run := v.Blocks[0].Paragraph.Runs[0]
	if run.Object == nil || !run.Object.Editable || run.Object.Text != "팀🙂 이름 변경 확인!" {
		t.Fatal("multi-run content control was not exposed")
	}
	op := Operation{Type: "update_control_text", TargetID: run.ID, ExpectedText: run.Object.Text, Text: "팀🙂 새 확인!"}
	if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}, "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	saved := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[0]
	want := `<w:r><w:rPr/></w:r><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">팀🙂 새</w:t></w:r>` +
		`<w:r><w:rPr><w:i/></w:rPr><w:t xml:space="preserve"> 확인</w:t></w:r><w:r><w:t>!</w:t></w:r>`
	if saved.Object == nil || saved.Object.Text != op.Text || !strings.Contains(string(reopened.pkg.document), want) {
		t.Fatal("the edit moved untouched text across formatting runs")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo changed original package bytes")
	}
}

func TestPlainContentControlInsertionRetainsDocumentAndCanBeEdited(t *testing.T) {
	ctx := context.Background()
	originalControl := `<w:sdt><w:sdtPr><w:id w:val="1"/><w:text/></w:sdtPr><w:sdtContent><w:r><w:t>Existing</w:t></w:r></w:sdtContent></w:sdt>`
	originalRun := `<w:r><w:rPr><w:b/></w:rPr><w:t>Prefix</w:t></w:r>`
	_, filename := withDocument(t, bodyXML(`<w:p>`+originalRun+`</w:p><w:p/>`+
		`<w:p>`+originalControl+`</w:p>`))
	rewritePackage(t, filename, func(_ string, data []byte) []byte { return data }, map[string][]byte{
		"word/header1.xml": []byte(`<h:hdr xmlns:h="` + wordNS + `"><h:p><h:sdt><h:sdtPr><h:id h:val="2"/><h:text/></h:sdtPr><h:sdtContent><h:r><h:t>Header</h:t></h:r></h:sdtContent></h:sdt></h:p></h:hdr>`),
	})
	original := fileBytes(t, filename)
	s, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v := mustSnapshot(t, s)
	value := " 새 콘텐츠 & <값>🙂 "
	next, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "insert_control_text", TargetID: v.Blocks[0].ID, Text: value},
		{Type: "insert_control_text", TargetID: v.Blocks[1].ID, Text: "Second control"},
	}}, "ai")
	if err != nil {
		t.Fatal(err)
	}
	if next.Revision != v.Revision+1 || next.Sequence != v.Sequence+1 ||
		len(s.events) != 1 || s.events[0].TextOnly {
		t.Fatal("insertion did not commit as one structural change")
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	saved := mustSnapshot(t, reopened)
	run := saved.Blocks[0].Paragraph.Runs[1]
	if run.Object == nil || !run.Object.Editable || run.Object.Text != value ||
		!strings.Contains(string(reopened.pkg.document), originalControl) ||
		!strings.Contains(string(reopened.pkg.document), originalRun) ||
		!strings.Contains(string(reopened.pkg.document), `<dws:id dws:val="3"/>`) ||
		!strings.Contains(string(reopened.pkg.document), `<dws:id dws:val="4"/>`) {
		t.Fatal("insertion failed to preserve source content, unique IDs or initial text")
	}
	before, err := loadPackage(original)
	if err != nil {
		t.Fatal(err)
	}
	for i, item := range before.parts {
		if item.header.Name != "word/document.xml" && !bytes.Equal(item.data, reopened.pkg.parts[i].data) {
			t.Fatalf("insertion changed unrelated part %s", item.header.Name)
		}
	}
	updated, err := reopened.Apply(ctx, ApplyInput{SessionID: saved.ID, BaseRevision: saved.Revision, Operations: []Operation{
		{Type: "update_control_text", TargetID: run.ID, ExpectedText: value, Text: "Updated control"},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Blocks[0].Paragraph.Runs[1].Object.Text != "Updated control" {
		t.Fatal("inserted control could not be edited")
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore original package bytes")
	}
}

func TestPlainContentControlInsertionRejectsInvalidBatches(t *testing.T) {
	for _, tc := range []struct {
		name     string
		block    int
		text     string
		expected string
	}{
		{name: "tracked paragraph", block: 2, text: "invalid target"},
		{name: "blank", block: 1, text: " "},
		{name: "multiline", block: 1, text: "a\nb"},
		{name: "too long", block: 1, text: strings.Repeat("한", 342)},
		{name: "unexpected match text", block: 1, text: "valid", expected: "stale"},
		{name: "duplicate target", block: 0, text: "valid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, filename := withDocument(t, bodyXML(`<w:p/><w:p/>`+
				`<w:p><w:ins w:id="1"><w:r><w:t>Tracked</w:t></w:r></w:ins></w:p>`))
			original := fileBytes(t, filename)
			v := mustSnapshot(t, s)
			_, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
				{Type: "insert_control_text", TargetID: v.Blocks[0].ID, Text: "Control"},
				{Type: "insert_control_text", TargetID: v.Blocks[tc.block].ID, Text: tc.text, ExpectedText: tc.expected},
			}}, "human")
			if err == nil || mustSnapshot(t, s).Revision != v.Revision || !bytes.Equal(original, fileBytes(t, filename)) {
				t.Fatal("invalid content control insertion modified the document")
			}
		})
	}
}
