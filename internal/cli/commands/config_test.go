package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigCommand(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "liteclaw.json")

	// Pre-create config file
	initialConfig := `{"gateway": {"port": 1234}}`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0644))

	// Isolate from real ~/.liteclaw
	os.Setenv("LITECLAW_CONFIG_PATH", configPath)
	os.Setenv("LITECLAW_STATE_DIR", tempDir)
	defer os.Unsetenv("LITECLAW_CONFIG_PATH")
	defer os.Unsetenv("LITECLAW_STATE_DIR")

	// 1. Test 'config get'
	getCmd := newConfigGetCommand()
	b := bytes.NewBufferString("")
	getCmd.SetOut(b)
	getCmd.SetArgs([]string{"gateway.port"})

	err := getCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "1234")

	// 2. Test 'config set'
	setCmd := newConfigSetCommand()
	b.Reset()
	setCmd.SetOut(b)
	setCmd.SetArgs([]string{"gateway.port", "5678"})

	err = setCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "Updated gateway.port = 5678")

	// 3. Verify file was written (viper may write in different format)
	data, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	// Check that the port value is updated (viper may format differently)
	assert.Contains(t, string(data), "5678")
}
