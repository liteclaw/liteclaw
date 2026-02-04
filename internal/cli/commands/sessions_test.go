package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionsListCommand(t *testing.T) {
	tempDir := t.TempDir()
	sessionsFile := filepath.Join(tempDir, "sessions.json")

	// Pre-create sessions.json
	now := time.Now().UnixMilli()
	sessions := map[string]interface{}{
		"telegram:123": map[string]interface{}{
			"sessionId": "abc-123",
			"key":       "telegram:123",
			"channel":   "telegram",
			"updatedAt": now,
		},
		"discord:456": map[string]interface{}{
			"sessionId": "def-456",
			"key":       "discord:456",
			"channel":   "discord",
			"updatedAt": now - 100000,
		},
	}
	data, _ := json.Marshal(sessions)
	require.NoError(t, os.WriteFile(sessionsFile, data, 0644))

	// Mock env
	_ = os.Setenv("LITECLAW_DATA_DIR", tempDir)
	defer func() { _ = os.Unsetenv("LITECLAW_DATA_DIR") }()

	cmd := newSessionsListCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	err := cmd.Execute()
	assert.NoError(t, err)

	out := b.String()
	assert.Contains(t, out, "telegram:123")
	assert.Contains(t, out, "discord:456")
	assert.Contains(t, out, "just now")
}

func TestSessionsListCommand_Empty(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.Setenv("LITECLAW_DATA_DIR", tempDir)
	defer func() { _ = os.Unsetenv("LITECLAW_DATA_DIR") }()

	cmd := newSessionsListCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "No sessions found.")
}
