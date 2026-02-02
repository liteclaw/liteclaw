// Package infra provides infrastructure utilities.
package infra

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/liteclaw/liteclaw/internal/config"
)

// Paths holds commonly used paths.
var Paths = struct {
	ConfigDir string
	DataDir   string
	CacheDir  string
	LogDir    string
}{
	ConfigDir: resolveConfigDir(),
	DataDir:   resolveDataDir(),
	CacheDir:  resolveCacheDir(),
	LogDir:    resolveLogDir(),
}

func resolveConfigDir() string {
	// Use config.StateDir() for consistency with TS clawdbot
	return config.StateDir()
}

func resolveDataDir() string {
	stateDir := config.StateDir()

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(stateDir, "data")
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, "Clawdbot", "data")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Clawdbot", "data")
	default:
		xdg := os.Getenv("XDG_DATA_HOME")
		if xdg != "" {
			return filepath.Join(xdg, "clawdbot")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "clawdbot")
	}
}

func resolveCacheDir() string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Caches", "clawdbot")
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, "Clawdbot", "cache")
		}
		return filepath.Join(home, "Clawdbot", "cache")
	default:
		xdg := os.Getenv("XDG_CACHE_HOME")
		if xdg != "" {
			return filepath.Join(xdg, "clawdbot")
		}
		return filepath.Join(home, ".cache", "clawdbot")
	}
}

func resolveLogDir() string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Logs", "clawdbot")
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, "Clawdbot", "logs")
		}
		return filepath.Join(home, "Clawdbot", "logs")
	default:
		return filepath.Join(home, ".local", "state", "clawdbot", "logs")
	}
}

// EnsureDirs creates all required directories.
func EnsureDirs() error {
	dirs := []string{
		Paths.ConfigDir,
		Paths.DataDir,
		Paths.CacheDir,
		Paths.LogDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}
