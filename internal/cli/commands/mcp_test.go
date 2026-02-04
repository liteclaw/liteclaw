package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMCPCommand(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "liteclaw.extras.json")

	_ = os.Setenv("LITECLAW_MCP_CONFIG_PATH", configPath)
	defer func() { _ = os.Unsetenv("LITECLAW_MCP_CONFIG_PATH") }()

	// 1. Test 'mcp install'
	installCmd := newMCPInstallCommand()
	b := bytes.NewBufferString("")
	installCmd.SetOut(b)
	installCmd.SetArgs([]string{"@modelcontextprotocol/server-filesystem", "--args", "/tmp"})

	err := installCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "Successfully installed")
	assert.Contains(t, b.String(), "server ID 'filesystem'")

	// 2. Test 'mcp list'
	listCmd := newMCPListCommand()
	b.Reset()
	listCmd.SetOut(b)
	err = listCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "filesystem")
	assert.Contains(t, b.String(), "Command: npx")
	assert.Contains(t, b.String(), "/tmp")
}
