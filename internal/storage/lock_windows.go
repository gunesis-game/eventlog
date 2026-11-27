//go:build windows

package storage

import (
	"fmt"
	"os"
	"syscall"
)

// acquireLock opens the lock file with no sharing, so any other open of
// it fails until the owning handle closes, which the operating system does when
// the owning process exits.
func acquireLock(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("storage lock path %q: %w", path, err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, fmt.Errorf("storage %q is owned by another process: %w", path, err)
	}
	return os.NewFile(uintptr(handle), path), nil
}
