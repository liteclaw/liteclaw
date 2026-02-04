package infra

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathResolution(t *testing.T) {
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tempDir)
	_ = os.Setenv("LITECLAW_STATE_DIR", filepath.Join(tempDir, ".liteclaw"))
	defer func() { _ = os.Setenv("HOME", oldHome) }()
	defer func() { _ = os.Unsetenv("LITECLAW_STATE_DIR") }()

	// Since Paths is a global variable initialized at package load,
	// we might need to call the resolution functions again to verify they work with our mock HOME.

	configDir := resolveConfigDir()
	assert.Contains(t, configDir, ".liteclaw")

	dataDir := resolveDataDir()
	// On macOS: ~/.liteclaw/data, on Linux: ~/.local/share/clawdbot or ~/.liteclaw/data with LITECLAW_STATE_DIR
	assert.True(t, strings.Contains(dataDir, "data") || strings.Contains(dataDir, "clawdbot") || strings.Contains(dataDir, "liteclaw"),
		"dataDir should contain 'data', 'clawdbot', or 'liteclaw': %s", dataDir)
}

func TestEnsureDirs(t *testing.T) {
	tempDir := t.TempDir()

	// Temporarily override Paths for testing
	oldPaths := Paths
	defer func() { Paths = oldPaths }()

	Paths.ConfigDir = tempDir + "/config"
	Paths.DataDir = tempDir + "/data"
	Paths.CacheDir = tempDir + "/cache"
	Paths.LogDir = tempDir + "/log"

	err := EnsureDirs()
	assert.NoError(t, err)

	assert.DirExists(t, Paths.ConfigDir)
	assert.DirExists(t, Paths.DataDir)
	assert.DirExists(t, Paths.CacheDir)
	assert.DirExists(t, Paths.LogDir)
}
