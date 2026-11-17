package docx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDOCXReferencesPreserveTargetsAndRejectUnsafeURLs(t *testing.T) {
	ctx := context.Background()
	s, filename := withDocument(t, bodyXML(`<w:p><w:r><w:t>link 1</w:t></w:r></w:p><w:p><w:r><w:t>target</w:t></w:r></w:p><w:p><w:r><w:t>link 2</w:t></w:r></w:p>`))
	original := fileBytes(t, filename)
	apply := func(index int, edit ReferenceEdit) (Snapshot, error) {
		v := mustSnapshot(t, s)
		return s.Apply(ctx, ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []Operation{{Type: "update_reference", TargetID: v.Blocks[index].ID, Reference: &edit}}}, "human")
	}
	for _, raw := range []string{"javascript:alert(1)", "file:///tmp/secret", "https://user:pass@example.com", "//example.com", "https://example.com/\nfoo"} {
		if _, err := apply(0, ReferenceEdit{Action: "insert_link", Text: "unsafe", URL: raw}); err == nil || !bytes.Equal(original, fileBytes(t, filename)) {
			t.Fatalf("unsafe URL accepted: %q", raw)
		}
	}
	if _, err := apply(0, ReferenceEdit{Action: "insert_link", Text: "missing", Bookmark: "Missing"}); err == nil {
		t.Fatal("missing bookmark was accepted")
	}
	if _, err := apply(1, ReferenceEdit{Action: "insert_bookmark", Bookmark: "Target"}); err != nil {
		t.Fatal(err)
	}
	if _, err := apply(2, ReferenceEdit{Action: "insert_bookmark", Bookmark: "target"}); err == nil {
		t.Fatal("duplicate case-insensitive bookmark accepted")
	}
	if _, err := apply(0, ReferenceEdit{Action: "insert_link", Text: "go here", Bookmark: "Target"}); err != nil {
		t.Fatal(err)
	}
	const external = "https://example.com/path?q=1&test=2"
	if _, err := apply(2, ReferenceEdit{Action: "insert_link", Text: "web", URL: external}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(s.pkg.document), `dwb:anchor="Target"`) || !strings.Contains(string(s.pkg.partData(documentRelationships, nil)), `Target="https://example.com/path?q=1&amp;test=2" TargetMode="External"`) {
		t.Fatal("missing encoded reference")
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	reloaded := mustSnapshot(t, reopened)
	if len(reloaded.Bookmarks) != 1 || reloaded.Bookmarks[0].ParagraphID != reloaded.Blocks[1].ID {
		t.Fatal("bookmark not projected after reopen")
	}
	internal, web := reloaded.Blocks[0].Paragraph.Runs[1].Link, reloaded.Blocks[2].Paragraph.Runs[1].Link
	if internal == nil || internal.Bookmark != "Target" || web == nil || web.URL != external {
		t.Fatal("safe links not projected after reopen")
	}
	prior := fileBytes(t, filename)
	ref := ReferenceEdit{Action: "set_link_url", URL: "javascript:alert(1)"}
	update := ApplyInput{SessionID: reloaded.ID, BaseRevision: reloaded.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: reloaded.Blocks[2].Paragraph.Runs[1].ID, Reference: &ref},
	}}
	if _, err := reopened.Apply(ctx, update, "human"); err == nil || !bytes.Equal(prior, fileBytes(t, filename)) {
		t.Fatal("unsafe retarget changed file")
	}
	ref.URL = "https://example.org/new"
	after, err := reopened.Apply(ctx, update, "human")
	if err != nil {
		t.Fatal(err)
	}
	run := after.Blocks[2].Paragraph.Runs[1]
	if run.Text != "web" || run.Link == nil || run.Link.URL != ref.URL ||
		!strings.Contains(string(reopened.pkg.document), `<dwb:color dwb:val="0563C1"/>`) ||
		!strings.Contains(string(reopened.pkg.partData(documentRelationships, nil)), `Target="https://example.com/path?q=1&amp;test=2"`) {
		t.Fatal("retarget changed the displayed text, style, or former relationship")
	}
	reopened, err = Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	if link := mustSnapshot(t, reopened).Blocks[2].Paragraph.Runs[1].Link; link == nil || link.URL != ref.URL {
		t.Fatal("new URL did not survive reopening")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(prior, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original URL")
	}
	current := mustSnapshot(t, reopened)
	unlinked, err := reopened.Apply(ctx, ApplyInput{SessionID: current.ID, BaseRevision: current.Revision, Operations: []Operation{
		{Type: "update_reference", TargetID: current.Blocks[2].Paragraph.Runs[1].ID, Reference: &ReferenceEdit{Action: "unlink"}},
	}}, "human")
	if err != nil {
		t.Fatal(err)
	}
	plain := unlinked.Blocks[2].Paragraph.Runs[1]
	if plain.Text != "web" || plain.Link != nil || !plain.Editable || !plain.Underline || plain.Color != "#0563C1" {
		t.Fatalf("unlink changed text or format: %+v", plain)
	}
	if !strings.Contains(string(reopened.pkg.partData(documentRelationships, nil)), `Target="https://example.com/path?q=1&amp;test=2"`) {
		t.Fatal("unlink removed the original relationship")
	}
	historyRestore(t, reopened, "undo")
	if !bytes.Equal(prior, fileBytes(t, filename)) {
		t.Fatal("undo did not restore the original hyperlink")
	}
	for range 3 {
		historyRestore(t, reopened, "undo")
	}
	if !bytes.Equal(original, fileBytes(t, filename)) {
		t.Fatal("undo changed original bytes")
	}
}
