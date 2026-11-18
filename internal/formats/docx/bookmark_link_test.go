package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRetargetBookmarkLinkPreservesRunXMLAndRelationships(t *testing.T) {
	ctx := context.Background()
	const contents = `<w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">목차 </w:t></w:r><w:r><w:rPr><w:i/></w:rPr><w:t>이동</w:t></w:r>`
	s, filename := withDocument(t, bodyXML(`<w:p><w:hyperlink w:anchor="Old" w:tooltip="Keep tip">`+contents+`</w:hyperlink><w:hyperlink w:anchor="Old"><w:r><w:t>Other link</w:t></w:r></w:hyperlink></w:p>`+
		`<w:p><w:bookmarkStart w:id="1" w:name="Old"/><w:r><w:t>Old target</w:t></w:r><w:bookmarkEnd w:id="1"/></w:p>`+
		`<w:p><w:bookmarkStart w:id="2" w:name="New"/><w:r><w:t>New target</w:t></w:r><w:bookmarkEnd w:id="2"/></w:p><w:p/>`))
	v := mustSnapshot(t, s)
	v, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: v.Blocks[3].ID, Reference: &ReferenceEdit{Action: "insert_link", Text: "Web", URL: "https://example.com/"}},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	live := v.Blocks[0].Paragraph.Runs[0]
	if live.Link == nil || live.Link.Bookmark != "Old" || !live.Link.BookmarkEditable {
		t.Fatal("valid bookmark link was not exposed as editable")
	}
	relationships := s.pkg.partData(documentRelationships, nil)
	prior := fileBytes(t, filename)
	update := func(base uint64, edit ReferenceEdit) (Snapshot, error) {
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: base, Operations: []Operation{
			{Type: "update_reference", TargetID: live.ID, Reference: &edit},
		}}, "human")
	}
	if _, err := update(v.Revision-1, ReferenceEdit{Action: "set_link_bookmark", Bookmark: "New"}); err == nil {
		t.Fatal("stale revision was accepted")
	}
	for _, invalid := range []ReferenceEdit{
		{Action: "set_link_bookmark", Bookmark: "Missing"},
		{Action: "set_link_bookmark", Bookmark: "#New"},
		{Action: "set_link_bookmark", Bookmark: "New", Text: "changed text"},
		{Action: "set_link_bookmark", Bookmark: "New", URL: "https://example.org/"},
		{Action: "set_link_bookmark", Bookmark: "New", NewBookmark: "other"},
	} {
		if _, err := update(v.Revision, invalid); err == nil {
			t.Fatalf("unexpected bookmark link edit accepted: %+v", invalid)
		}
	}
	unchanged, err := update(v.Revision, ReferenceEdit{Action: "set_link_bookmark", Bookmark: "OLD"})
	if err != nil || unchanged.Revision != v.Revision || !bytes.Equal(prior, fileBytes(t, filename)) {
		t.Fatal("invalid or unchanged destination changed the document")
	}

	if _, err := update(v.Revision, ReferenceEdit{Action: "set_link_bookmark", Bookmark: "new"}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	saved := mustSnapshot(t, reopened)
	link := saved.Blocks[0].Paragraph.Runs[0]
	other := saved.Blocks[0].Paragraph.Runs[1]
	if link.Link == nil || link.Link.Bookmark != "New" || link.Text != "목차 이동" ||
		other.Link == nil || other.Link.Bookmark != "Old" {
		t.Fatal("wrong link text or destination after reopen")
	}
	document := string(reopened.pkg.document)
	if !strings.Contains(document, contents) || !strings.Contains(document, `w:tooltip="Keep tip"`) ||
		!bytes.Equal(relationships, reopened.pkg.partData(documentRelationships, nil)) {
		t.Fatal("retarget changed link contents, tooltip, or relationships")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(prior, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the exact original DOCX")
	}
}

func TestRetargetBookmarkLinkRejectsUnsupportedLinksAndTargets(t *testing.T) {
	ctx := context.Background()
	const link = `<w:hyperlink w:anchor="Old"><w:r><w:t>Jump</w:t></w:r></w:hyperlink>`
	const destinations = `<w:p><w:bookmarkStart w:id="1" w:name="Old"/><w:r><w:t>Old target</w:t></w:r><w:bookmarkEnd w:id="1"/></w:p>
<w:p><w:bookmarkStart w:id="2" w:name="New"/><w:r><w:t>New target</w:t></w:r><w:bookmarkEnd w:id="2"/></w:p>`
	for name, modify := range map[string]func(string) string{
		"external": func(body string) string {
			return strings.Replace(body, `w:anchor="Old"`, `xmlns:r="`+officeRelNS+`" r:id="rIdWeb"`, 1)
		},
		"document location": func(body string) string {
			return strings.Replace(body, `w:anchor="Old"`, `w:anchor="Old" w:docLocation="other"`, 1)
		},
		"complex field": func(body string) string {
			return `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText>HYPERLINK</w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r></w:p>` +
				strings.Replace(body, `</w:p>`, `</w:p><w:p><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`, 1)
		},
		"ambiguous target": func(body string) string {
			return body + `<w:p><w:bookmarkStart w:id="3" w:name="new"/><w:bookmarkEnd w:id="3"/></w:p>`
		},
		"unpaired target": func(body string) string {
			return strings.Replace(body, `<w:bookmarkEnd w:id="2"/>`, "", 1)
		},
		"dangling source": func(body string) string {
			return strings.Replace(body, `w:anchor="Old"`, `w:anchor="Missing"`, 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, filename := withDocument(t, bodyXML(modify(`<w:p>`+link+`</w:p>`+destinations)))
			before := fileBytes(t, filename)
			v := mustSnapshot(t, s)
			for _, block := range v.Blocks {
				if block.Paragraph == nil {
					continue
				}
				for _, run := range block.Paragraph.Runs {
					if run.Text != "Jump" {
						continue
					}
					if _, err := s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
						{Type: "update_reference", TargetID: run.ID, Reference: &ReferenceEdit{Action: "set_link_bookmark", Bookmark: "New"}},
					}}, "human"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
						t.Fatal("protected bookmark link edit changed the DOCX")
					}
					return
				}
			}
			t.Fatal("fixture lost its hyperlink")
		})
	}
}
