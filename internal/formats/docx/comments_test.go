package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDOCXCommentStoresNativeRangeAndRestoresPackage(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>comment target</w:t></w:r></w:p><w:p><w:r><w:t>other</w:t></w:r></w:p>`))
	original := fileBytes(t, filename)
	v := mustSnapshot(t, s)
	operations := []Operation{{Type: "add_comment", TargetID: v.Blocks[0].ID, Comment: &CommentEdit{Text: "first\nsecond"}}}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: operations}
	next, err := s.Apply(ctx, input, "ai")
	if err != nil || len(next.Comments) != 1 || next.Comments[0].Text != "first\nsecond" || next.Comments[0].ParagraphID != next.Blocks[0].ID || next.Comments[0].Author != "AI" {
		t.Fatalf("comment save: %v %+v", err, next.Comments)
	}
	if next.Blocks[0].Paragraph.StructureEditable || !next.Blocks[0].Paragraph.Runs[0].Editable {
		t.Fatal("range must preserve text editing but protect topology")
	}
	for _, name := range []string{"commentRangeStart", "commentRangeEnd", "commentReference"} {
		if !strings.Contains(string(s.pkg.document), name) {
			t.Fatalf("missing %s", name)
		}
	}
	events, err := s.EventsAfter(ctx, 0)
	if err != nil || len(events) != 1 || events[0].TextOnly {
		t.Fatalf("comment event: %v %+v", err, events)
	}
	if _, err = s.Apply(ctx, input, "ai"); err == nil {
		t.Fatal("stale write succeeded")
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	if after := mustSnapshot(t, reopened); len(after.Comments) != 1 || after.Comments[0].ParagraphID != after.Blocks[0].ID {
		t.Fatal("comment location lost after reopen")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo changed original bytes")
	}
}

func TestDOCXCommentLimitsRangeToSelectedRuns(t *testing.T) {
	ctx := context.Background()
	s, path := withDocument(t, bodyXML(`<w:p><w:r><w:t>outside</w:t></w:r><w:r><w:rPr><w:b/></w:rPr><w:t>inside</w:t></w:r><w:r><w:t>after</w:t></w:r></w:p><w:p><w:r><w:t>other</w:t></w:r></w:p>`))
	original := fileBytes(t, path)
	v := mustSnapshot(t, s)
	runs := v.Blocks[0].Paragraph.Runs
	comment := CommentEdit{Text: "selection", StartRunID: runs[1].ID, EndRunID: v.Blocks[1].Paragraph.Runs[0].ID}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "add_comment", TargetID: v.Blocks[0].ID, Comment: &comment},
	}}
	if _, err := s.Apply(ctx, input, "human"); err == nil || !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("cross-paragraph comment was persisted")
	}
	comment.EndRunID = runs[1].ID
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	doc := string(s.pkg.document)
	if !strings.Contains(doc, `dwc:id="0"/><w:r><w:rPr><w:b/></w:rPr><w:t>inside</w:t></w:r><dwc:commentRangeEnd`) {
		t.Fatal("comment markers did not wrap the selected run")
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := mustSnapshot(t, reopened)
	if len(snapshot.Comments) != 1 || snapshot.Comments[0].ParagraphID != snapshot.Blocks[0].ID {
		t.Fatal("selection comment was not located after reopening")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("undo did not restore the original file")
	}
}

func TestDOCXCommentSplitsUTF16RangeWithoutLosingStyle(t *testing.T) {
	ctx := context.Background()
	s, path := withDocument(t, bodyXML(`<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>앞😀뒤</w:t></w:r><w:r><w:rPr><w:i/></w:rPr><w:t>이어간다</w:t></w:r></w:p>`))
	original := fileBytes(t, path)
	v := mustSnapshot(t, s)
	runs := v.Blocks[0].Paragraph.Runs
	start, end := 2, 2
	comment := CommentEdit{Text: "character range", StartRunID: runs[0].ID, EndRunID: runs[1].ID, StartOffset: &start, EndOffset: &end}
	input := ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{
		{Type: "add_comment", TargetID: v.Blocks[0].ID, Comment: &comment},
	}}
	if _, err := s.Apply(ctx, input, "human"); err == nil || !bytes.Equal(original, fileBytes(t, path)) {
		t.Fatal("offset inside a surrogate pair changed the document")
	}
	start = 1
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	checkRange := func(want string) {
		t.Helper()
		reopened, err := Open(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		root, err := parseXML(reopened.pkg.document)
		if err != nil {
			t.Fatal(err)
		}
		doc := root.child("body").child("p")
		var selected strings.Builder
		inRange := false
		for _, child := range doc.children {
			switch {
			case child.is("commentRangeStart"):
				inRange = true
			case child.is("commentRangeEnd"):
				inRange = false
			case child.is("r") && child.child("t") != nil:
				value := textContent(child)
				prop := "b"
				if strings.ContainsAny(value, "이어간다") {
					prop = "i"
				}
				if child.child("rPr").child(prop) == nil {
					t.Fatalf("formatting lost for %q", value)
				}
				if inRange {
					selected.WriteString(value)
				}
			}
		}
		if selected.String() != want || textContent(doc) != "앞😀뒤이어간다" {
			t.Fatalf("incorrect range %q or text %q", selected.String(), textContent(doc))
		}
		historyRestore(t, reopened, "undo")
		if !bytes.Equal(original, fileBytes(t, path)) {
			t.Fatal("undo did not restore the original file")
		}
	}
	checkRange("😀뒤이어")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, s)
	runs = v.Blocks[0].Paragraph.Runs
	input.SessionID, input.BaseRevision, input.Operations[0].TargetID = v.ID, v.Revision, v.Blocks[0].ID
	comment.StartRunID, comment.EndRunID, end = runs[0].ID, runs[0].ID, 3
	if _, err := s.Apply(ctx, input, "human"); err != nil {
		t.Fatal(err)
	}
	checkRange("😀")
}
