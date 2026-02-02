package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelsStatusCommand_Gateway(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/status" {
			status := map[string]interface{}{
				"status": "running",
				"uptime": "123s",
				"channels": []map[string]interface{}{
					{"name": "telegram", "type": "telegram", "connected": true, "sessions": 5},
				},
			}
			_ = json.NewEncoder(w).Encode(status)
		}
	}))
	defer server.Close()

	url := strings.TrimPrefix(server.URL, "http://")
	parts := strings.Split(url, ":")

	// Isolate from real ~/.liteclaw
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "liteclaw.json")
	initialConfig := `{"gateway": {"port": ` + parts[1] + `}}`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0644))

	os.Setenv("LITECLAW_CONFIG_PATH", configPath)
	os.Setenv("LITECLAW_STATE_DIR", tempDir)
	defer os.Unsetenv("LITECLAW_CONFIG_PATH")
	defer os.Unsetenv("LITECLAW_STATE_DIR")

	cmd := newChannelsStatusCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	err := cmd.Execute()
	assert.NoError(t, err)

	out := b.String()
	assert.Contains(t, out, "Gateway: RUNNING")
	assert.Contains(t, out, "telegram")
	assert.Contains(t, out, "Online")
}

func TestChannelsStatusCommand_Fallback(t *testing.T) {
	// Isolate from real ~/.liteclaw - use port that won't have gateway
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "liteclaw.json")
	initialConfig := `{"gateway": {"port": 65535}, "channels": {"telegram": {"enabled": true, "botToken": "token123"}}}`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0644))

	os.Setenv("LITECLAW_CONFIG_PATH", configPath)
	os.Setenv("LITECLAW_STATE_DIR", tempDir)
	defer os.Unsetenv("LITECLAW_CONFIG_PATH")
	defer os.Unsetenv("LITECLAW_STATE_DIR")

	cmd := newChannelsStatusCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	err := cmd.Execute()
	assert.NoError(t, err)

	out := b.String()
	assert.Contains(t, out, "Gateway not reachable")
	assert.Contains(t, out, "telegram")
	assert.Contains(t, out, "true")
}
