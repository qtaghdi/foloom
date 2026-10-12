package xlsx

import (
	"fmt"
	"strings"
	"testing"
)

func TestHistoryEntryLimitSurvivesRestartAndKeepsRevisionsMonotonic(t *testing.T) {
	session, path := newTransactionSession(t, "value-0")
	limits := defaultHistoryLimits()
	if limits.entries != 10 || limits.snapshotBytes != 32<<20 {
		t.Fatalf("default history limits = %#v", limits)
	}

	for edit := 1; edit <= 12; edit++ {
		baseRevision := session.Snapshot().Revision
		snapshot, err := session.Apply(baseRevision, "human", []Operation{{
			Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: fmt.Sprintf("value-%d", edit),
		}})
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Revision != uint64(edit+1) {
			t.Fatalf("edit %d revision = %d, want %d", edit, snapshot.Revision, edit+1)
		}
	}
	if len(session.undoHistory) != limits.entries || len(session.redoHistory) != 0 {
		t.Fatalf("in-memory history = undo %d, redo %d", len(session.undoHistory), len(session.redoHistory))
	}
	manifest, err := readHistoryManifest(session.historyStore.manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Revision != 13 || len(manifest.Undo) != limits.entries || len(manifest.Redo) != 0 {
		t.Fatalf("durable history before restart = %#v", manifest)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if snapshot := reopened.Snapshot(); snapshot.Revision != 13 || !snapshot.CanUndo || snapshot.CanRedo {
		t.Fatalf("history after restart = %#v", snapshot)
	}

	for step := 1; step <= limits.entries; step++ {
		before := reopened.Snapshot().Revision
		snapshot, err := reopened.RestoreHistory(before, "human", "undo")
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Revision != before+1 {
			t.Fatalf("undo %d revision = %d, want %d", step, snapshot.Revision, before+1)
		}
		value, err := reopened.ReadRange("Sheet1", "A1")
		want := fmt.Sprintf("value-%d", 12-step)
		if err != nil || value.Rows[0][0].Value != want {
			t.Fatalf("undo %d value = %#v, want %q, error = %v", step, value, want, err)
		}
	}
	afterUndo := reopened.Snapshot()
	if afterUndo.Revision != 23 || afterUndo.CanUndo || !afterUndo.CanRedo {
		t.Fatalf("history at retained undo boundary = %#v", afterUndo)
	}
	if _, err := reopened.RestoreHistory(afterUndo.Revision, "human", "undo"); err == nil || !strings.Contains(err.Error(), "nothing to undo") {
		t.Fatalf("extra undo error = %v", err)
	}
	if reopened.Snapshot().Revision != afterUndo.Revision {
		t.Fatal("unavailable undo advanced the revision")
	}

	for step := 1; step <= limits.entries; step++ {
		before := reopened.Snapshot().Revision
		snapshot, err := reopened.RestoreHistory(before, "human", "redo")
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Revision != before+1 {
			t.Fatalf("redo %d revision = %d, want %d", step, snapshot.Revision, before+1)
		}
		value, err := reopened.ReadRange("Sheet1", "A1")
		want := fmt.Sprintf("value-%d", step+2)
		if err != nil || value.Rows[0][0].Value != want {
			t.Fatalf("redo %d value = %#v, want %q, error = %v", step, value, want, err)
		}
	}
	afterRedo := reopened.Snapshot()
	if afterRedo.Revision != 33 || !afterRedo.CanUndo || afterRedo.CanRedo {
		t.Fatalf("history at retained redo boundary = %#v", afterRedo)
	}

	events, _ := reopened.EventsAfter(0)
	if len(events) != 20 {
		t.Fatalf("history event count = %d, want 20", len(events))
	}
	for index, event := range events {
		if event.Sequence != uint64(index+1) || event.Revision != uint64(index+14) || event.Type != "workbook.reload" {
			t.Fatalf("history event %d = %#v", index, event)
		}
	}
	lastUndo := events[limits.entries-1]
	if lastUndo.CanUndo == nil || *lastUndo.CanUndo || lastUndo.CanRedo == nil || !*lastUndo.CanRedo {
		t.Fatalf("last undo availability = %#v", lastUndo)
	}
	lastRedo := events[len(events)-1]
	if lastRedo.CanUndo == nil || !*lastRedo.CanUndo || lastRedo.CanRedo == nil || *lastRedo.CanRedo {
		t.Fatalf("last redo availability = %#v", lastRedo)
	}
}

func TestOversizedHistorySnapshotDoesNotBlockCommit(t *testing.T) {
	session, path := newTransactionSession(t, "original")
	limits := defaultHistoryLimits()
	limits.snapshotBytes = 1
	session.historyLimits = limits

	snapshot, err := session.Apply(1, "human", []Operation{{
		Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "committed without history",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Revision != 2 || snapshot.CanUndo || snapshot.CanRedo {
		t.Fatalf("snapshot after oversized history omission = %#v", snapshot)
	}
	if len(session.undoHistory) != 0 || len(session.redoHistory) != 0 {
		t.Fatalf("oversized snapshot retained: undo %d, redo %d", len(session.undoHistory), len(session.redoHistory))
	}
	manifest, err := readHistoryManifest(session.historyStore.manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Revision != 2 || len(manifest.Undo) != 0 || len(manifest.Redo) != 0 {
		t.Fatalf("durable history after oversized omission = %#v", manifest)
	}
	events, _ := session.EventsAfter(0)
	commit := events[len(events)-1]
	if commit.CanUndo == nil || *commit.CanUndo || commit.CanRedo == nil || *commit.CanRedo {
		t.Fatalf("commit availability = %#v", commit)
	}
	if _, err := session.RestoreHistory(2, "human", "undo"); err == nil || !strings.Contains(err.Error(), "nothing to undo") {
		t.Fatalf("undo after oversized omission = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted := reopened.Snapshot()
	if restarted.Revision != 2 || restarted.CanUndo || restarted.CanRedo {
		t.Fatalf("history after restart = %#v", restarted)
	}
	value, err := reopened.ReadRange("Sheet1", "A1")
	if err != nil || value.Rows[0][0].Value != "committed without history" {
		t.Fatalf("committed value = %#v, error = %v", value, err)
	}
	if _, err := reopened.RestoreHistory(2, "human", "undo"); err == nil || !strings.Contains(err.Error(), "nothing to undo") {
		t.Fatalf("restarted undo = %v", err)
	}
}
