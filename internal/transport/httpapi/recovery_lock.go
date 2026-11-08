package httpapi

import (
	"fmt"
	"os"
	"path/filepath"
)

func lockRecovery(root string) (func(), error) {
	path := filepath.Join(root, ".store.lock")
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil && (!info.Mode().IsRegular() || !privateRecoveryMode(info.Mode())) {
		return nil, fmt.Errorf("invalid DOCX recovery lock")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := acquireRecoveryLock(file); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("DOCX recovery store is busy: %w", err)
	}
	return func() { _ = file.Close() }, nil
}
