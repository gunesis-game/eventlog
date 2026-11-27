//go:build !windows

package filesystem

import (
	"fmt"
	"os"
	"syscall"
)

// acquireLock takes an exclusive, non-blocking advisory lock that the
// operating system releases when the owning process exits.
func acquireLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open storage lock %q: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("storage %q is owned by another process: %w", path, err)
	}
	return f, nil
}
