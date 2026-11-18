package docx

import (
	"bytes"
	"context"
	"testing"
)

func TestImportedCommentResolutionPreservesOriginalCommentAndReply(t *testing.T) {
	ctx := context.Background()
	_, filename := withDocument(t, bodyXML(`<w:p><w:commentRangeStart w:id="0"/><w:r><w:t>body</w:t></w:r><w:commentRangeEnd w:id="0"/><w:r><w:commentReference w:id="0"/></w:r></w:p>`))
	comments := []byte(`<w:comments xmlns:w="` + wordNS + `" xmlns:w14="` + word2010NS + `"><w:comment w:id="0" w:author="Reviewer"><w:p w14:paraId="01234567"><w:r><w:t>root</w:t></w:r></w:p></w:comment><w:comment w:id="1"><w:p w14:paraId="10234567"><w:r><w:t>reply</w:t></w:r></w:p></w:comment></w:comments>`)
	reply := `<w15:commentEx w15:paraId="10234567" w15:paraIdParent="01234567"/>`
	extended := []byte(`<w15:commentsEx xmlns:w15="` + commentExtNS + `"><w15:commentEx w15:paraId="01234567" w15:done="0"/>` + reply + `</w15:commentsEx>`)
	rels := []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="comments" Type="` + officeRelNS + `/comments" Target="comments.xml"/><Relationship Id="commentsExtended" Type="` + commentExtRelType + `" Target="commentsExtended.xml"/></Relationships>`)
	rewritePackage(t, filename, func(_ string, data []byte) []byte { return data }, map[string][]byte{
		"word/comments.xml": comments, "word/commentsExtended.xml": extended, documentRelationships: rels,
	})
	s, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	original := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	if len(v.Comments) != 2 || !v.Comments[0].ResolutionEditable || v.Comments[0].Resolved || v.Comments[1].ResolutionEditable {
		t.Fatalf("invalid imported comment state: %+v", v.Comments)
	}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "resolve_comment", TargetID: "1"}}}
	if _, err := s.Apply(ctx, input, "human"); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("a reply was resolved directly")
	}
	input.Operations[0].TargetID = "0"
	saved, err := s.Apply(ctx, input, "human")
	if err != nil || !saved.Comments[0].Resolved || saved.Comments[0].Text != "root" || saved.Comments[1].Text != "reply" {
		t.Fatalf("resolution did not retain root or reply: %v", err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	if !v.Comments[0].Resolved || !bytes.Equal(reopened.pkg.partData("word/comments.xml", nil), comments) ||
		!bytes.Contains(reopened.pkg.partData("word/commentsExtended.xml", nil), []byte(reply)) {
		t.Fatal("resolution or the unrelated comment parts changed on disk")
	}
	updated, err := reopened.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "reopen_comment", TargetID: "0"},
	}}, "human")
	if err != nil || updated.Comments[0].Resolved {
		t.Fatal("could not reopen resolved comment", err)
	}
	historyRestore(t, reopened, "undo")
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo did not restore exact original comment state")
	}
}

func TestImportedCommentReplyPreservesThreadAndAllocatesNativeIDs(t *testing.T) {
	ctx := context.Background()
	_, filename := withDocument(t, bodyXML(`<w:p xmlns:w14="`+word2010NS+`" w14:paraId="00000001"><w:commentRangeStart w:id="0"/><w:r><w:t>anchored</w:t></w:r><w:commentRangeEnd w:id="0"/><w:r><w:commentReference w:id="0"/></w:r></w:p>`))
	comments := []byte(`<w:comments xmlns:w="` + wordNS + `" xmlns:w14="` + word2010NS + `"><w:comment w:id="0"><w:p w14:paraId="01234567"><w:r><w:t>original thread</w:t></w:r></w:p></w:comment><w:comment w:id="1"><w:p w14:paraId="10234567"><w:r><w:t>old reply</w:t></w:r></w:p></w:comment></w:comments>`)
	extended := []byte(`<w15:commentsEx xmlns:w15="` + commentExtNS + `"><w15:commentEx w15:paraId="01234567" w15:done="1"/><w15:commentEx w15:paraId="10234567" w15:paraIdParent="01234567"/></w15:commentsEx>`)
	rels := []byte(`<Relationships xmlns="` + packageRelNS + `"><Relationship Id="comments" Type="` + officeRelNS + `/comments" Target="comments.xml"/><Relationship Id="extended" Type="` + commentExtRelType + `" Target="commentsExtended.xml"/></Relationships>`)
	rewritePackage(t, filename, func(_ string, data []byte) []byte { return data }, map[string][]byte{
		"word/comments.xml": comments, "word/commentsExtended.xml": extended, documentRelationships: rels,
	})
	s, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	before := fileBytes(t, filename)
	body := bytes.Clone(s.pkg.document)
	v := mustSnapshot(t, s)
	op := Operation{Type: "reply_comment", TargetID: "1", Comment: &CommentEdit{Text: "새 답글 & 근거"}}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{op}}
	if _, err := s.Apply(ctx, input, "ai"); err == nil || !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("reply to another reply was accepted")
	}
	input.Operations[0].TargetID = "0"
	if _, err := s.Apply(ctx, input, "ai"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	if len(v.Comments) != 3 || !v.Comments[0].Resolved || v.Comments[0].ParagraphID == "" ||
		v.Comments[1].ParentID != "0" || v.Comments[2].ParentID != "0" || v.Comments[2].ID != "2" ||
		v.Comments[2].Text != op.Comment.Text || v.Comments[2].Author != "AI" || !bytes.Equal(reopened.pkg.document, body) ||
		!bytes.Contains(reopened.pkg.partData("word/commentsExtended.xml", nil), []byte(`w15:paraId="00000002" w15:paraIdParent="01234567"`)) {
		t.Fatalf("saved reply lost its native linkage, thread or original markers: %+v", v.Comments)
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(before, fileBytes(t, filename)) {
		t.Fatal("undo did not restore original comment and metadata parts")
	}
}
