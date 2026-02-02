//go:build !windows

package pairing

import (
	"os"
	"path/filepath"
	"syscall"
)

// withFileLock provides exclusive file locking on Unix systems
func withFileLock(path string, fn func() error) error {
	// Ensure dir exists
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	// Create/Open file for locking
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Exclusive lock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	return fn()
}
