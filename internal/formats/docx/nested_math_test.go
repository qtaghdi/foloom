package docx

import (
	"bytes"
	"context"
	"testing"
)

func TestNestedMathCreateAndEditLeaf(t *testing.T) {
	for _, kind := range []string{"root_fraction", "fraction_root_numerator", "fraction_root_denominator"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			s, file := withDocument(t, bodyXML(`<w:p><w:r><w:t>Prefix</w:t></w:r></w:p>`))
			v := mustSnapshot(t, s)
			original := fileBytes(t, file)
			apply := func(view Snapshot, id string, math MathEdit) (Snapshot, error) {
				return s.Apply(ctx, ApplyInput{SessionID: view.ID, BaseRevision: view.Revision, Operations: []Operation{{Type: "update_math", TargetID: id, Math: &math}}}, "human")
			}
			v, err := apply(v, v.Blocks[0].ID, MathEdit{Action: "insert_nested", Nested: &MathNested{Kind: kind, Numerator: "Alpha", Denominator: "Beta"}})
			if err != nil {
				t.Fatal(err)
			}
			run := v.Blocks[0].Paragraph.Runs[1]
			if run.Object == nil || run.Object.NestedMath == nil || run.Object.NestedMath.Kind != kind || !run.Object.Editable {
				t.Fatal("missing nested math preview")
			}
			saved := fileBytes(t, file)
			raw := bytes.Clone(s.pkg.document)
			for _, bad := range []MathNestedLeafEdit{{Leaf: "denominator", ExpectedText: "stale", Text: "new"}, {Leaf: "other", ExpectedText: "Beta", Text: "new"}} {
				if _, err := apply(v, run.ID, MathEdit{Action: "edit_nested_leaf", NestedLeaf: &bad}); err == nil || !bytes.Equal(saved, fileBytes(t, file)) {
					t.Fatal("invalid leaf mutated source")
				}
			}
			_, err = apply(v, run.ID, MathEdit{Action: "edit_nested_leaf", NestedLeaf: &MathNestedLeafEdit{Leaf: "denominator", ExpectedText: "Beta", Text: "C < D"}})
			if err != nil {
				t.Fatal(err)
			}
			want := bytes.Replace(raw, []byte(`>Beta</dwm:t>`), []byte(`>C &lt; D</dwm:t>`), 1)
			if !bytes.Equal(want, s.pkg.document) {
				t.Fatal("other OMML changed")
			}
			reopened, err := Open(ctx, file)
			if err != nil {
				t.Fatal(err)
			}
			got := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[1].Object.NestedMath
			if got == nil || got.Numerator != "Alpha" || got.Denominator != "C < D" || got.Kind != kind {
				t.Fatal("nested leaf did not survive reopen")
			}
			historyRestore(t, reopened, "undo")
			if !bytes.Equal(saved, fileBytes(t, file)) {
				t.Fatal("leaf undo failed")
			}
			historyRestore(t, reopened, "undo")
			if !bytes.Equal(original, fileBytes(t, file)) {
				t.Fatal("insertion undo failed")
			}
		})
	}
}
