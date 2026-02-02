//go:build windows

package pairing

import (
	"os"
	"path/filepath"
)

// withFileLock on Windows - simplified implementation without flock
// Windows uses different file locking mechanisms, but for this use case
// (local pairing files), we skip locking and just ensure directory exists
func withFileLock(path string, fn func() error) error {
	// Ensure dir exists
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	// On Windows, we proceed without file locking
	// This is acceptable for local pairing operations
	return fn()
}
