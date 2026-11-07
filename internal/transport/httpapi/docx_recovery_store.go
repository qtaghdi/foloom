package httpapi

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	recoveryLifetime = 7 * 24 * time.Hour
	maxRecoveryCount = 3
	maxRecoveryBytes = 32 << 20
)

var recoveryID = regexp.MustCompile(`^[a-f0-9]{32}$`)

type recoveryEntry struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updatedAt"`
	Damaged   bool      `json:"damaged,omitempty"`
	HasDraft  bool      `json:"hasDraft"`
	Hash      [32]byte  `json:"hash"`
}

func (s *Server) recoveryRoot(create bool) (string, error) {
	path := s.recoveryDir
	if path == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(config, "document-workspace", "docx-recovery")
	}
	if create {
		if err := os.MkdirAll(path, 0700); err != nil {
			return "", err
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || !privateRecoveryMode(info.Mode()) {
		return "", fmt.Errorf("DOCX recovery directory must be private")
	}
	return path, nil
}

// Windows uses the current user's configuration directory ACL rather than
// POSIX mode bits, which Go does not expose there.
func privateRecoveryMode(mode os.FileMode) bool {
	return runtime.GOOS == "windows" || mode.Perm()&0077 == 0
}

func recoveryPath(root, id string) string { return filepath.Join(root, id+".recovery") }

// Only fixed-size metadata is read for listings. Reopening additionally verifies
// the exact OOXML bytes so a partial or damaged record cannot become a new tab.
func readRecovery(root, id string, content bool) (recoveryEntry, []byte, error) {
	var entry recoveryEntry
	if !recoveryID.MatchString(id) {
		return entry, nil, os.ErrNotExist
	}
	path := recoveryPath(root, id)
	info, err := os.Lstat(path)
	if err != nil {
		return entry, nil, err
	}
	if !info.Mode().IsRegular() || !privateRecoveryMode(info.Mode()) || info.Size() > maxRecoveryBytes+1024 {
		return entry, nil, fmt.Errorf("invalid DOCX recovery record")
	}
	file, err := os.Open(path)
	if err != nil {
		return entry, nil, err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 1024)
	header, err := reader.ReadSlice('\n')
	if err != nil || json.Unmarshal(header, &entry) != nil || entry.ID != id || entry.UpdatedAt.IsZero() {
		return recoveryEntry{}, nil, fmt.Errorf("invalid DOCX recovery metadata")
	}
	if name, err := docxName(entry.Name); err != nil || name != entry.Name {
		return recoveryEntry{}, nil, fmt.Errorf("invalid DOCX recovery filename")
	}
	if !content {
		return entry, nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxRecoveryBytes+1))
	if err != nil || len(data) > maxRecoveryBytes || sha256.Sum256(data) != entry.Hash {
		return recoveryEntry{}, nil, fmt.Errorf("invalid DOCX recovery content")
	}
	return entry, data, nil
}

// Callers serialize scan/write/delete with the workspace open lock. Expired
// records are removed on access and never offered for recovery.
func scanRecovery(root string, now time.Time) ([]recoveryEntry, int, error) {
	entries := make([]recoveryEntry, 0, maxRecoveryCount)
	items, err := os.ReadDir(root)
	if err != nil {
		return nil, 0, err
	}
	count := 0
	for _, item := range items {
		id := strings.TrimSuffix(item.Name(), ".recovery")
		if id == item.Name() || !recoveryID.MatchString(id) {
			if strings.HasPrefix(item.Name(), ".checkpoint-") {
				info, err := item.Info()
				if err == nil && info.Mode().IsRegular() && now.Sub(info.ModTime()) >= recoveryLifetime {
					if err := os.Remove(filepath.Join(root, item.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
						return nil, 0, err
					}
				}
			}
			continue
		}
		entry, _, err := readRecovery(root, id, false)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		info, statErr := item.Info()
		if statErr != nil {
			return nil, 0, statErr
		}
		if now.Sub(info.ModTime()) >= recoveryLifetime || err == nil && now.Sub(entry.UpdatedAt) >= recoveryLifetime {
			if err := os.Remove(recoveryPath(root, id)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, 0, err
			}
			continue
		}
		count++
		if err != nil {
			entry = recoveryEntry{ID: id, Name: "손상된 복구 사본.docx", UpdatedAt: info.ModTime().UTC(), Damaged: true}
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].UpdatedAt.After(entries[j].UpdatedAt) })
	return entries, count, nil
}

func writeRecovery(root string, entry recoveryEntry, content []byte) error {
	if len(content) > maxRecoveryBytes || !recoveryID.MatchString(entry.ID) {
		return fmt.Errorf("DOCX recovery size or identity is invalid")
	}
	entry.Hash = sha256.Sum256(content)
	header, err := json.Marshal(entry)
	if err != nil || len(header) >= 1024 {
		return fmt.Errorf("DOCX recovery metadata is too large")
	}
	file, err := os.CreateTemp(root, ".checkpoint-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(append(header, '\n')); err == nil {
		_, err = file.Write(content)
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(file.Name(), recoveryPath(root, entry.ID))
}

func recoveryName(entry recoveryEntry, attempt int) string {
	extension := filepath.Ext(entry.Name)
	stem := strings.TrimSuffix(entry.Name, extension)
	suffix := fmt.Sprintf(" (복구 %s %d).docx", hex.EncodeToString(entry.Hash[:2]), attempt)
	for len(stem)+len(suffix) > 240 {
		chars := []rune(stem)
		stem = string(chars[:len(chars)-1])
	}
	return stem + suffix
}
