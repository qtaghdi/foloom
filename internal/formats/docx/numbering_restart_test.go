package docx

import (
	"bytes"
	"context"
	"testing"
)

const restartListAbstract = `<q:abstractNum q:abstractNumId="5">
	<q:multiLevelType q:val="multilevel"/>
	<q:lvl q:ilvl="0"><q:start q:val="2"/><q:numFmt q:val="upperRoman"/><q:lvlText q:val="Article %1)"/><q:lvlJc q:val="left"/>
		<q:pPr><q:ind q:left="1000" q:hanging="240"/></q:pPr></q:lvl>
	<q:lvl q:ilvl="1"><q:start q:val="1"/><q:numFmt q:val="decimal"/><q:lvlText q:val="%1.%2"/></q:lvl>
</q:abstractNum>`
const restartListOtherLevel = `<q:lvlOverride q:ilvl="1"><q:startOverride q:val="3"/></q:lvlOverride>`

func TestRestartImportedListPreservesNativeDefinitions(t *testing.T) {
	ctx := context.Background()
	paragraph := func(text string) string {
		return `<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="7"/></w:numPr></w:pPr><w:r><w:t>` + text + `</w:t></w:r></w:p>`
	}
	_, path := withDocument(t, bodyXML(paragraph("first")+paragraph("second")+paragraph("third")+paragraph("fourth")))
	top := `<q:lvlOverride q:ilvl="0"><q:startOverride q:val="4"/><q:lvl q:ilvl="0">
		<q:numFmt q:val="upperRoman"/><q:lvlText q:val="Appendix %1)"/><q:lvlJc q:val="left"/>
		<q:pPr><q:ind q:left="1180" q:hanging="340"/></q:pPr></q:lvl></q:lvlOverride>`
	instance := `<q:num q:numId="7"><q:abstractNumId q:val="5"/>` + top + restartListOtherLevel + `</q:num>`
	numbering := []byte(`<q:numbering xmlns:q="` + wordNS + `">` + restartListAbstract + instance + `</q:numbering>`)
	rels := []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="numbering" Type="` + numberingRelationship + `" Target="custom-lists.xml"/></Relationships>`)
	rewritePackage(t, path, func(_ string, b []byte) []byte { return b }, map[string][]byte{"word/custom-lists.xml": numbering, "word/_rels/document.xml.rels": rels})
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	baseline := fileBytes(t, path)
	v := mustSnapshot(t, s)
	var edits []ParagraphContent
	for _, block := range v.Blocks[1:3] {
		p := block.Paragraph
		edits = append(edits, ParagraphContent{SourceID: p.ID, Format: &ParagraphFormat{List: &ParagraphList{Key: "restart"}},
			Runs: []RunContent{{SourceID: p.Runs[0].ID, Text: p.Runs[0].Text}}})
	}
	rangeSnapshot := v
	rangeSnapshot.Blocks = v.Blocks[1:3]
	apply := func(definition ListDefinition) error {
		input := paragraphPlan(rangeSnapshot, edits...)
		input.Operations = append(input.Operations, Operation{Type: "define_list", ListDefinition: &definition})
		_, err := s.Apply(ctx, input, "human")
		return err
	}
	for _, invalid := range []ListDefinition{
		{Key: "restart", SourceID: 900, Start: 5},
		{Key: "restart", SourceID: 7, Start: 0},
		{Key: "restart", SourceID: 7, Start: 4000},
		{Key: "restart", SourceID: 7, Start: 5, Kind: "decimal"},
	} {
		if err := apply(invalid); err == nil || !bytes.Equal(baseline, fileBytes(t, path)) {
			t.Fatalf("invalid restart changed the package: %+v", invalid)
		}
	}
	if err := apply(ListDefinition{Key: "restart", SourceID: 7, Start: 9}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	next := mustSnapshot(t, reopened)
	for i, want := range []string{"Appendix IV)", "Appendix IX)", "Appendix X)", "Appendix V)"} {
		p := next.Blocks[i].Paragraph
		if p.ListLabel != want || (p.List.ID == 7) != (i == 0 || i == 3) {
			t.Fatalf("paragraph %d has label %q and list ID %d", i, p.ListLabel, p.List.ID)
		}
	}
	if next.Blocks[1].Paragraph.List.ID != next.Blocks[2].Paragraph.List.ID {
		t.Fatal("restarted items have independent counters")
	}
	raw := reopened.pkg.partData(reopened.pkg.numberingPath, nil)
	for _, keep := range []string{restartListAbstract, instance} {
		if !bytes.Contains(raw, []byte(keep)) {
			t.Fatal("original numbering definition or instance changed")
		}
	}
	if bytes.Count(raw, []byte(restartListOtherLevel)) != 2 || bytes.Count(raw, []byte(`q:left="1180"`)) != 2 {
		t.Fatal("restart lost an unrelated override or the custom list indentation")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(baseline, fileBytes(t, path)) {
		t.Fatal("undo did not restore the exact package")
	}
}

func TestRestartListAddsOrExpandsLevelOverride(t *testing.T) {
	for name, top := range map[string]string{"absent": "", "empty": `<q:lvlOverride q:ilvl="0"/>`} {
		t.Run(name, func(t *testing.T) {
			source := []byte(`<q:numbering xmlns:q="` + wordNS + `">` + restartListAbstract +
				`<q:num q:numId="7"><q:abstractNumId q:val="5"/>` + top + restartListOtherLevel + `</q:num></q:numbering>`)
			root, err := parseXML(source)
			if err != nil {
				t.Fatal(err)
			}
			definition, clone, err := copyListWithStart(root, source, ListDefinition{SourceID: 7, Start: 9}, 8, 6)
			if err != nil {
				t.Fatal(err)
			}
			result, err := parseXML([]byte(clone))
			if err != nil {
				t.Fatal(err)
			}
			abstract, err := parseXML([]byte(definition))
			if err != nil || abstract.attr("abstractNumId") != "6" {
				t.Fatalf("missing independent copied abstract definition: %s", definition)
			}
			if result.attr("numId") != "8" || result.child("abstractNumId").attr("val") != "6" ||
				result.child("lvlOverride").child("startOverride").attr("val") != "9" {
				t.Fatalf("unexpected native start override: %s", clone)
			}
		})
	}
}

func TestRestartIndependentSublevelPreservesParentAndCompoundLabels(t *testing.T) {
	ctx := context.Background()
	paragraph := func(level, label string) string {
		return `<w:p><w:pPr><w:numPr><w:ilvl w:val="` + level + `"/><w:numId w:val="7"/></w:numPr></w:pPr><w:r><w:t>` + label + `</w:t></w:r></w:p>`
	}
	_, filename := withDocument(t, bodyXML(paragraph("0", "Parent")+paragraph("1", "First child")+paragraph("1", "Selected")+paragraph("1", "Last child")+paragraph("2", "Compound")))
	numbering := `<w:numbering xmlns:w="` + wordNS + `">
	<w:abstractNum w:abstractNumId="1"><w:multiLevelType w:val="multilevel"/>
		<w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="upperRoman"/><w:lvlText w:val="%1."/></w:lvl>
		<w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="lowerRoman"/><w:lvlText w:val="Item %2)"/><w:pPr><w:ind w:left="1300"/></w:pPr></w:lvl>
		<w:lvl w:ilvl="2"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1.%3."/></w:lvl>
	</w:abstractNum>
	<w:num w:numId="7"><w:abstractNumId w:val="1"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="4"/></w:lvlOverride></w:num>
</w:numbering>`
	rels := []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="num" Type="` + numberingRelationship + `" Target="numbering.xml"/></Relationships>`)
	rewritePackage(t, filename, func(_ string, b []byte) []byte { return b }, map[string][]byte{"word/numbering.xml": []byte(numbering), "word/_rels/document.xml.rels": rels})
	s, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	before := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	item := v.Blocks[2].Paragraph
	selection := v
	selection.Blocks = v.Blocks[2:3]
	apply := func(level int) error {
		input := paragraphPlan(selection, ParagraphContent{SourceID: item.ID,
			Format: &ParagraphFormat{List: &ParagraphList{Key: "restart", Level: level}},
			Runs:   []RunContent{{SourceID: item.Runs[0].ID, Text: item.Runs[0].Text}},
		})
		input.Operations = append(input.Operations, Operation{
			Type: "define_list", ListDefinition: &ListDefinition{SourceID: 7, Level: level, Start: 9, Key: "restart"},
		})
		_, err := s.Apply(ctx, input, "human")
		return err
	}
	for _, unsupported := range []int{-1, 2, 8, 9} {
		if err := apply(unsupported); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
			t.Fatalf("unsafe number level %d changed the document", unsupported)
		}
	}
	if err := apply(1); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	next := mustSnapshot(t, reopened)
	for i, label := range []string{"IV.", "Item i)", "Item ix)", "Item ii)", "IV.1."} {
		p := next.Blocks[i].Paragraph
		if p.ListLabel != label || (p.List.ID == 7) == (i == 2) || i == 2 && p.List.Level != 1 {
			t.Fatalf("paragraph %d has unexpected label or list: %q %+v", i, p.ListLabel, p.List)
		}
	}
	raw := reopened.pkg.partData(reopened.pkg.numberingPath, nil)
	if bytes.Count(raw, []byte(`<w:lvlOverride w:ilvl="0"><w:startOverride w:val="4"/></w:lvlOverride>`)) != 2 ||
		bytes.Count(raw, []byte(`<w:ind w:left="1300"/>`)) != 2 {
		t.Fatal("parent override or sublevel indentation was lost")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the exact package")
	}
}

func TestSublevelRestartRejectsDescendantReferencingItsCounter(t *testing.T) {
	levels := map[int]listLevel{
		1: {kind: "decimal", text: "Item %2)"},
		2: {kind: "decimal", text: "%1.%2.%3."},
	}
	if restartableListLevel(levels, 1, 3) {
		t.Fatal("restarting an ancestor would disconnect the descendant's displayed number")
	}
}
