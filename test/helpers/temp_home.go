// Package test provides test utilities and helpers for LiteClaw tests.
package test

import (
	"os"
	"path/filepath"
	"testing"
)

// TempHome creates a temporary home directory for isolated tests.
type TempHome struct {
	Dir      string
	Original string
	restore  map[string]string
}

// NewTempHome creates a new temporary home directory and sets HOME.
func NewTempHome(t *testing.T) *TempHome {
	t.Helper()

	dir := t.TempDir()

	th := &TempHome{
		Dir:      dir,
		Original: os.Getenv("HOME"),
		restore:  make(map[string]string),
	}

	// Save and set environment variables
	envVars := []string{
		"HOME",
		"XDG_CONFIG_HOME",
		"XDG_DATA_HOME",
		"XDG_STATE_HOME",
		"XDG_CACHE_HOME",
		"LITECLAW_CONFIG_PATH",
		"LITECLAW_STATE_DIR",
	}

	for _, key := range envVars {
		th.restore[key] = os.Getenv(key)
	}

	_ = os.Setenv("HOME", dir)
	_ = os.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	_ = os.Setenv("XDG_DATA_HOME", filepath.Join(dir, ".local", "share"))
	_ = os.Setenv("XDG_STATE_HOME", filepath.Join(dir, ".local", "state"))
	_ = os.Setenv("XDG_CACHE_HOME", filepath.Join(dir, ".cache"))
	_ = os.Unsetenv("LITECLAW_CONFIG_PATH")
	_ = os.Unsetenv("LITECLAW_STATE_DIR")

	// Create standard directories
	_ = os.MkdirAll(filepath.Join(dir, ".config", "liteclaw"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, ".liteclaw"), 0755)

	return th
}

// Cleanup restores the original environment.
func (th *TempHome) Cleanup() {
	for key, value := range th.restore {
		if value == "" {
			_ = os.Unsetenv(key)
		} else {
			_ = os.Setenv(key, value)
		}
	}
}

// ConfigDir returns the LiteClaw config directory in the temp home.
func (th *TempHome) ConfigDir() string {
	return filepath.Join(th.Dir, ".liteclaw")
}

// WriteConfig writes a config file to the temp home.
func (th *TempHome) WriteConfig(t *testing.T, content string) string {
	t.Helper()

	configPath := filepath.Join(th.ConfigDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	return configPath
}

// CreateFile creates a file in the temp home.
func (th *TempHome) CreateFile(t *testing.T, relPath, content string) string {
	t.Helper()

	fullPath := filepath.Join(th.Dir, relPath)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	return fullPath
}
