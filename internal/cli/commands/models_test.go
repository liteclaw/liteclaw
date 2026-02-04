package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelsCommand(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "liteclaw.json")

	// Pre-create config file with some models
	initialConfig := `{
		"models": {
			"providers": {
				"test-p": {
					"baseUrl": "http://test",
					"models": [
						{"id": "m1", "name": "Model 1"}
					]
				}
			}
		},
		"agents": {
			"defaults": {
				"model": {"primary": "test-p/m1"}
			}
		}
	}`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0644))

	// Isolate from real ~/.liteclaw
	_ = os.Setenv("LITECLAW_CONFIG_PATH", configPath)
	_ = os.Setenv("LITECLAW_STATE_DIR", tempDir)
	defer func() { _ = os.Unsetenv("LITECLAW_CONFIG_PATH") }()
	defer func() { _ = os.Unsetenv("LITECLAW_STATE_DIR") }()

	// 1. Test 'models list'
	listCmd := newModelsListCommand()
	b := bytes.NewBufferString("")
	listCmd.SetOut(b)
	err := listCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "test-p")
	assert.Contains(t, b.String(), "m1")

	// 2. Test 'models status'
	statusCmd := newModelsStatusCommand()
	b.Reset()
	statusCmd.SetOut(b)
	statusCmd.SetArgs([]string{"--plain"})
	err = statusCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "Default Model: test-p/m1")

	// 3. Test 'models add' (Non-interactive via pipe simulated)
	addCmd := newModelsAddCommand()
	b.Reset()
	addCmd.SetOut(b)
	// We need to provide input for the prompts
	// Creating a pipe for stdin
	inR, inW, _ := os.Pipe()
	addCmd.SetIn(inR)

	// Input sequence for add:
	// "Base URL: " -> "http://new\n"
	// "API Key: " -> "key123\n" (non-terminal fallback)
	// "API Type: " -> "openai\n"
	// "Context Window: " -> "1000\n"
	go func() {
		_, _ = inW.Write([]byte("http://new\nkey123\nopenai\n1000\n"))
		_ = inW.Close()
	}()

	addCmd.SetArgs([]string{"new-p/new-m"})
	err = addCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "Successfully added new-p/new-m")

	// Verify config updated
	data, _ := os.ReadFile(configPath)
	assert.Contains(t, string(data), "new-p")
	assert.Contains(t, string(data), "new-m")
}

func TestModelsSetCommand(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "liteclaw.json")
	initialConfig := `{
		"models": {
			"providers": {
				"p1": {"baseUrl": "http://p1.local", "models": [{"id": "m1"}]}
			}
		},
		"agents": {"defaults": {"model": {"primary": "none"}}}
	}`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0644))

	// Isolate from real ~/.liteclaw
	_ = os.Setenv("LITECLAW_CONFIG_PATH", configPath)
	_ = os.Setenv("LITECLAW_STATE_DIR", tempDir)
	defer func() { _ = os.Unsetenv("LITECLAW_CONFIG_PATH") }()
	defer func() { _ = os.Unsetenv("LITECLAW_STATE_DIR") }()

	cmd := newModelsSetCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{"p1/m1"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "Setting default model to: p1/m1")

	// Verify file
	data, _ := os.ReadFile(configPath)
	assert.Contains(t, string(data), `"primary": "p1/m1"`)
}
