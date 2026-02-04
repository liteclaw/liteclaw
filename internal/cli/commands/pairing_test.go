package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liteclaw/liteclaw/internal/pairing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPairingCommand(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.Setenv("LITECLAW_STATE_DIR", tempDir)
	defer func() { _ = os.Unsetenv("LITECLAW_STATE_DIR") }()

	channel := "telegram"
	oauthDir := filepath.Join(tempDir, "oauth")
	_ = os.MkdirAll(oauthDir, 0700)
	pairingFile := filepath.Join(oauthDir, channel+"-pairing.json")

	// 1. Pre-populate a pairing request
	request := pairing.PairingRequest{
		ID:        "user1",
		Code:      "ABCDEF12",
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	ctx := struct {
		Version  int                      `json:"version"`
		Requests []pairing.PairingRequest `json:"requests"`
	}{
		Version:  1,
		Requests: []pairing.PairingRequest{request},
	}
	data, _ := json.Marshal(ctx)
	require.NoError(t, os.WriteFile(pairingFile, data, 0600))

	// 2. Test 'pairing list'
	listCmd := newPairingListCommand()
	b := bytes.NewBufferString("")
	listCmd.SetOut(b)
	listCmd.SetArgs([]string{channel})

	err := listCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "ABCDEF12")
	assert.Contains(t, b.String(), "user1")

	// 3. Test 'pairing approve'
	approveCmd := newPairingApproveCommand()
	b.Reset()
	approveCmd.SetOut(b)
	approveCmd.SetArgs([]string{channel, "ABCDEF12"})

	err = approveCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "Approved telegram sender user1")

	// 4. Verify request removed
	requests, _ := pairing.ListChannelPairingRequests(channel)
	assert.Empty(t, requests)
}
