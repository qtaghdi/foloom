package docx

import (
	"bytes"
	"context"
	"testing"
)

func TestNewCommentsSupportNativeRepliesAndResolution(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p xmlns:w14="`+word2010NS+`" w14:paraId="00000001"><w:r><w:t>first</w:t></w:r></w:p><w:p><w:r><w:t>second</w:t></w:r></w:p>`))
	before := fileBytes(t, filename)
	apply := func(op Operation) Snapshot {
		t.Helper()
		snapshot := mustSnapshot(t, s)
		out, err := s.Apply(ctx, ApplyInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision, Operations: []Operation{op}}, "human")
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	v := mustSnapshot(t, s)
	v = apply(Operation{Type: "add_comment", TargetID: v.Blocks[0].ID, Comment: &CommentEdit{Text: "새 주석\n둘째 줄"}})
	v = apply(Operation{Type: "add_comment", TargetID: v.Blocks[1].ID, Comment: &CommentEdit{Text: "별도 주석"}})
	first, second := v.Comments[0], v.Comments[1]
	if !first.ResolutionEditable || !second.ResolutionEditable || first.Resolved || first.Text != "새 주석\n둘째 줄" {
		t.Fatalf("new comments lack thread metadata: %+v", v.Comments)
	}
	document := bytes.Clone(s.pkg.document)
	apply(Operation{Type: "reply_comment", TargetID: first.ID, Comment: &CommentEdit{Text: "답글"}})
	apply(Operation{Type: "resolve_comment", TargetID: first.ID})
	s, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, s)
	const part = "word/commentsExtended.xml"
	if len(v.Comments) != 3 || !v.Comments[0].Resolved || v.Comments[1].Resolved ||
		v.Comments[2].ParentID != first.ID || v.Comments[2].Text != "답글" || !bytes.Equal(document, s.pkg.document) ||
		!bytes.Contains(s.pkg.partData(part, nil), []byte(`w15:paraId="00000004" w15:paraIdParent="00000002"`)) ||
		!bytes.Contains(s.pkg.partData(documentRelationships, nil), []byte(commentExtRelType)) ||
		!bytes.Contains(s.pkg.partData("[Content_Types].xml", nil), []byte(commentExtContentType)) {
		t.Fatalf("persisted native comment metadata or original markers missing: %+v", v.Comments)
	}
	v = apply(Operation{Type: "reopen_comment", TargetID: first.ID})
	if v.Comments[0].Resolved {
		t.Fatal("created comment could not be reopened")
	}
	for range 5 {
		historyRestore(t, s, "undo")
	}
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original DOCX")
	}
}

func TestAddingCommentKeepsLegacyMetadataUnchanged(t *testing.T) {
	ctx := context.Background()
	_, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>body</w:t></w:r></w:p>`))
	comments := []byte(`<w:comments xmlns:w="` + wordNS + `"><w:comment w:id="0"><w:p><w:r><w:t>legacy</w:t></w:r></w:p></w:comment></w:comments>`)
	rels := []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="comments" Type="` + officeRelNS + `/comments" Target="comments.xml"/></Relationships>`)
	rewritePackage(t, filename, func(_ string, data []byte) []byte { return data }, map[string][]byte{
		"word/comments.xml": comments, documentRelationships: rels,
	})
	s, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	before := mustSnapshot(t, s)
	after, err := s.Apply(ctx, ApplyInput{SessionID: before.ID, BaseRevision: before.Revision, Operations: []Operation{{
		Type: "add_comment", TargetID: before.Blocks[0].ID, Comment: &CommentEdit{Text: "new"},
	}}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Comments) != 2 || after.Comments[0].Text != "legacy" || after.Comments[1].ResolutionEditable ||
		s.pkg.partData("word/commentsExtended.xml", nil) != nil || !bytes.Equal(s.pkg.partData(documentRelationships, nil), rels) {
		t.Fatal("adding a note changed unsupported legacy comment metadata")
	}
}
