//go:build darwin || linux

package httpapi

import (
	"golang.org/x/sys/unix"
	"os"
)

// The kernel releases this lock after a crash. The stable lock file is never
// unlinked, because another process may already hold or be waiting on its inode.
func acquireRecoveryLock(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}
