package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageSendCommand_Errors(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "liteclaw.json")

	// Pre-create config with telegram disabled
	initialConfig := `{"channels": {"telegram": {"enabled": false}}}`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0644))

	_ = os.Setenv("LITECLAW_CONFIG_PATH", configPath)
	defer func() { _ = os.Unsetenv("LITECLAW_CONFIG_PATH") }()

	cmd := newMessageSendCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	// 1. Missing target
	cmd.SetArgs([]string{"--channel", "telegram", "--message", "hello"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "target is required")

	// 2. Telegram disabled
	cmd.SetArgs([]string{"--channel", "telegram", "--target", "123", "--message", "hello"})
	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "telegram not enabled")

	// 3. Unsupported channel
	cmd.SetArgs([]string{"--channel", "unknown", "--target", "123", "--message", "hello"})
	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channel 'unknown' not supported")
}
