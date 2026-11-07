package docx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func historyEdit(t *testing.T, s *Session, text, actor string) Snapshot {
	t.Helper()
	v := mustSnapshot(t, s)
	next, err := s.Apply(context.Background(), batch(v, "replace_run_text", v.Blocks[0].Paragraph.Runs[0], text), actor)
	if err != nil {
		t.Fatal(err)
	}
	return next
}
func historyRestore(t *testing.T, s *Session, direction string) Snapshot {
	t.Helper()
	v := mustSnapshot(t, s)
	next, err := s.RestoreHistory(context.Background(), HistoryInput{v.ID, v.Revision, direction}, "human")
	if err != nil {
		t.Fatal(err)
	}
	return next
}
func TestHistoryRestartsAndPreservesExactPackage(t *testing.T) {
	ctx := context.Background()
	s, path := createTest(t)
	original := fileBytes(t, path)
	v := historyEdit(t, s, "AI text", "ai")
	edited := fileBytes(t, path)
	if v.History.Undo != 1 || v.History.UndoActor != "ai" {
		t.Fatal(v.History)
	}
	v = historyRestore(t, s, "undo")
	if !bytes.Equal(original, fileBytes(t, path)) || v.Revision != 3 || v.History.RedoActor != "ai" {
		t.Fatal("undo did not restore exact package")
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if mustSnapshot(t, reopened).Revision != 3 {
		t.Fatal("revision lost on reopen")
	}
	v = historyRestore(t, reopened, "redo")
	if !bytes.Equal(edited, fileBytes(t, path)) || v.Revision != 4 {
		t.Fatal("redo changed package")
	}
	events, err := reopened.EventsAfter(ctx, 0)
	if err != nil || len(events) != 1 || events[0].Revision != 4 {
		t.Fatalf("events: %+v %v", events, err)
	}
	historyRestore(t, reopened, "undo")
	v = historyEdit(t, reopened, "new branch", "human")
	if v.History.Redo != 0 || v.History.UndoActor != "human" {
		t.Fatal(v.History)
	}
}
func TestHistoryRejectsStaleExternalAndPersistenceFailure(t *testing.T) {
	s, path := createTest(t)
	v := historyEdit(t, s, "edited", "human")
	content := fileBytes(t, path)
	input := HistoryInput{v.ID, v.Revision, "undo"}
	if _, err := s.RestoreHistory(context.Background(), HistoryInput{v.ID, v.Revision - 1, "undo"}, "ai"); !errors.Is(err, ErrRevisionConflict) {
		t.Fatal(err)
	}
	s.rename = func(string, string) error { return fmt.Errorf("injected failure") }
	if _, err := s.RestoreHistory(context.Background(), input, "human"); err == nil {
		t.Fatal("rename failure accepted")
	}
	if !bytes.Equal(content, fileBytes(t, path)) || mustSnapshot(t, s).Revision != v.Revision {
		t.Fatal("failed undo mutated document")
	}
	s.rename = os.Rename
	if err := os.WriteFile(path, append(content, 0), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RestoreHistory(context.Background(), input, "human"); !errors.Is(err, ErrExternalChange) {
		t.Fatal(err)
	}
}
func TestHistoryBoundsCorruptionAndSymlinks(t *testing.T) {
	s, path := createTest(t)
	for i := 0; i < 14; i++ {
		historyEdit(t, s, fmt.Sprintf("edit %d", i), "human")
	}
	v := mustSnapshot(t, s)
	if v.History.Undo != maxHistoryEntries {
		t.Fatal(v.History)
	}
	entries, err := os.ReadDir(s.history.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > maxHistoryEntries+1 {
		t.Fatal("unbounded snapshots", len(entries))
	}
	latest := s.history.manifest.Undo[len(s.history.manifest.Undo)-1]
	snapshotPath := filepath.Join(s.history.dir, latest.Hash+".docx")
	if err = os.WriteFile(snapshotPath, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	before := fileBytes(t, path)
	if _, err = s.RestoreHistory(context.Background(), HistoryInput{v.ID, v.Revision, "undo"}, "human"); err == nil {
		t.Fatal("corrupt snapshot accepted")
	}
	if !bytes.Equal(before, fileBytes(t, path)) {
		t.Fatal("corrupt undo mutated file")
	}
	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	v = mustSnapshot(t, reopened)
	if v.History.Undo != 0 || len(v.Warnings) < 3 {
		t.Fatal("corruption not reported")
	}
	target := filepath.Join(t.TempDir(), "outside")
	if err = os.WriteFile(target, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, snapshotPath); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RestoreHistory(context.Background(), HistoryInput{s.id, s.revision, "undo"}, "human"); err == nil {
		t.Fatal("symlink snapshot accepted")
	}
	if string(fileBytes(t, target)) != "untouched" {
		t.Fatal("followed external symlink")
	}
}
func TestHistoryReconcilesOnlyCommittedPending(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprint(committed), func(t *testing.T) {
			s, path := createTest(t)
			historyEdit(t, s, "first", "human")
			before := s.history.manifest
			v := mustSnapshot(t, s)
			content, err := s.pkg.serialize([]byte(string(s.pkg.document)))
			if err != nil {
				t.Fatal(err)
			}
			// A pending record matching current bytes is a metadata-only interrupted commit.
			m := before
			m.Revision = v.Revision + 1
			if !committed {
				m.Current = historyHash([]byte("not the current file"))
			}
			if err = s.history.prepare(m, content); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			want := v.Revision
			if committed {
				want++
			}
			if got := mustSnapshot(t, reopened).Revision; got != want {
				t.Fatalf("revision %d want %d", got, want)
			}
			if _, err = os.Lstat(filepath.Join(s.history.dir, "pending.json")); !os.IsNotExist(err) {
				t.Fatal("pending not reconciled", err)
			}
		})
	}
}
