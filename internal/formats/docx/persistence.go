package docx

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func readBounded(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFileBytes {
		return nil, fmt.Errorf("DOCX exceeds 32 MiB limit")
	}
	return data, nil
}

// A stale lock fails closed after a crash. Never automatically steal a writer lock.
func lockFile(filename string) (func(), error) {
	name := filename + ".document-workspace.lock"
	if err := os.Mkdir(name, 0o700); err != nil {
		if os.IsExist(err) {
			return nil, ErrWriterBusy
		}
		return nil, err
	}
	return func() { _ = os.Remove(name) }, nil
}
func prepareFile(ctx context.Context, filename string, content []byte, mode os.FileMode) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(filepath.Dir(filename), ".document-workspace-docx-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create DOCX temporary file: %w", err)
	}
	name := file.Name()
	success := false
	defer func() {
		if !success {
			_ = file.Close()
			_ = os.Remove(name)
		}
	}()
	if err := file.Chmod(mode); err != nil {
		return "", err
	}
	if n, err := file.Write(content); err != nil {
		return "", err
	} else if n != len(content) {
		return "", io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	success = true
	return name, nil
}
func replaceFile(ctx context.Context, filename string, content []byte, check func() error, rename func(string, string) error) error {
	info, err := os.Lstat(filename)
	if err != nil {
		return err
	}
	name, err := prepareFile(ctx, filename, content, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer os.Remove(name)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	if err := rename(name, filename); err != nil {
		return fmt.Errorf("replace DOCX atomically: %w", err)
	}
	return nil
}
