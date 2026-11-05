package docx

import (
	"context"
	"crypto/sha256"
)

// SavedFile exposes committed bytes only. A download does not reserialize OOXML
// or change the revision, and an externally replaced source is never returned.
func (s *Session) SavedFile(ctx context.Context, sessionID string, revision uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sessionID != s.id || revision != s.revision {
		return nil, ErrRevisionConflict
	}
	content, err := readFile(s.path)
	if err != nil || sha256.Sum256(content) != sha256.Sum256(s.content) {
		return nil, ErrExternalChange
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return content, nil
}
