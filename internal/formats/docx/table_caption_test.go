package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func captionableTable(text string) string {
	return `<w:tbl><w:tblPr><w:tblW w:w="1800" w:type="dxa"/></w:tblPr><w:tblGrid><w:gridCol w:w="1800"/></w:tblGrid>` +
		`<w:tr><w:tc><w:tcPr><w:shd w:fill="C5D0E0"/></w:tcPr><w:p><w:r><w:rPr><w:b/></w:rPr><w:t>` +
		escapeText(text) + `</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`
}

func TestInsertNumberedTableCaptionRenumbersFollowingTablesAndFeedsList(t *testing.T) {
	ctx := context.Background()
	first := captionableTable("First data")
	second := captionableTable("Second data")
	figure := captionFixture("그림", figureSequence, "1", "Figure remains")
	anchor := `<w:p><w:r><w:t>List anchor</w:t></w:r></w:p>`
	body := bodyXML(anchor + figure + first + second)
	session, filename := withDocument(t, body)
	original := fileBytes(t, filename)
	initialPackage := session.pkg
	snapshot := mustSnapshot(t, session)
	insert := func(snapshot Snapshot, index int, text string) (Snapshot, error) {
		t.Helper()
		return session.Apply(ctx, ApplyInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision, Operations: []Operation{
			{Type: "update_table", TargetID: snapshot.Blocks[index].ID, Table: &TableEdit{Action: "insert_caption", Caption: text}},
		}}, "human")
	}
	if !snapshot.Blocks[2].TableCaptionInsertable || !snapshot.Blocks[3].TableCaptionInsertable {
		t.Fatal("simple body tables were not advertised for captions")
	}
	snapshot, err := insert(snapshot, 3, "Second & caption")
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Blocks[2].TableCaptionInsertable || snapshot.Blocks[4].TableCaptionInsertable {
		t.Fatal("caption before the second table was associated with the wrong table")
	}
	snapshot, err = insert(snapshot, 2, "First <caption>")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Blocks[3].Type != "table" || snapshot.Blocks[5].Type != "table" ||
		snapshot.Blocks[3].TableCaptionInsertable || snapshot.Blocks[5].TableCaptionInsertable {
		t.Fatal("captions were not directly above the selected tables")
	}
	xml := string(session.pkg.document)
	if !strings.Contains(xml, figure+`<w:p xmlns:w="`+wordNS+`"><w:pPr><w:keepNext/></w:pPr>`) ||
		!strings.Contains(xml, inlineXML(". First <caption>")+"</w:r></w:p>"+first) ||
		!strings.Contains(xml, inlineXML(". Second & caption")+"</w:r></w:p>"+second) ||
		!strings.Contains(xml, inlineXML("2")+"</w:r></w:fldSimple><w:r>"+inlineXML(". Second & caption")) {
		t.Fatal("caption creation changed surrounding markup, adjacency or numbering")
	}
	if _, err := insert(snapshot, 3, "Duplicate"); err == nil {
		t.Fatal("accepted a duplicate caption")
	}
	_, err = session.Apply(ctx, ApplyInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: snapshot.Blocks[0].ID, Reference: &ReferenceEdit{Action: "insert_table_list"}},
	}}, "ai")
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = mustSnapshot(t, reopened)
	list := snapshot.Blocks[0].Paragraph.Runs[0].Object
	if list == nil || list.Kind != "table_list" || list.Text != "표 1. First <caption>\n표 2. Second & caption" {
		t.Fatalf("the new captions were not in the saved native table list: %+v", list)
	}
	for _, part := range initialPackage.parts {
		if part.header.Name != "word/document.xml" && !bytes.Equal(part.data, reopened.pkg.partData(part.header.Name, nil)) {
			t.Fatalf("unrelated part %s changed", part.header.Name)
		}
	}
	for range 3 {
		historyRestore(t, reopened, "undo")
	}
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("three undos did not restore exact original package")
	}
}

func TestInsertTableCaptionRejectsProtectedTablesAndInvalidText(t *testing.T) {
	table := captionableTable("Data")
	for _, tc := range []struct {
		name, body, text string
		editable         bool
	}{
		{"empty text", table, " ", true},
		{"multiple lines", table, "one\ntwo", true},
		{"too much UTF-8", table, strings.Repeat("표", 342), true},
		{"caption below", table + captionFixture("표", "SEQ Table", "1", "Existing"), "Duplicate", false},
		{"reset sequence", captionFixture("표", `SEQ Table \r 3`, "3", "Existing") + table, "New", false},
		{"floating table", strings.Replace(table, "<w:tblPr>", "<w:tblPr><w:tblpPr/>", 1), "New", false},
		{"protected range", `<w:p><w:bookmarkStart w:id="19" w:name="protected"/></w:p>` + table + `<w:p><w:bookmarkEnd w:id="19"/></w:p>`, "New", false},
		{"complex field", `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText>SEQ Table</w:instrText></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>` + table, "New", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session, filename := withDocument(t, bodyXML(tc.body))
			original := fileBytes(t, filename)
			before := mustSnapshot(t, session)
			var block Block
			for _, candidate := range before.Blocks {
				if candidate.Type == "table" {
					block = candidate
					break
				}
			}
			if block.ID == "" || block.TableCaptionInsertable != tc.editable {
				t.Fatalf("unexpected advertised eligibility: %+v", block)
			}
			_, err := session.Apply(context.Background(), ApplyInput{SessionID: before.ID, BaseRevision: before.Revision, Operations: []Operation{
				{Type: "update_table", TargetID: block.ID, Table: &TableEdit{Action: "insert_caption", Caption: tc.text}},
			}}, "human")
			if err == nil || !bytes.Equal(original, fileBytes(t, filename)) || mustSnapshot(t, session).Revision != before.Revision {
				t.Fatal("invalid caption changed the source document")
			}
		})
	}
}
