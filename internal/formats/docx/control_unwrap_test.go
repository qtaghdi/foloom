package docx

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestUnwrapContentControlKeepsRunsAndPackage(t *testing.T) {
	ctx := context.Background()
	content := `<w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">팀 &amp; 이름 </w:t></w:r>
<w:r><w:rPr><w:i/></w:rPr><w:t>변경 🙂</w:t></w:r>`
	control := `<w:sdt><w:sdtPr><w:id w:val="42"/><w:lock w:val="unlocked"/><w:text/></w:sdtPr><w:sdtContent>` + content + `</w:sdtContent></w:sdt>`
	prefix := `<w:p><w:r><w:t xml:space="preserve">before </w:t></w:r>`
	suffix := `<w:r><w:t xml:space="preserve"> after</w:t></w:r></w:p><w:p>` + chartDeleteDrawing + `</w:p>`
	s, filename := chartDocument(t, prefix+control+suffix, chartTitlePart, embeddedChartData)
	initial, baseline := mustSnapshot(t, s), s.pkg
	original := fileBytes(t, filename)
	run := initial.Blocks[0].Paragraph.Runs[1]
	if run.Object == nil || !run.Object.UnwrapEditable {
		t.Fatal("unlocked control must advertise wrapper removal")
	}
	op := Operation{Type: "unwrap_control", TargetID: run.ID}
	input := ApplyInput{SessionID: initial.ID, BaseRevision: initial.Revision, Operations: []Operation{op}}
	view, err := s.Apply(ctx, input, "human")
	if err != nil {
		t.Fatal(err)
	}
	if view.Revision != initial.Revision+1 || view.Sequence != initial.Sequence+1 || s.events[0].TextOnly {
		t.Fatal("control removal must advance the structural revision")
	}
	if _, err := s.Apply(ctx, input, "ai"); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale removal returned %v", err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	runs := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs
	if len(runs) != 4 || runs[1].Object != nil || !runs[1].Bold || !runs[1].Editable || runs[1].Text != "팀 & 이름 " ||
		runs[2].Object != nil || !runs[2].Italic || !runs[2].Editable || runs[2].Text != "변경 🙂" {
		t.Fatalf("plain preserved runs were not editable after reopening: %+v", runs)
	}
	if string(reopened.pkg.document) != bodyXML(prefix+content+suffix) || len(baseline.parts) != len(reopened.pkg.parts) {
		t.Fatal("wrapper removal changed neighboring XML or package entries")
	}
	for _, part := range baseline.parts {
		if part.header.Name != "word/document.xml" && !bytes.Equal(reopened.pkg.partData(part.header.Name, nil), part.data) {
			t.Fatalf("wrapper removal changed %s", part.header.Name)
		}
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore exact original package")
	}
}

func TestUnwrapContentControlPreservesScopedNamespaces(t *testing.T) {
	const control = `<c:sdt xmlns:c="` + wordNS + `"><c:sdtPr><c:text/></c:sdtPr><c:sdtContent xmlns="` + wordNS + `"><r/><r><rPr><c:b/></rPr><t>Preserved prefix</t></r></c:sdtContent></c:sdt>`
	s, _ := withDocument(t, bodyXML("<w:p>"+control+"</w:p><w:p/>"))
	ctx := context.Background()
	view := mustSnapshot(t, s)
	if _, err := s.Apply(ctx, ApplyInput{SessionID: view.ID, BaseRevision: view.Revision, Operations: []Operation{
		{Type: "insert_control_text", TargetID: view.Blocks[1].ID, Text: "New control"},
	}}, "human"); err != nil {
		t.Fatal(err)
	}
	for i, text := range []string{"Preserved prefix", "New control"} {
		view = mustSnapshot(t, s)
		run := view.Blocks[i].Paragraph.Runs[0]
		if run.Object == nil || !run.Object.UnwrapEditable {
			t.Fatalf("control %q did not advertise removal", text)
		}
		after, err := s.Apply(ctx, ApplyInput{SessionID: view.ID, BaseRevision: view.Revision, Operations: []Operation{
			{Type: "unwrap_control", TargetID: run.ID},
		}}, "human")
		if err != nil {
			t.Fatal(err)
		}
		runs := after.Blocks[i].Paragraph.Runs
		plain := runs[len(runs)-1]
		if plain.Object != nil || plain.Text != text || !plain.Editable || plain.Bold != (i == 0) {
			t.Fatalf("namespace scope lost when unwrapping %q: %+v", text, plain)
		}
	}
	if strings.Contains(string(s.pkg.document), "sdt") {
		t.Fatal("unwrapping did not remove the control elements")
	}
}

func TestUnwrapContentControlRejectsProtectedSources(t *testing.T) {
	const control = `<w:sdt><w:sdtPr><w:text/></w:sdtPr><w:sdtContent><w:r><w:t>original</w:t></w:r></w:sdtContent></w:sdt>`
	for _, tc := range []struct {
		name, body string
		paragraph  int
	}{
		{"control locked", "<w:p>" + strings.Replace(control, "<w:text/>", `<w:lock w:val="sdtLocked"/><w:text/>`, 1) + "</w:p>", 0},
		{"content locked", "<w:p>" + strings.Replace(control, "<w:text/>", `<w:lock w:val="contentLocked"/><w:text/>`, 1) + "</w:p>", 0},
		{"both locked", "<w:p>" + strings.Replace(control, "<w:text/>", `<w:lock w:val="sdtContentLocked"/><w:text/>`, 1) + "</w:p>", 0},
		{"bound", "<w:p>" + strings.Replace(control, "<w:text/>", `<w:dataBinding w:xpath="/secret"/><w:text/>`, 1) + "</w:p>", 0},
		{"placeholder", "<w:p>" + strings.Replace(control, "<w:text/>", "<w:showingPlcHdr/><w:text/>", 1) + "</w:p>", 0},
		{"wrapper formatting", "<w:p>" + strings.Replace(control, "<w:text/>", "<w:rPr><w:b/></w:rPr><w:text/>", 1) + "</w:p>", 0},
		{"end formatting", "<w:p>" + strings.Replace(control, "<w:sdtContent>", "<w:sdtEndPr><w:rPr><w:b/></w:rPr></w:sdtEndPr><w:sdtContent>", 1) + "</w:p>", 0},
		{"non-namespace scope", "<w:p>" + strings.Replace(control, "<w:sdt>", `<w:sdt xml:lang="ko">`, 1) + "</w:p>", 0},
		{"inside comment range", `<w:p><w:commentRangeStart w:id="7"/></w:p><w:p>` + control + `</w:p><w:p><w:commentRangeEnd w:id="7"/></w:p>`, 1},
		{"permission range", `<w:p><w:permStart w:id="7"/>` + control + `<w:permEnd w:id="7"/></w:p>`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, filename := withDocument(t, bodyXML(tc.body))
			assertControlUnwrapRejected(t, s, filename, tc.paragraph)
		})
	}

	_, filename := withDocument(t, bodyXML("<w:p>"+control+"</w:p>"))
	rewritePackage(t, filename, func(_ string, data []byte) []byte { return data }, map[string][]byte{
		documentRelationships:         []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="settings" Type="` + officeRelNS + `/settings" Target="protected-settings.xml"/></Relationships>`),
		"word/protected-settings.xml": []byte(`<w:settings xmlns:w="` + wordNS + `"><w:documentProtection w:edit="readOnly" w:enforcement="true"/></w:settings>`),
	})
	protected, err := Open(context.Background(), filename)
	if err != nil {
		t.Fatal(err)
	}
	assertControlUnwrapRejected(t, protected, filename, 0)
}

func assertControlUnwrapRejected(t *testing.T, s *Session, filename string, paragraph int) {
	t.Helper()
	initial, original := mustSnapshot(t, s), fileBytes(t, filename)
	var id string
	for _, run := range initial.Blocks[paragraph].Paragraph.Runs {
		if run.Object != nil && run.Object.Kind == "control" {
			id = run.ID
			if run.Object.UnwrapEditable {
				t.Fatal("protected content control must not offer wrapper removal")
			}
		}
	}
	if id == "" {
		t.Fatal("protected content control was not preserved in snapshot")
	}
	_, err := s.Apply(context.Background(), ApplyInput{SessionID: initial.ID, BaseRevision: initial.Revision, Operations: []Operation{
		{Type: "unwrap_control", TargetID: id},
	}}, "ai")
	if err == nil || !bytes.Equal(original, fileBytes(t, filename)) || mustSnapshot(t, s).Revision != initial.Revision {
		t.Fatal("protected removal changed original package or revision")
	}
}
