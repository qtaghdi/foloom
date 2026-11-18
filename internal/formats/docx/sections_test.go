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

func TestHeaderFooterDistancesPreserveSectionContentsAndUndo(t *testing.T) {
	ctx := context.Background()
	s, path := withDocument(t, bodyXML(`<w:p><w:pPr><w:sectPr><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="540" w:gutter="0"/></w:sectPr></w:pPr><w:r><w:t>first</w:t></w:r></w:p><w:p><w:r><w:t>second</w:t></w:r></w:p><w:sectPr xmlns:c="urn:custom"><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="1080" w:footer="720" w:gutter="144" c:keep="yes"/></w:sectPr>`))
	v := mustSnapshot(t, s)
	header, footer := "Shared header", "Shared footer"
	v, err := s.Apply(ctx, headerBatch(v, 0, HeaderFooterUpdate{Kind: "header", Variant: "default", Text: &header}, HeaderFooterUpdate{Kind: "footer", Variant: "default", Text: &footer}), "human")
	if err != nil {
		t.Fatal(err)
	}
	preserved := map[string][]byte{}
	for _, part := range s.pkg.parts {
		if part.header.Name != "word/document.xml" {
			preserved[part.header.Name] = append([]byte(nil), part.data...)
		}
	}
	baseline := fileBytes(t, path)
	if v.Sections[1].HeaderDistance == nil || *v.Sections[1].HeaderDistance != 1080 {
		t.Fatal("missing imported distance")
	}
	apply := func(header, footer *int) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{
			Type: "update_section", SectionID: v.Sections[1].ID, Section: &SectionFormat{HeaderDistance: header, FooterDistance: footer},
		}}}, "human")
	}
	zero, tooFar, negative := 0, 31681, -1
	if _, err := apply(&tooFar, &zero); err == nil {
		t.Fatal("out-of-range header distance accepted")
	}
	if _, err := apply(&zero, &negative); err == nil || !bytes.Equal(baseline, fileBytes(t, path)) {
		t.Fatal("invalid distance changed the source")
	}
	if _, err := apply(&zero, nil); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	next := mustSnapshot(t, reopened)
	first, second := next.Sections[0], next.Sections[1]
	if first.HeaderDistance == nil || *first.HeaderDistance != 720 || first.FooterDistance == nil || *first.FooterDistance != 540 ||
		second.HeaderDistance == nil || *second.HeaderDistance != 0 || second.FooterDistance == nil || *second.FooterDistance != 720 {
		t.Fatalf("distance changes crossed section boundaries or lost explicit zero: %+v", next.Sections)
	}
	for _, kind := range []string{"header", "footer"} {
		slot := headerSlot(t, next, 1, kind, "default")
		if !slot.Linked || slot.SourceSectionID != first.ID {
			t.Fatalf("%s inheritance changed after position edit: %+v", kind, slot)
		}
	}
	for _, raw := range []string{`w:gutter="144"`, `c:keep="yes"`, `w:top="1440"`} {
		if !strings.Contains(string(reopened.pkg.document), raw) {
			t.Fatalf("lost %s", raw)
		}
	}
	for name, data := range preserved {
		if !bytes.Equal(data, reopened.pkg.partData(name, nil)) {
			t.Fatalf("distance edit changed %s", name)
		}
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(baseline, fileBytes(t, path)) {
		t.Fatal("undo did not restore the exact package")
	}
}

func TestHeaderDistanceCreatesCompletePageMarginWhenMissing(t *testing.T) {
	s, _ := withDocument(t, bodyXML(`<w:p><w:r><w:t>body</w:t></w:r></w:p>`))
	v := mustSnapshot(t, s)
	if v.Sections[0].HeaderDistance != nil || v.Sections[0].FooterDistance != nil {
		t.Fatal("missing distances projected as explicit")
	}
	header := 360
	_, err := s.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{
		Type: "update_section", SectionID: v.Sections[0].ID, Section: &SectionFormat{HeaderDistance: &header},
	}}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseXML(s.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	margins := root.child("body").child("sectPr").child("pgMar")
	for key, value := range map[string]string{"top": "1440", "right": "1440", "bottom": "1440", "left": "1440", "header": "360", "footer": "720", "gutter": "0"} {
		if got := margins.attr(key); got != value {
			t.Fatalf("margin %s = %q; want %q", key, got, value)
		}
	}
}
