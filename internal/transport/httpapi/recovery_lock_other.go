//go:build !darwin && !linux && !windows

package httpapi

import (
	"fmt"
	"os"
)

func acquireRecoveryLock(file *os.File) error {
	return fmt.Errorf("DOCX recovery file locking is unavailable on this OS")
}
