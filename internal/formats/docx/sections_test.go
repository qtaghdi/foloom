package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSectionSettingsPreserveBoundariesAndOpaqueProperties(t *testing.T) {
	s, path := withDocument(t, bodyXML(`<w:p><w:pPr><w:sectPr xmlns:c="urn:custom"><w:headerReference xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" w:type="default" r:id="rIdKeep"/><w:pgSz w:w="11906" w:h="16838" c:keep="yes"/><w:pgMar w:top="1440" w:bottom="1440" w:left="1440" w:right="1440" w:header="720"/><w:docGrid w:linePitch="360"/></w:sectPr></w:pPr><w:r><w:t>first</w:t></w:r></w:p><w:p><w:r><w:t>second</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr>`))
	original := fileBytes(t, path)
	v := mustSnapshot(t, s)
	if len(v.Sections) != 2 || v.Sections[0].StartBlock != 0 || v.Sections[0].EndBlock != 0 || v.Sections[1].StartBlock != 1 || v.Sections[1].EndBlock != 1 {
		t.Fatalf("sections %+v", v.Sections)
	}
	width, height, left, columns, space := 16838, 11906, 720, 2, 360
	next, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "update_section", SectionID: v.Sections[0].ID, Section: &SectionFormat{Width: &width, Height: &height, Orientation: "landscape", Left: &left, Columns: &columns, ColumnSpace: &space, BreakType: "continuous"}}}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	if next.Revision != v.Revision+1 || next.Sections[0].Page.Width != width || next.Sections[1].Page.Width != 11906 || next.Sections[0].Columns != 2 || next.Sections[0].BreakType != "continuous" {
		t.Fatalf("mixed settings %+v", next.Sections)
	}
	for _, raw := range []string{`c:keep="yes"`, `w:header="720"`, `r:id="rIdKeep"`, `<w:docGrid w:linePitch="360"/>`, `<w:t>first</w:t>`, `<w:t>second</w:t>`} {
		if !strings.Contains(string(s.pkg.document), raw) {
			t.Fatalf("lost %s", raw)
		}
	}
	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if mustSnapshot(t, reopened).Sections[0].Orientation != "landscape" {
		t.Fatal("settings lost on reopen")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("undo changed original bytes")
	}
}

func TestSectionUpdateRejectsInvalidAndOverlappingBatches(t *testing.T) {
	for _, kind := range []string{"dimension", "margins", "columns", "spacing", "orientation", "break", "stale", "duplicate", "overlap", "protected"} {
		t.Run(kind, func(t *testing.T) {
			xml := `<w:p><w:r><w:t>x</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr>`
			if kind == "protected" {
				xml = strings.Replace(xml, `</w:sectPr>`, `<w:sectPrChange/></w:sectPr>`, 1)
			}
			s, path := withDocument(t, bodyXML(xml))
			v := mustSnapshot(t, s)
			before := fileBytes(t, path)
			width, bad := 12000, -1
			op := Operation{Type: "update_section", SectionID: v.Sections[0].ID, Section: &SectionFormat{Width: &width}}
			switch kind {
			case "dimension":
				op.Section.Width = &bad
			case "margins":
				op.Section.Left = &width
			case "columns":
				op.Section.Columns = &bad
			case "spacing":
				op.Section.ColumnSpace = &width
				n := 3
				op.Section.Columns = &n
			case "orientation":
				op.Section.Orientation = "bad"
			case "break":
				op.Section.BreakType = "bad"
			case "stale":
				op.SectionID = "old"
			}
			ops := []Operation{op}
			if kind == "duplicate" {
				ops = append(ops, op)
			}
			if kind == "overlap" {
				ops[0].Text = "unexpected"
			}
			_, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: ops}, "human")
			if err == nil || !bytes.Equal(before, fileBytes(t, path)) || mustSnapshot(t, s).Revision != v.Revision {
				t.Fatalf("invalid section mutated: %s", kind)
			}
		})
	}
}

func TestSectionCanBeAddedToDocumentWithoutExplicitSettings(t *testing.T) {
	s, _ := withDocument(t, bodyXML(`<w:p><w:r><w:t>x</w:t></w:r></w:p>`))
	v := mustSnapshot(t, s)
	n := 2
	next, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "update_section", SectionID: v.Sections[0].ID, Section: &SectionFormat{Columns: &n}}}}, "human")
	if err != nil || next.Sections[0].Columns != 2 {
		t.Fatalf("create section: %+v, %v", next, err)
	}
}

func TestPageNumberingContinuesWithoutLosingChapterProperties(t *testing.T) {
	s, path := withDocument(t, bodyXML(`<w:p><w:pPr><w:sectPr><w:pgNumType w:start="4" w:fmt="upperRoman"/></w:sectPr></w:pPr></w:p><w:p/><w:sectPr xmlns:c="urn:custom"><w:pgNumType w:start="0" w:fmt="lowerRoman" w:chapStyle="1" w:chapSep="hyphen" c:start="keep"/><w:cols w:num="1"/></w:sectPr>`))
	original := fileBytes(t, path)
	apply := func(start int, format string) (Snapshot, error) {
		v := mustSnapshot(t, s)
		return s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "update_section", SectionID: v.Sections[1].ID, Section: &SectionFormat{PageNumberStart: &start, PageNumberFormat: format}}}}, "human")
	}
	initial := mustSnapshot(t, s)
	if n := initial.Sections[1].PageNumberStart; n == nil || *n != 0 {
		t.Fatal("explicit zero is different from continuation")
	}
	for _, invalid := range []struct {
		start  int
		format string
	}{{-2, "decimal"}, {1, "unsupported"}} {
		if _, err := apply(invalid.start, invalid.format); err == nil || !bytes.Equal(original, fileBytes(t, path)) || mustSnapshot(t, s).Revision != initial.Revision {
			t.Fatal("invalid page numbering was persisted")
		}
	}
	next, err := apply(-1, "upperLetter")
	if err != nil || next.Sections[1].PageNumberStart != nil || next.Sections[1].PageNumberFormat != "upperLetter" || next.Sections[0].PageNumberFormat != "upperRoman" {
		t.Fatalf("continue numbering: %+v, %v", next.Sections, err)
	}
	for _, raw := range []string{`w:start="4"`, `c:start="keep"`, `w:chapStyle="1"`, `w:chapSep="hyphen"`, `<w:cols w:num="1"/>`} {
		if !strings.Contains(string(s.pkg.document), raw) {
			t.Fatalf("lost %s", raw)
		}
	}
	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if n := mustSnapshot(t, reopened).Sections[1]; n.PageNumberStart != nil || n.PageNumberFormat != "upperLetter" {
		t.Fatalf("reopened numbering: %+v", n)
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("undo changed original bytes")
	}
}
