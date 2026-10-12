package xlsx

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestMixedBatchRejectsAllChangesAcrossTransactionBoundaries(t *testing.T) {
	session, path := newTransactionSession(t, "original")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = session.Apply(1, "ai", []Operation{
		{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "must not commit"},
		{Type: "set_cell", Sheet: "Missing", Cell: "B1", Value: "invalid"},
	})
	if err == nil {
		t.Fatal("mixed valid and invalid batch unexpectedly succeeded")
	}

	snapshot := session.Snapshot()
	if snapshot.Revision != 1 || snapshot.Sequence != 0 || snapshot.CanUndo || snapshot.CanRedo {
		t.Fatalf("snapshot changed after rejected batch: %#v", snapshot)
	}
	value, err := session.ReadRange("Sheet1", "A1")
	if err != nil || value.Rows[0][0].Value != "original" {
		t.Fatalf("memory value = %#v, error = %v", value, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatal("workbook bytes changed after rejected batch")
	}
	if _, err := os.Stat(session.historyStore.manifestPath); !os.IsNotExist(err) {
		t.Fatalf("history manifest exists after rejected batch: %v", err)
	}
	events, eventSnapshot := session.EventsAfter(0)
	if len(events) != 0 || eventSnapshot.Sequence != 0 || eventSnapshot.Revision != 1 {
		t.Fatalf("events after rejected batch = %#v, snapshot = %#v", events, eventSnapshot)
	}

	reopened, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	diskValue, err := reopened.GetCellValue("Sheet1", "A1")
	if err != nil || diskValue != "original" {
		t.Fatalf("disk value = %q, error = %v", diskValue, err)
	}
}

func TestConcurrentSameRevisionWritesCommitExactlyOneTransaction(t *testing.T) {
	session, path := newTransactionSession(t, "original")
	type writer struct {
		actor string
		value string
	}
	type result struct {
		writer   writer
		snapshot Snapshot
		err      error
	}
	writers := []writer{
		{actor: "human", value: "human value"},
		{actor: "ai", value: "ai value"},
	}

	start := make(chan struct{})
	results := make(chan result, len(writers))
	var ready sync.WaitGroup
	ready.Add(len(writers))
	for _, candidate := range writers {
		candidate := candidate
		go func() {
			ready.Done()
			<-start
			snapshot, err := session.Apply(1, candidate.actor, []Operation{{
				Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: candidate.value,
			}})
			results <- result{writer: candidate, snapshot: snapshot, err: err}
		}()
	}
	ready.Wait()
	close(start)

	var winner result
	var successes int
	var conflicts int
	for range writers {
		outcome := <-results
		switch {
		case outcome.err == nil:
			successes++
			winner = outcome
			if outcome.snapshot.Revision != 2 {
				t.Fatalf("winning revision = %d, want 2", outcome.snapshot.Revision)
			}
		case errors.Is(outcome.err, ErrRevisionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected writer error: %v", outcome.err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes = %d, conflicts = %d, want one each", successes, conflicts)
	}

	snapshot := session.Snapshot()
	if snapshot.Revision != 2 || snapshot.Sequence != 3 || !snapshot.CanUndo || snapshot.CanRedo {
		t.Fatalf("final snapshot = %#v", snapshot)
	}
	value, err := session.ReadRange("Sheet1", "A1")
	if err != nil || value.Rows[0][0].Value != winner.writer.value {
		t.Fatalf("memory value = %#v, winner = %#v, error = %v", value, winner.writer, err)
	}

	reopened, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	diskValue, err := reopened.GetCellValue("Sheet1", "A1")
	_ = reopened.Close()
	if err != nil || diskValue != winner.writer.value {
		t.Fatalf("disk value = %q, winner = %#v, error = %v", diskValue, winner.writer, err)
	}

	manifest, err := readHistoryManifest(session.historyStore.manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Revision != 2 || len(manifest.Undo) != 1 || len(manifest.Redo) != 0 {
		t.Fatalf("history manifest = %#v", manifest)
	}

	events, eventSnapshot := session.EventsAfter(0)
	if len(events) != 3 || eventSnapshot.Revision != 2 || eventSnapshot.Sequence != 3 {
		t.Fatalf("events = %#v, snapshot = %#v", events, eventSnapshot)
	}
	wantTypes := []string{"presence.update", "cell.typing", "cell.commit"}
	for index, event := range events {
		if event.Sequence != uint64(index+1) || event.Type != wantTypes[index] || event.Actor != winner.writer.actor {
			t.Fatalf("event %d = %#v", index, event)
		}
	}
	if events[0].Revision != 1 || events[1].Revision != 1 || events[2].Revision != 2 || events[2].Text != winner.writer.value {
		t.Fatalf("event revision or value mismatch: %#v", events)
	}
}

func newTransactionSession(t *testing.T, initialValue string) (*Session, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "book.xlsx")
	file := excelize.NewFile()
	if err := file.SetCellValue("Sheet1", "A1", initialValue); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, path
}
