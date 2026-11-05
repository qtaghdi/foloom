package docx

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSavedFilePreservesCommittedPackageAndGuardsRevision(t *testing.T) {
	session, path := createTest(t)
	snapshot := mustSnapshot(t, session)
	original := fileBytes(t, path)
	downloaded, err := session.SavedFile(context.Background(), snapshot.ID, snapshot.Revision)
	if err != nil || !bytes.Equal(original, downloaded) {
		t.Fatalf("original bytes changed: %v", err)
	}
	snapshot, err = session.Apply(context.Background(), batch(snapshot, "replace_run_text", snapshot.Blocks[0].Paragraph.Runs[0], "saved download"), "human")
	if err != nil {
		t.Fatal(err)
	}
	downloaded, err = session.SavedFile(context.Background(), snapshot.ID, snapshot.Revision)
	if err != nil || !bytes.Equal(downloaded, fileBytes(t, path)) {
		t.Fatalf("committed bytes changed: %v", err)
	}
	after := mustSnapshot(t, session)
	if after.Revision != snapshot.Revision || after.Sequence != snapshot.Sequence {
		t.Fatal("download changed revision")
	}
	copyPath := filepath.Join(t.TempDir(), "download.docx")
	os.WriteFile(copyPath, downloaded, 0600)
	reopened, err := Open(context.Background(), copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := mustSnapshot(t, reopened).Blocks[0].Paragraph.Runs[0].Text; got != "saved download" {
		t.Fatalf("download text: %q", got)
	}
	for _, input := range []struct {
		id       string
		revision uint64
	}{{"other", snapshot.Revision}, {snapshot.ID, snapshot.Revision - 1}} {
		if _, err = session.SavedFile(context.Background(), input.id, input.revision); !errors.Is(err, ErrRevisionConflict) {
			t.Fatalf("revision guard: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = session.SavedFile(ctx, snapshot.ID, snapshot.Revision); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	os.WriteFile(path, []byte("external replacement"), 0600)
	if _, err = session.SavedFile(context.Background(), snapshot.ID, snapshot.Revision); !errors.Is(err, ErrExternalChange) {
		t.Fatalf("external change: %v", err)
	}
}
