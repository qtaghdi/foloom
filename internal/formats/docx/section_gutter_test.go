package docx

import (
	"bytes"
	"context"
	"testing"
)

func TestSectionGutterChangesOnlyTargetAndPreservesNativeMargins(t *testing.T) {
	ctx := context.Background()
	s, name := withDocument(t, bodyXML(`<w:p><w:pPr><w:sectPr><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="600" w:gutter="40"/></w:sectPr></w:pPr><w:r><w:t>First</w:t></w:r></w:p><w:p><w:r><w:t>Second</w:t></w:r></w:p><w:sectPr><w:pgMar xmlns:c="urn:custom" c:keep="yes" w:left="1440" w:right="1440" w:top="1440" w:bottom="1440" w:header="720" w:footer="600" w:gutter="0"/><w:cols w:num="3" w:space="3600"/></w:sectPr>`))
	original := fileBytes(t, name)
	view := mustSnapshot(t, s)
	input := func(gutter int) ApplyInput {
		return ApplyInput{SessionID: view.ID, BaseRevision: view.Revision, Operations: []Operation{{
			Type: "update_section", SectionID: view.Sections[1].ID, Section: &SectionFormat{Gutter: &gutter},
		}}}
	}
	for _, invalid := range []int{-1, 31681, 1800} {
		if _, err := s.Apply(ctx, input(invalid), "human"); err == nil || !bytes.Equal(original, fileBytes(t, name)) {
			t.Fatalf("invalid binding gutter %d changed the file", invalid)
		}
	}
	if _, err := s.Apply(ctx, input(360), "human"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	next := mustSnapshot(t, reopened)
	first, changed := next.Sections[0], next.Sections[1]
	if first.Gutter == nil || *first.Gutter != 40 || changed.Gutter == nil || *changed.Gutter != 360 ||
		changed.Page.Left != 1440 || changed.Columns != 3 || changed.ColumnSpace != 3600 || changed.FooterDistance == nil || *changed.FooterDistance != 600 {
		t.Fatalf("gutter changed unrelated settings: %+v", next.Sections)
	}
	if !bytes.Contains(reopened.pkg.document, []byte(`c:keep="yes"`)) {
		t.Fatal("custom margin attribute was lost")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, name)) {
		t.Fatal("undo did not restore the exact original package")
	}
}

func TestSectionGutterProtectsBindingAndPreservesSettingsPart(t *testing.T) {
	ctx := context.Background()
	for _, scenario := range []struct{ name, section, settings string }{
		{name: "mirrored", settings: `<w:mirrorMargins/>`},
		{name: "right", section: `<w:rtlGutter/>`},
		{name: "uncertain top binding", settings: `<w:gutterAtTop w:val="invalid"/>`},
		{name: "book fold", settings: `<w:bookFoldPrinting/>`},
		{name: "uncertain binding flag", settings: `<w:printTwoOnOne w:val="invalid"/>`},
		{name: "unequal", section: `<w:cols w:equalWidth="0"><w:col w:w="3600"/></w:cols>`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			_, name := withDocument(t, bodyXML(`<w:p><w:r><w:t>Body</w:t></w:r></w:p><w:sectPr><w:pgMar w:gutter="300"/>`+scenario.section+`</w:sectPr>`))
			if scenario.settings != "" {
				content := []byte(`<w:settings xmlns:w="` + wordNS + `">` + scenario.settings + `</w:settings>`)
				rels := []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="settings" Type="` + officeRelNS + `/settings" Target="binding-settings.xml"/></Relationships>`)
				rewritePackage(t, name, func(_ string, b []byte) []byte { return b }, map[string][]byte{"word/binding-settings.xml": content, "word/_rels/document.xml.rels": rels})
			}
			s, err := Open(ctx, name)
			if err != nil {
				t.Fatal(err)
			}
			view := mustSnapshot(t, s)
			if view.Sections[0].Gutter != nil {
				t.Fatal("protected binding was exposed for editing")
			}
			before := fileBytes(t, name)
			gutter := 360
			_, err = s.Apply(ctx, ApplyInput{SessionID: view.ID, BaseRevision: view.Revision, Operations: []Operation{{
				Type: "update_section", SectionID: view.Sections[0].ID, Section: &SectionFormat{Gutter: &gutter},
			}}}, "human")
			if err == nil || !bytes.Equal(before, fileBytes(t, name)) {
				t.Fatal("protected gutter update changed the original package")
			}
		})
	}
}

func TestTopBindingGutterUsesBodyHeightAndPreservesDirection(t *testing.T) {
	ctx := context.Background()
	_, name := withDocument(t, bodyXML(`<w:p><w:pPr><w:sectPr><w:pgMar w:gutter="40"/></w:sectPr></w:pPr><w:r><w:t>First</w:t></w:r></w:p><w:p><w:r><w:t>Second</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:bottom="1440" w:gutter="0"/></w:sectPr>`))
	settings := []byte(`<w:settings xmlns:w="` + wordNS + `"><w:gutterAtTop/><!-- preserve --></w:settings>`)
	rels := []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="settings" Type="` + officeRelNS + `/settings" Target="binding.xml"/></Relationships>`)
	rewritePackage(t, name, func(_ string, b []byte) []byte { return b }, map[string][]byte{"word/binding.xml": settings, documentRelationships: rels})
	s, err := Open(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	original := fileBytes(t, name)
	view := mustSnapshot(t, s)
	last := view.Sections[1]
	if !last.GutterAtTop || last.Gutter == nil || *last.Gutter != 0 {
		t.Fatalf("top binding unavailable: %+v", last)
	}
	input := func(gutter int) ApplyInput {
		return ApplyInput{SessionID: view.ID, BaseRevision: view.Revision, Operations: []Operation{{
			Type: "update_section", SectionID: last.ID, Section: &SectionFormat{Gutter: &gutter},
		}}}
	}
	tooTall := input(12800)
	if _, err := s.Apply(ctx, tooTall, "human"); err == nil || !bytes.Equal(original, fileBytes(t, name)) {
		t.Fatal("top binding exhausted body height")
	}
	after, err := s.Apply(ctx, input(360), "human")
	if err != nil || *after.Sections[0].Gutter != 40 || *after.Sections[1].Gutter != 360 || after.Sections[1].Page != last.Page ||
		!after.Sections[1].GutterAtTop || !bytes.Equal(settings, s.pkg.partData("word/binding.xml", nil)) {
		t.Fatalf("setting changed other section, page margins or binding direction: %v", err)
	}
	reopened, err := Open(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	after = mustSnapshot(t, reopened)
	tinyHeight := 3000
	if _, err := reopened.Apply(ctx, ApplyInput{SessionID: after.ID, BaseRevision: after.Revision, Operations: []Operation{{
		Type: "update_section", SectionID: after.Sections[1].ID, Section: &SectionFormat{Height: &tinyHeight},
	}}}, "human"); err == nil {
		t.Fatal("height edit ignored existing top binding")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, name)) {
		t.Fatal("undo did not restore exact package")
	}
}
