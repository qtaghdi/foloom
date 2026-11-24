package docx

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestInsertInlineLinePreservesExistingDrawingsAndPackage(t *testing.T) {
	ctx := context.Background()
	first := `<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>Line anchor</w:t></w:r>`
	second := `<w:p>`
	chart := "<w:p>" + chartDeleteDrawing + "</w:p>"
	body := first + "</w:p>" + second + "</w:p>" + chart
	s, filename := chartDocument(t, body, chartTitlePart, embeddedChartData)
	original, baseline := fileBytes(t, filename), s.pkg
	v := mustSnapshot(t, s)
	size := LineSizeEdit{WidthEMU: 1524000, HeightEMU: 457200}
	secondSize := LineSizeEdit{WidthEMU: 12700, HeightEMU: 18288000}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "insert_line", TargetID: v.Blocks[0].ID, LineSize: &size},
		{Type: "insert_line", TargetID: v.Blocks[1].ID, LineSize: &secondSize},
	}}
	result, err := s.Apply(ctx, input, "human")
	if err != nil {
		t.Fatal(err)
	}
	if result.Revision != v.Revision+1 || result.Sequence != v.Sequence+1 || s.events[0].TextOnly {
		t.Fatal("line insertion was not a single structural revision")
	}
	if _, err := s.Apply(ctx, input, "ai"); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale line insertion returned %v", err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	for i, want := range []LineSizeEdit{size, secondSize} {
		runs := v.Blocks[i].Paragraph.Runs
		line := runs[len(runs)-1].Object
		if line == nil || line.Kind != "shape" || !line.Editable || line.Rectangle != nil || line.Line == nil ||
			*line.Line != (LinePreview{LineSizeEdit: want, Stroke: "0F766E"}) {
			t.Fatalf("unexpected reopened line: %+v", line)
		}
	}
	raw := string(reopened.pkg.document)
	if raw != bodyXML(first+inlineLine(16, size)+"</w:p>"+second+inlineLine(17, secondSize)+"</w:p>"+chart) ||
		strings.Count(raw, `<a:prstGeom prst="line">`) != 2 {
		t.Fatal("line insertion changed unrelated XML or used unsupported markup")
	}
	if len(baseline.parts) != len(reopened.pkg.parts) {
		t.Fatal("line insertion changed package contents")
	}
	for _, part := range baseline.parts {
		if part.header.Name != "word/document.xml" && !bytes.Equal(reopened.pkg.partData(part.header.Name, nil), part.data) {
			t.Fatalf("line insertion changed %s", part.header.Name)
		}
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original package bytes")
	}
}

func TestInsertInlineLineRejectsInvalidDimensionsAndTargets(t *testing.T) {
	body := `<w:p><w:r><w:t>Editable</w:t></w:r></w:p><w:p><w:ins w:id="3"><w:r><w:t>Tracked</w:t></w:r></w:ins></w:p>`
	s, filename := withDocument(t, bodyXML(body))
	original := fileBytes(t, filename)
	initial := mustSnapshot(t, s)
	valid := LineSizeEdit{WidthEMU: 12700, HeightEMU: 18288000}
	for _, bad := range []Operation{
		{Type: "insert_line", TargetID: initial.Blocks[0].ID},
		{Type: "insert_line", TargetID: initial.Blocks[0].ID, LineSize: &LineSizeEdit{WidthEMU: 12699, HeightEMU: 12700}},
		{Type: "insert_line", TargetID: initial.Blocks[0].ID, LineSize: &LineSizeEdit{WidthEMU: 12700, HeightEMU: 18288001}},
		{Type: "insert_line", TargetID: initial.Blocks[0].ID, LineSize: &valid, Text: "Unexpected"},
		{Type: "insert_line", TargetID: initial.Blocks[1].ID, LineSize: &valid},
		{Type: "insert_rectangle", TargetID: initial.Blocks[0].ID, LineSize: &valid},
	} {
		_, err := s.Apply(context.Background(), ApplyInput{
			SessionID: initial.ID, BaseRevision: initial.Revision, Operations: []Operation{bad},
		}, "ai")
		if err == nil || !bytes.Equal(original, fileBytes(t, filename)) || mustSnapshot(t, s).Revision != initial.Revision {
			t.Fatalf("invalid line edit mutated the package or revision: %+v", bad)
		}
	}
}

func TestLinePreviewKeepsUnmodeledShapesOpaque(t *testing.T) {
	line := inlineLine(1, LineSizeEdit{WidthEMU: 914400, HeightEMU: 457200})
	for _, edit := range [][2]string{
		{`<a:xfrm>`, `<a:xfrm flipV="1">`},
		{`<a:ln w="12700">`, `<a:ln w="12700" cap="rnd">`},
		{`prst="line"`, `prst="lineInv"`},
		{`<a:ext cx="914400"`, `<a:ext cx="1016000"`},
	} {
		s, _ := withDocument(t, bodyXML("<w:p>"+strings.Replace(line, edit[0], edit[1], 1)+"</w:p>"))
		object := mustSnapshot(t, s).Blocks[0].Paragraph.Runs[0].Object
		if object == nil || object.Kind != "shape" || object.Line != nil {
			t.Fatalf("unsupported outline was projected as a basic line: %+v", object)
		}
	}
}

func TestResizeInlineLineChangesOnlyExtents(t *testing.T) {
	ctx := context.Background()
	old := LineSizeEdit{WidthEMU: 914400, HeightEMU: 457200}
	next := LineSizeEdit{WidthEMU: 1270000, HeightEMU: 381000}
	prefix := `<w:r><w:t>Prefix</w:t></w:r>`
	neighbor := inlineRectangle(2)
	s, filename := withDocument(t, bodyXML("<w:p>"+prefix+inlineLine(1, old)+neighbor+"</w:p>"))
	v := mustSnapshot(t, s)
	original := fileBytes(t, filename)
	line := v.Blocks[0].Paragraph.Runs[1]
	if line.Object == nil || !line.Object.Editable {
		t.Fatal("line offered no resize capability")
	}
	op := Operation{Type: "resize_line", TargetID: line.ID, LineSize: &next}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}
	for _, bad := range []Operation{{Type: "resize_line", TargetID: line.ID, LineSize: &old}, {Type: "resize_line", TargetID: v.Blocks[0].Paragraph.Runs[2].ID, LineSize: &next}, {Type: "resize_line", TargetID: line.ID, LineSize: &LineSizeEdit{WidthEMU: 0, HeightEMU: 12700}}} {
		invalid := input
		invalid.Operations = []Operation{bad}
		if _, err := s.Apply(ctx, invalid, "human"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
			t.Fatal("invalid line resize changed source")
		}
	}
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	extent := regexp.MustCompile(`<(?:wp:extent|a:ext)\s[^>]*>(?:</(?:wp:extent|a:ext)>)?`)
	if extent.ReplaceAllString(string(s.pkg.document), "extent") != extent.ReplaceAllString(bodyXML("<w:p>"+prefix+inlineLine(1, next)+neighbor+"</w:p>"), "extent") || !bytes.Contains(s.pkg.document, []byte(neighbor)) {
		t.Fatal("modified unrelated XML")
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	got := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[1].Object
	if got == nil || got.Line == nil || got.Line.LineSizeEdit != next {
		t.Fatal("line size did not survive reopen")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore original bytes")
	}
}

func TestDeleteInlineLinePreservesOtherRunsAndProtectedRanges(t *testing.T) {
	ctx := context.Background()
	line := inlineLine(1, LineSizeEdit{WidthEMU: 914400, HeightEMU: 457200})
	prefix := `<w:r><w:t>Prefix</w:t></w:r>`
	neighbor := inlineRectangle(2)
	protected := strings.Replace(inlineLine(3, LineSizeEdit{WidthEMU: 914400, HeightEMU: 457200}), `name="Line 3"`, `name="Different representation"`, 1)
	s, filename := withDocument(t, bodyXML("<w:p>"+prefix+line+neighbor+"</w:p><w:p>"+protected+"</w:p>"))
	v := mustSnapshot(t, s)
	original := fileBytes(t, filename)
	run := v.Blocks[0].Paragraph.Runs[1]
	if run.Object == nil || !run.Object.DeleteEditable || v.Blocks[1].Paragraph.Runs[0].Object.DeleteEditable {
		t.Fatal("unexpected deletion capability")
	}
	op := Operation{Type: "delete_line", TargetID: run.ID}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}
	for _, bad := range []Operation{{Type: "delete_line", TargetID: v.Blocks[0].Paragraph.Runs[2].ID}, {Type: "delete_line", TargetID: v.Blocks[1].Paragraph.Runs[0].ID}, {Type: "delete_line", TargetID: run.ID, Text: "Unexpected"}} {
		invalid := input
		invalid.Operations = []Operation{bad}
		if _, err := s.Apply(ctx, invalid, "human"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
			t.Fatal("invalid deletion changed source")
		}
	}
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	if string(s.pkg.document) != bodyXML("<w:p>"+prefix+neighbor+"</w:p><w:p>"+protected+"</w:p>") {
		t.Fatal("modified unrelated XML")
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	if len(mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs) != 2 {
		t.Fatal("deleted line survived reopening")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore original bytes")
	}
}

func TestDeleteInlineLineRejectsActiveBookmark(t *testing.T) {
	line := inlineLine(1, LineSizeEdit{WidthEMU: 914400, HeightEMU: 457200})
	s, filename := withDocument(t, bodyXML(`<w:p><w:bookmarkStart w:id="1" w:name="line_range"/></w:p><w:p>`+line+`</w:p><w:p><w:bookmarkEnd w:id="1"/></w:p>`))
	v := mustSnapshot(t, s)
	original := fileBytes(t, filename)
	run := v.Blocks[1].Paragraph.Runs[0]
	if run.Object == nil || run.Object.DeleteEditable {
		t.Fatal("protected referenced line advertised deletion")
	}
	_, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "delete_line", TargetID: run.ID}}}, "human")
	if err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("reference range was modified")
	}
}
