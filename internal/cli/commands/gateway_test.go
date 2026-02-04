package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGatewayCommand_StartForeground(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "liteclaw.json")
	require.NoError(t, os.WriteFile(configPath, []byte("{}"), 0644))

	_ = os.Setenv("LITECLAW_CONFIG_PATH", configPath)
	defer func() { _ = os.Unsetenv("LITECLAW_CONFIG_PATH") }()
	// Mock config dir for lock file
	_ = os.Setenv("LITECLAW_STATE_DIR", tempDir)
	defer func() { _ = os.Unsetenv("LITECLAW_STATE_DIR") }()

	_ = os.Setenv("LITECLAW_SKIP_GATEWAY_START", "true")
	defer func() { _ = os.Unsetenv("LITECLAW_SKIP_GATEWAY_START") }()

	cmd := newGatewayStartCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "Starting LiteClaw Gateway")
}

func TestGatewayCommand_Restart(t *testing.T) {
	// Isolate from real ~/.liteclaw
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "liteclaw.json")
	_ = os.WriteFile(configPath, []byte(`{"gateway":{"port":18789}}`), 0644)

	_ = os.Setenv("LITECLAW_CONFIG_PATH", configPath)
	_ = os.Setenv("LITECLAW_STATE_DIR", tempDir)
	_ = os.Setenv("LITECLAW_SKIP_GATEWAY_START", "true")
	defer func() { _ = os.Unsetenv("LITECLAW_CONFIG_PATH") }()
	defer func() { _ = os.Unsetenv("LITECLAW_STATE_DIR") }()
	defer func() { _ = os.Unsetenv("LITECLAW_SKIP_GATEWAY_START") }()

	cmd := newGatewayRestartCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "Restarting gateway server")
}
