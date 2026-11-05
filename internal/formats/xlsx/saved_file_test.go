package xlsx

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSavedFilePreservesCommittedBytesAndRevision(t *testing.T) {
	for _, fixture := range []string{"feature-rich.xlsx", "libreoffice.xlsx"} {
		t.Run(fixture, func(t *testing.T) {
			content, err := os.ReadFile(compatibilityFixturePath(t, fixture))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), fixture)
			if err = os.WriteFile(path, content, 0600); err != nil {
				t.Fatal(err)
			}
			session, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			snapshot := session.Snapshot()
			downloaded, err := session.SavedFile(context.Background(), snapshot.ID, snapshot.Revision)
			if err != nil || !bytes.Equal(downloaded, content) {
				t.Fatalf("original bytes changed: %v", err)
			}
			snapshot, err = session.Apply(snapshot.Revision, "human", []Operation{{Type: "set_cell", Sheet: "Compatibility", Cell: "Z50", Value: "download marker"}})
			if err != nil {
				t.Fatal(err)
			}
			committed, _ := os.ReadFile(path)
			downloaded, err = session.SavedFile(context.Background(), snapshot.ID, snapshot.Revision)
			if err != nil || !bytes.Equal(downloaded, committed) {
				t.Fatalf("committed bytes changed: %v", err)
			}
			after := session.Snapshot()
			if after.Revision != snapshot.Revision || after.Sequence != snapshot.Sequence || after.CanUndo != snapshot.CanUndo {
				t.Fatal("download changed history")
			}
			copyPath := filepath.Join(t.TempDir(), "download.xlsx")
			os.WriteFile(copyPath, downloaded, 0600)
			reopened, err := Open(copyPath)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			cells, err := reopened.ReadRange("Compatibility", "Z50")
			if err != nil || cells.Rows[0][0].Value != "download marker" {
				t.Fatal("saved edit missing from download")
			}
			if _, err = session.SavedFile(context.Background(), "another-session", snapshot.Revision); !errors.Is(err, ErrRevisionConflict) {
				t.Fatalf("session guard: %v", err)
			}
			if _, err = session.SavedFile(context.Background(), snapshot.ID, snapshot.Revision-1); !errors.Is(err, ErrRevisionConflict) {
				t.Fatalf("revision guard: %v", err)
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
		})
	}
}
