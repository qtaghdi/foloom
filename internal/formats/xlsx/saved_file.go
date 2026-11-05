package xlsx

import (
	"context"
	"fmt"
	"io"
	"os"
)

// SavedFile returns the exact committed package, never a reserialized preview.
// The revision and source hash are checked under the session lock. No history,
// events or document bytes are changed by save confirmation or download.
func (s *Session) SavedFile(ctx context.Context, sessionID string, revision uint64) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sessionID != s.id || revision != s.revision {
		return nil, ErrRevisionConflict
	}
	info, err := os.Lstat(s.path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxWorkbookBytes {
		return nil, ErrExternalChange
	}
	file, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("read saved workbook: %w", err)
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxWorkbookBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read saved workbook: %w", err)
	}
	if len(content) > maxWorkbookBytes || hashBytes(content) != s.currentHash {
		return nil, ErrExternalChange
	}
	manifest, err := readHistoryManifest(s.historyStore.manifestPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("verify saved workbook revision: %w", err)
	}
	if err == nil && manifest.CurrentHash == s.currentHash && manifest.Revision != s.revision {
		return nil, ErrRevisionConflict
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return content, nil
}
