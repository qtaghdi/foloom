package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSquareRootInsertsAndEditsOnlyItsRadicand(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>before root</w:t></w:r></w:p>`))
	before := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	insert := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_math", TargetID: v.Blocks[0].ID, Math: &MathEdit{Action: "insert_root", Expression: "x + 2"}},
	}}
	added, err := s.Apply(ctx, insert, "human")
	if err != nil {
		t.Fatal(err)
	}
	run := added.Blocks[0].Paragraph.Runs[1]
	if run.Object == nil || !run.Object.Editable || run.Object.SquareRoot != "x + 2" ||
		!strings.Contains(string(s.pkg.document), `<dwm:degHide dwm:val="1"/></dwm:radPr><dwm:deg/>`) {
		t.Fatal("square root not written as a real OMML radical")
	}
	previous := string(s.pkg.document)
	_, err = s.Apply(ctx, ApplyInput{SessionID: added.ID, BaseRevision: added.Revision, Operations: []Operation{
		{Type: "update_math", TargetID: run.ID, Math: &MathEdit{Action: "replace_root", Expression: "a & b"}},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	view := mustSnapshot(t, reopened)
	if view.Blocks[0].Paragraph.Runs[1].Object.SquareRoot != "a & b" ||
		string(reopened.pkg.document) != strings.Replace(previous, "x + 2", "a &amp; b", 1) {
		t.Fatal("editing root changed markup outside its radicand")
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original document")
	}
}
