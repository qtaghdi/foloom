package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func captionFixture(prefix, instruction, number, label string) string {
	return `<w:p><w:r><w:t xml:space="preserve">` + prefix + ` </w:t></w:r><w:fldSimple w:instr="` +
		escapeText(instruction) + `" w:dirty="true"><w:r><w:t>` + number +
		`</w:t></w:r></w:fldSimple><w:r><w:t xml:space="preserve">. ` + escapeText(label) + `</w:t></w:r></w:p>`
}

func TestCaptionListsAndHeadingsCoexistAndPersist(t *testing.T) {
	ctx := context.Background()
	session, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>Anchor</w:t></w:r></w:p>`+
		`<w:p><w:pPr><w:outlineLvl w:val="0"/></w:pPr><w:r><w:t>Overview</w:t></w:r></w:p>`+
		captionFixture("그림", figureSequence, "1", "개요 & 예시")+
		captionFixture("표", "SEQ Table", "1", "요약")+
		captionFixture("그림", figureSequence, "2", "두 번째")))
	before := fileBytes(t, filename)
	v := mustSnapshot(t, session)
	apply := func(action string, block int) (Snapshot, error) {
		return session.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
			{Type: "update_reference", TargetID: v.Blocks[block].ID, Reference: &ReferenceEdit{Action: action}},
		}}, "human")
	}
	for i, insertion := range []string{"insert_figure_list", "insert_table_list", "insert_toc"} {
		next, err := apply(insertion, i)
		if err != nil {
			t.Fatal(err)
		}
		v = next
	}
	saved := fileBytes(t, filename)
	for _, insertion := range []string{"insert_figure_list", "insert_table_list", "insert_toc"} {
		if _, err := apply(insertion, 3); err == nil || !bytes.Equal(saved, fileBytes(t, filename)) {
			t.Fatalf("%s did not reject a duplicate without changing the file", insertion)
		}
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	got := mustSnapshot(t, reopened)
	for i, expected := range []struct{ kind, text, code string }{
		{"figure_list", "그림 1. 개요 & 예시\n그림 2. 두 번째", figureListInstruction},
		{"table_list", "표 1. 요약", tableListInstruction},
		{"toc", "Overview", headingTOCInstruction},
	} {
		run := got.Blocks[i].Paragraph.Runs[0]
		if run.Editable || run.Object == nil || run.Object.Kind != expected.kind || run.Object.Text != expected.text ||
			!strings.Contains(string(reopened.pkg.document), escapeText(expected.code)) {
			t.Fatalf("unexpected %s cache: %+v", expected.kind, run)
		}
	}
	for range 3 {
		historyRestore(t, reopened, "undo")
	}
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original DOCX")
	}
}

func TestInsertCaptionListRejectsUnsupportedCaptionsAndDuplicates(t *testing.T) {
	table := captionFixture("Table", "SEQ Table", "1", "Caption")
	for name, extra := range map[string]string{
		"no captions":          "",
		"incorrect number":     captionFixture("Table", "SEQ Table", "other", "Unsupported"),
		"reset sequence":       captionFixture("Table", `SEQ Table \r 3`, "3", "Unsupported"),
		"nested result":        `<w:p><w:hyperlink>` + table + `</w:hyperlink></w:p>`,
		"complex caption":      `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> SE</w:instrText></w:r><w:r><w:instrText>Q Table </w:instrText></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`,
		"existing native list": table + `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> TOC \c</w:instrText></w:r><w:r><w:instrText>"TABLE"</w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`,
	} {
		t.Run(name, func(t *testing.T) {
			session, filename := withDocument(t, bodyXML(`<w:p/>`+extra))
			before := fileBytes(t, filename)
			v := mustSnapshot(t, session)
			if _, err := session.Apply(context.Background(), ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
				{Type: "update_reference", TargetID: v.Blocks[0].ID, Reference: &ReferenceEdit{Action: "insert_table_list"}},
			}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
				t.Fatal("invalid caption list changed the original DOCX")
			}
		})
	}
}
