package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDOCXMovesHyperlinkWithOriginalParagraphAndRelationship(t *testing.T) {
	ctx := context.Background()
	first := `<w:p><w:r><w:t>first</w:t></w:r></w:p>`
	second := `<w:p><w:pPr><w:keepNext/></w:pPr><w:r><w:rPr><w:i/></w:rPr><w:t>linked </w:t></w:r></w:p>`
	third := `<w:p><w:r><w:t>third</w:t></w:r></w:p>`
	s, filename := withDocument(t, bodyXML(first+second+third))
	v := mustSnapshot(t, s)
	apply := func(op Operation) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}, "human")
	}
	const site = "https://example.com/reference"
	insert := Operation{Type: "update_reference", TargetID: v.Blocks[1].ID, Reference: &ReferenceEdit{
		Action: "insert_link", Text: "웹 링크", URL: site,
	}}
	v, err := apply(insert)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocks[1].Paragraph.MoveHyperlinkUp || !v.Blocks[1].Paragraph.MoveHyperlinkDown {
		t.Fatal("both adjacent plain paragraphs should be available")
	}
	linkPackage := fileBytes(t, filename)
	root, err := parseXML(s.pkg.document)
	if err != nil {
		t.Fatal(err)
	}
	linkParagraph := root.child("body").children[1]
	linkXML := string(s.pkg.document[linkParagraph.start:linkParagraph.end])
	rels := bytes.Clone(s.pkg.partData(documentRelationships, nil))
	v, err = apply(Operation{Type: "move_hyperlink_up", TargetID: v.Blocks[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(s.pkg.document), linkXML+first+third) || !bytes.Equal(rels, s.pkg.partData(documentRelationships, nil)) {
		t.Fatal("move changed source paragraph or relationship")
	}
	s, err = Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, s)
	linkRun := v.Blocks[0].Paragraph.Runs[1]
	if linkRun.Link == nil || linkRun.Link.URL != site || linkRun.Text != "웹 링크" || !v.Blocks[0].Paragraph.Runs[0].Italic {
		t.Fatalf("moved hyperlink lost original formatting or destination: %+v", linkRun)
	}
	if _, err := apply(Operation{Type: "move_hyperlink_down", TargetID: v.Blocks[0].ID}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(s.pkg.document), first+linkXML+third) {
		t.Fatal("move down did not restore source order")
	}
	historyRestore(t, s, "undo")
	historyRestore(t, s, "undo")
	if !bytes.Equal(linkPackage, fileBytes(t, filename)) {
		t.Fatal("history undo did not restore exact source package")
	}
}

func TestDOCXHyperlinkMoveRespectsOpaqueBoundaries(t *testing.T) {
	plain := `<w:p><w:r><w:t>plain</w:t></w:r></w:p>`
	link := `<w:p><w:hyperlink w:anchor="Place"><w:r><w:t>link</w:t></w:r></w:hyperlink></w:p>`
	for _, test := range []struct{ name, prefix, between, suffix, paragraph string }{
		{name: "valid", paragraph: link},
		{name: "comment range", prefix: `<w:p><w:commentRangeStart w:id="1"/></w:p>`, suffix: `<w:p><w:commentRangeEnd w:id="1"/></w:p>`, paragraph: link},
		{name: "bookmark range", prefix: `<w:p><w:bookmarkStart w:id="1" w:name="Wide"/></w:p>`, suffix: `<w:p><w:bookmarkEnd w:id="1"/></w:p>`, paragraph: link},
		{name: "XML comment", between: `<!-- keep here -->`, paragraph: link},
		{name: "table", between: `<w:tbl/>`, paragraph: link},
		{name: "page break", paragraph: strings.Replace(link, "<w:p>", "<w:p><w:pPr><w:pageBreakBefore/></w:pPr>", 1)},
		{name: "section break", paragraph: strings.Replace(link, "<w:p>", "<w:p><w:pPr><w:sectPr/></w:pPr>", 1)},
		{name: "opaque link", paragraph: strings.Replace(link, "<w:r>", "<w:r><w:fldChar w:fldCharType=\"begin\"/>", 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			s, filename := withDocument(t, bodyXML(test.prefix+plain+test.between+test.paragraph+test.suffix))
			v := mustSnapshot(t, s)
			original := fileBytes(t, filename)
			var target *Paragraph
			for _, block := range v.Blocks {
				if block.Paragraph != nil && len(block.Paragraph.Runs) != 0 && block.Paragraph.Runs[0].Text == "link" {
					target = block.Paragraph
				}
			}
			want := test.name == "valid"
			if target == nil || target.MoveHyperlinkUp != want {
				t.Fatalf("unexpected link paragraph movement availability: %+v", target)
			}
			input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
				{Type: "move_hyperlink_up", TargetID: target.ID},
			}}
			_, err := s.Apply(ctx, input, "human")
			if (err == nil) != want || !want && !bytes.Equal(original, fileBytes(t, filename)) {
				t.Fatalf("unexpected or non-atomic movement: %v", err)
			}
		})
	}
}
