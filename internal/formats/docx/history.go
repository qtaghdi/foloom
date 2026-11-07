package docx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxHistoryEntries = 10
const maxHistoryBytes = 64 << 20

type historyEntry struct {
	Hash  string `json:"hash"`
	Actor string `json:"actor"`
	Size  int    `json:"size"`
}
type historyManifest struct {
	Version  int            `json:"version"`
	Revision uint64         `json:"revision"`
	Current  string         `json:"current"`
	Undo     []historyEntry `json:"undo"`
	Redo     []historyEntry `json:"redo"`
}
type historyStore struct {
	dir      string
	manifest historyManifest
	warning  string
}

func historyHash(content []byte) string { h := sha256.Sum256(content); return hex.EncodeToString(h[:]) }
func historyPath(path string) string {
	return filepath.Join(filepath.Dir(path), ".document-workspace-docx-history", historyHash([]byte(path)))
}
func validHistoryHash(h string) bool {
	b, e := hex.DecodeString(h)
	return e == nil && len(b) == sha256.Size
}
func historyRead(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("invalid DOCX history file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("DOCX history exceeds limit")
	}
	return b, nil
}
func (h *historyStore) guard(create bool) error {
	for _, dir := range []string{filepath.Dir(h.dir), h.dir} {
		if create {
			if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
				return err
			}
		}
		st, err := os.Lstat(dir)
		if err != nil {
			return err
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("DOCX history directory must not be a symlink")
		}
	}
	return nil
}
func validManifest(m historyManifest) bool {
	if m.Version != 1 || m.Revision < 1 || !validHistoryHash(m.Current) || len(m.Undo)+len(m.Redo) > maxHistoryEntries {
		return false
	}
	total := 0
	for _, entries := range [][]historyEntry{m.Undo, m.Redo} {
		for _, e := range entries {
			if !validHistoryHash(e.Hash) || e.Size < 1 || e.Size > maxFileBytes || (e.Actor != "human" && e.Actor != "ai") {
				return false
			}
			total += e.Size
		}
	}
	return total <= maxHistoryBytes
}
func (h *historyStore) readManifest(name string) (historyManifest, error) {
	b, err := historyRead(filepath.Join(h.dir, name), 64<<10)
	if err != nil {
		return historyManifest{}, err
	}
	var m historyManifest
	if err = json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if !validManifest(m) {
		return m, fmt.Errorf("invalid DOCX history manifest")
	}
	return m, nil
}
func (h *historyStore) atomic(name string, b []byte) error {
	path := filepath.Join(h.dir, name)
	if st, err := os.Lstat(path); err == nil && !st.Mode().IsRegular() {
		return fmt.Errorf("DOCX history destination must be regular")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	tmp, err := prepareFile(context.Background(), path, b, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	return os.Rename(tmp, path)
}
func (h *historyStore) snapshot(e historyEntry) ([]byte, error) {
	if err := h.guard(false); err != nil {
		return nil, err
	}
	b, err := historyRead(filepath.Join(h.dir, e.Hash+".docx"), maxFileBytes)
	if err != nil {
		return nil, err
	}
	if len(b) != e.Size || historyHash(b) != e.Hash {
		return nil, fmt.Errorf("DOCX history snapshot hash mismatch")
	}
	return b, nil
}
func loadHistory(path string, current []byte) (*historyStore, error) {
	h := &historyStore{dir: historyPath(path), manifest: historyManifest{Version: 1, Revision: 1, Current: historyHash(current)}}
	if err := h.guard(false); os.IsNotExist(err) {
		return h, nil
	} else if err != nil {
		return nil, err
	}
	pending, err := h.readManifest("pending.json")
	if err == nil && pending.Current == historyHash(current) {
		if err = os.Rename(filepath.Join(h.dir, "pending.json"), filepath.Join(h.dir, "manifest.json")); err != nil {
			return nil, fmt.Errorf("recover DOCX history: %w", err)
		}
	} else if err == nil || !os.IsNotExist(err) {
		if err = os.Remove(filepath.Join(h.dir, "pending.json")); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	m, err := h.readManifest("manifest.json")
	if os.IsNotExist(err) {
		return h, nil
	}
	if err != nil || m.Current != historyHash(current) {
		h.warning = "저장 이력이 손상되었거나 파일과 일치하지 않아 이전 이력을 사용하지 않습니다."
		return h, nil
	}
	for _, list := range [][]historyEntry{m.Undo, m.Redo} {
		for _, e := range list {
			if _, err := h.snapshot(e); err != nil {
				h.warning = "저장 이력 파일을 검증하지 못해 이전 이력을 사용하지 않습니다."
				return h, nil
			}
		}
	}
	h.manifest = m
	return h, nil
}
func (h *historyStore) prepare(m historyManifest, current []byte) error {
	if !validManifest(m) {
		return fmt.Errorf("invalid DOCX history plan")
	}
	if err := h.guard(true); err != nil {
		return err
	}
	// Snapshot the entire package so unrelated objects and unknown parts survive undo.
	if err := h.atomic(historyHash(current)+".docx", current); err != nil {
		return err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return h.atomic("pending.json", b)
}
func (h *historyStore) commit(m historyManifest) {
	h.manifest = m
	if err := os.Rename(filepath.Join(h.dir, "pending.json"), filepath.Join(h.dir, "manifest.json")); err != nil {
		h.warning = "문서는 저장되었지만 이력 정리가 끝나지 않았습니다. 다시 열 때 확정된 파일의 hash로 복구합니다."
		return
	}
	h.warning = ""
	keep := map[string]bool{}
	for _, list := range [][]historyEntry{m.Undo, m.Redo} {
		for _, e := range list {
			keep[e.Hash+".docx"] = true
		}
	}
	entries, err := os.ReadDir(h.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		n := e.Name()
		if len(n) == 64+5 && validHistoryHash(n[:64]) && n[64:] == ".docx" && !keep[n] {
			_ = os.Remove(filepath.Join(h.dir, n))
		}
	}
}
func trimHistory(m *historyManifest) {
	for {
		total := 0
		for _, list := range [][]historyEntry{m.Undo, m.Redo} {
			for _, e := range list {
				total += e.Size
			}
		}
		if len(m.Undo)+len(m.Redo) <= maxHistoryEntries && total <= maxHistoryBytes {
			return
		}
		if len(m.Undo) > 0 {
			m.Undo = m.Undo[1:]
		} else {
			m.Redo = m.Redo[1:]
		}
	}
}
func (s *Session) persistHistory(ctx context.Context, content []byte, pkg *documentPackage, m historyManifest, actor string, textOnly bool) (Snapshot, error) {
	unlock, err := lockFile(s.path)
	if err != nil {
		return Snapshot{}, err
	}
	defer unlock()
	if err = s.checkSource(); err != nil {
		return Snapshot{}, err
	}
	if err = s.history.prepare(m, s.content); err != nil {
		return Snapshot{}, fmt.Errorf("prepare DOCX history: %w", err)
	}
	if err = replaceFile(ctx, s.path, content, s.checkSource, s.rename); err != nil {
		_ = os.Remove(filepath.Join(s.history.dir, "pending.json"))
		return Snapshot{}, err
	}
	s.content, s.pkg = content, pkg
	s.revision++
	s.sequence++
	s.history.commit(m)
	s.events = append(s.events, Event{Sequence: s.sequence, Revision: s.revision, Actor: actor, Type: "docx.committed", TextOnly: textOnly})
	if len(s.events) > 128 {
		s.events = s.events[len(s.events)-128:]
	}
	return s.snapshotLocked()
}

// RestoreHistory uses the current revision and restores one complete accepted
// batch. Human and AI writes share one chronological stack, never separate files.
func (s *Session) RestoreHistory(ctx context.Context, input HistoryInput, actor string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if input.SessionID != s.id || input.BaseRevision != s.revision {
		return Snapshot{}, ErrRevisionConflict
	}
	if actor != "ai" && actor != "human" {
		return Snapshot{}, fmt.Errorf("DOCX actor must be human or ai")
	}
	if input.Direction != "undo" && input.Direction != "redo" {
		return Snapshot{}, fmt.Errorf("DOCX history direction must be undo or redo")
	}
	m := s.history.manifest
	m.Undo = append([]historyEntry(nil), m.Undo...)
	m.Redo = append([]historyEntry(nil), m.Redo...)
	from, to := &m.Undo, &m.Redo
	if input.Direction == "redo" {
		from, to = &m.Redo, &m.Undo
	}
	if len(*from) == 0 {
		return Snapshot{}, fmt.Errorf("DOCX history is empty")
	}
	entry := (*from)[len(*from)-1]
	content, err := s.history.snapshot(entry)
	if err != nil {
		return Snapshot{}, err
	}
	pkg, err := loadPackage(content)
	if err != nil {
		return Snapshot{}, err
	}
	if _, _, _, err = inspect(pkg.document, s.id, s.revision+1); err != nil {
		return Snapshot{}, err
	}
	*from = (*from)[:len(*from)-1]
	*to = append(*to, historyEntry{Hash: historyHash(s.content), Size: len(s.content), Actor: entry.Actor})
	m.Revision = s.revision + 1
	m.Current = historyHash(content)
	trimHistory(&m)
	return s.persistHistory(ctx, content, pkg, m, actor, false)
}
