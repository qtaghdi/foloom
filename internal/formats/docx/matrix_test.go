package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestPlainMatrixCellsEditOnlySelectedText(t *testing.T) {
	ctx := context.Background()
	neighbor := `<m:oMath xmlns:m="` + mathNS + `"><m:m><m:mr><m:e><m:r><m:t>protected</m:t></m:r></m:e></m:mr></m:m></m:oMath>`
	s, file := withDocument(t, bodyXML(`<w:p><w:r><w:t>Matrix prefix</w:t></w:r></w:p><w:p>`+neighbor+`</w:p>`))
	save := func(snapshot Snapshot, operations ...Operation) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision, Operations: operations}, "human")
	}
	op := func(id string, math MathEdit) Operation {
		return Operation{Type: "update_math", TargetID: id, Math: &math}
	}
	v := mustSnapshot(t, s)
	original := fileBytes(t, file)
	if v.Blocks[1].Paragraph.Runs[0].Object.Editable {
		t.Fatal("non-2×2 matrix was offered for editing")
	}
	for _, values := range [][][]string{{{"1", "2", "extra"}, {"3", "4"}}, {{"", "2"}, {"3", "4"}},
		{{"1", "2"}, {"3", strings.Repeat("한", 86)}}, {{"1", "2"}, {"3"}}, nil} {
		if _, err := save(v, op(v.Blocks[0].ID, MathEdit{Action: "insert_matrix", Matrix: values})); err == nil ||
			!bytes.Equal(original, fileBytes(t, file)) {
			t.Fatal("invalid matrix changed the document")
		}
	}
	v, err := save(v, op(v.Blocks[0].ID, MathEdit{Action: "insert_matrix", Matrix: [][]string{{"11", "22"}, {"33", "44"}}}))
	if err != nil {
		t.Fatal(err)
	}
	initial := fileBytes(t, file)
	xml := bytes.Clone(s.pkg.document)
	run := v.Blocks[0].Paragraph.Runs[1]
	want := [2][2]string{{"11", "22"}, {"33", "44"}}
	if run.Object == nil || !run.Object.Editable || run.Object.Matrix == nil || *run.Object.Matrix != want ||
		!bytes.Contains(xml, []byte(neighbor)) {
		t.Fatal("missing native matrix preview or preserved complex object", run)
	}
	valid := op(run.ID, MathEdit{Action: "edit_matrix_cell", MatrixCell: &MathMatrixCellEdit{
		Row: 2, Column: 1, ExpectedText: "33", Text: "A < B",
	}})
	for _, invalid := range []Operation{
		op(run.ID, MathEdit{Action: "delete"}),
		op(run.ID, MathEdit{Action: "edit_matrix_cell", MatrixCell: &MathMatrixCellEdit{Row: 0, Column: 1, ExpectedText: "33", Text: "5"}}),
		op(run.ID, MathEdit{Action: "edit_matrix_cell", MatrixCell: &MathMatrixCellEdit{Row: 2, Column: 1, ExpectedText: "stale", Text: "5"}}),
		op(v.Blocks[1].Paragraph.Runs[0].ID, MathEdit{Action: "edit_matrix_cell", MatrixCell: valid.Math.MatrixCell}),
	} {
		if _, err := save(v, invalid); err == nil || !bytes.Equal(initial, fileBytes(t, file)) {
			t.Fatal("invalid edit changed the package", invalid)
		}
	}
	if _, err := save(v, valid, valid); err == nil || !bytes.Equal(initial, fileBytes(t, file)) {
		t.Fatal("overlapping edits changed the package")
	}
	if _, err := save(v, valid); err != nil {
		t.Fatal(err)
	}
	expected := bytes.Replace(xml, []byte(`>33</dwm:t>`), []byte(`>A &lt; B</dwm:t>`), 1)
	if !bytes.Equal(s.pkg.document, expected) {
		t.Fatal("the edit modified content outside the addressed cell")
	}
	reopened, err := Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	want[1][0] = "A < B"
	got := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[1].Object
	if got == nil || got.Matrix == nil || *got.Matrix != want {
		t.Fatal("matrix cell contents did not survive reopen", got)
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(fileBytes(t, file), initial) {
		t.Fatal("undo failed to restore the package before cell editing")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(fileBytes(t, file), original) {
		t.Fatal("undo failed to restore the package before matrix insertion")
	}
}
