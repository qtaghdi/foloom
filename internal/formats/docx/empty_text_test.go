package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestEmptyParagraphAndRunTypingPreservesPackage(t *testing.T) {
	for _, body := range []string{
		`<w:p/>`, `<w:p></w:p>`, `<w:p w:rsidR="123"><w:pPr><w:jc w:val="center"/></w:pPr></w:p>`,
		`<w:p><w:r/></w:p>`, `<w:p><w:r><w:rPr><w:b/></w:rPr></w:r></w:p>`,
		`<w:tbl><w:tr><w:tc><w:p/></w:tc></w:tr></w:tbl>`,
	} {
		t.Run(body, func(t *testing.T) {
			s, path := withDocument(t, bodyXML(body+`<w:sectPr/>`))
			before := mustSnapshot(t, s)
			var run Run
			kind := "replace_run_text"
			if before.Blocks[0].Type == "paragraph" {
				run = before.Blocks[0].Paragraph.Runs[0]
			} else {
				run = before.Blocks[0].Rows[0].Cells[0].Paragraphs[0].Runs[0]
				kind = "replace_cell_text"
			}
			if !run.Editable || run.Text != "" {
				t.Fatalf("missing empty editable target: %+v", run)
			}
			pkg, err := loadPackage(fileBytes(t, path))
			if err != nil {
				t.Fatal(err)
			}
			after, err := s.Apply(context.Background(), batch(before, kind, run, " 한글 typing <&> "), "human")
			if err != nil {
				t.Fatal(err)
			}
			if after.Revision != before.Revision+1 {
				t.Fatal("revision did not advance once")
			}
			reopened, err := Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			snap := mustSnapshot(t, reopened)
			if snap.Blocks[0].Type == "paragraph" {
				run = snap.Blocks[0].Paragraph.Runs[0]
			} else {
				run = snap.Blocks[0].Rows[0].Cells[0].Paragraphs[0].Runs[0]
			}
			if run.Text != " 한글 typing <&> " || !run.Editable {
				t.Fatalf("text did not reopen: %+v", run)
			}
			final, err := loadPackage(fileBytes(t, path))
			if err != nil {
				t.Fatal(err)
			}
			for index, part := range pkg.parts {
				name := part.header.Name
				if name != "word/document.xml" && !bytes.Equal(part.data, final.parts[index].data) {
					t.Fatalf("unrelated part changed: %s", name)
				}
			}
			if !strings.Contains(string(final.document), `<w:sectPr/>`) {
				t.Fatal("section properties changed")
			}
			if strings.Contains(body, `<w:pPr>`) && !strings.Contains(string(final.document), `<w:pPr><w:jc w:val="center"/></w:pPr>`) {
				t.Fatal("paragraph properties changed")
			}
		})
	}
}

func TestEmptyParagraphNamespaceAndProtectedStructures(t *testing.T) {
	s, _ := withDocument(t, `<x:document xmlns:x="`+wordNS+`"><x:body><x:p/></x:body></x:document>`)
	snap := mustSnapshot(t, s)
	if _, err := s.Apply(context.Background(), batch(snap, "replace_run_text", snap.Blocks[0].Paragraph.Runs[0], "alternate prefix"), "human"); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`<w:p><w:sdt/></w:p>`, `<w:p><w:pPr><w:pPrChange/></w:pPr></w:p>`,
		`<w:p><w:r><w:drawing/></w:r></w:p>`,
		`<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r></w:p><w:p/><w:p><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`,
	} {
		t.Run(body, func(t *testing.T) {
			s, _ := withDocument(t, bodyXML(body))
			snap := mustSnapshot(t, s)
			for _, block := range snap.Blocks {
				for _, run := range block.Paragraph.Runs {
					if run.Editable {
						t.Fatalf("protected structure became editable: %+v", run)
					}
				}
			}
		})
	}
}
