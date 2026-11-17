package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNativeMathEditsPreserveComplexObjectXML(t *testing.T) {
	ctx := context.Background()
	fraction := `<m:oMath xmlns:m="` + mathNS + `"><m:f><m:num><m:r><m:t>1</m:t></m:r></m:num><m:den><m:r><m:t>2</m:t></m:r></m:den></m:f></m:oMath>`
	chart := `<w:r><w:drawing><wp:inline xmlns:wp="` + drawingWordNS + `"><a:graphic xmlns:a="` + drawingNS + `"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"/></a:graphic></wp:inline></w:drawing></w:r>`
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>plain</w:t></w:r></w:p><w:p>`+fraction+chart+`</w:p>`))
	before := fileBytes(t, filename)
	apply := func(v Snapshot, id string, edit MathEdit) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "update_math", TargetID: id, Math: &edit}}}, "human")
	}
	v := mustSnapshot(t, s)
	imported := v.Blocks[1].Paragraph.Runs
	if imported[0].Object == nil || imported[0].Object.Editable || imported[1].Object == nil || imported[1].Object.Kind != "chart" {
		t.Fatal("complex objects must be classified and protected")
	}
	if _, err := apply(v, imported[0].ID, MathEdit{Action: "delete"}); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("complex equation was removed")
	}
	next, err := apply(v, v.Blocks[0].ID, MathEdit{Action: "insert", Expression: "x² + y²"})
	if err != nil {
		t.Fatal(err)
	}
	math := next.Blocks[0].Paragraph.Runs[1]
	if math.Object == nil || !math.Object.Editable || math.Object.Text != "x² + y²" {
		t.Fatal("equation preview unavailable")
	}
	next, err = apply(next, math.ID, MathEdit{Action: "replace", Expression: "x² < y²"})
	if err != nil || !strings.Contains(string(s.pkg.document), "x² &lt; y²") {
		t.Fatal("replacement not saved", err)
	}
	math = next.Blocks[0].Paragraph.Runs[1]
	next, err = apply(next, math.ID, MathEdit{Action: "delete"})
	if err != nil || len(next.Blocks[0].Paragraph.Runs) != 1 || !strings.Contains(string(s.pkg.document), fraction+chart) {
		t.Fatal("equation removal altered other objects", err)
	}
	if _, err := apply(next, next.Blocks[0].ID, MathEdit{Action: "insert_fraction", Fraction: &MathFraction{Numerator: "x"}}); err == nil {
		t.Fatal("missing denominator was accepted")
	}
	next, err = apply(next, next.Blocks[0].ID, MathEdit{Action: "insert_fraction", Fraction: &MathFraction{Numerator: "x < y", Denominator: "2"}})
	if err != nil || !strings.Contains(string(s.pkg.document), `<dwm:num><dwm:r><dwm:t xml:space="preserve">x &lt; y</dwm:t></dwm:r></dwm:num>`) ||
		!strings.Contains(string(s.pkg.document), fraction+chart) {
		t.Fatalf("fraction did not retain native math and neighboring objects: %v", err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	object := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[1].Object
	if object == nil || object.Fraction == nil || object.Fraction.Numerator != "x < y" || object.Fraction.Denominator != "2" || object.Editable {
		t.Fatalf("fraction preview unavailable after reload: %+v", object)
	}
	for range 4 {
		historyRestore(t, reopened, "undo")
	}
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo changed original bytes")
	}
}
