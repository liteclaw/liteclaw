package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusCommand_Running(t *testing.T) {
	// Mock gateway
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/status" {
			status := GatewayStatusResponse{
				Status:    "running",
				Version:   "v1.0.0",
				Uptime:    "1h",
				Sessions:  2,
				GoVersion: "go1.21",
				Arch:      "amd64",
				OS:        "linux",
			}
			_ = json.NewEncoder(w).Encode(status)
		}
	}))
	defer server.Close()

	// Parse host/port from mock server
	url := strings.TrimPrefix(server.URL, "http://")
	parts := strings.Split(url, ":")
	host := parts[0]

	cmd := NewStatusCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{"--host", host, "--port", parts[1]})

	err := cmd.Execute()
	assert.NoError(t, err)

	out := b.String()
	assert.Contains(t, out, "✓ Running")
	assert.Contains(t, out, "Version:   v1.0.0")
	assert.Contains(t, out, "Sessions:  2 active")
}

func TestStatusCommand_NotRunning(t *testing.T) {
	cmd := NewStatusCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	// Use a non-existent port
	cmd.SetArgs([]string{"--host", "127.0.0.1", "--port", "65535"})

	err := cmd.Execute()
	assert.NoError(t, err) // Execute itself doesn't error on connection fail, it prints error

	out := b.String()
	assert.Contains(t, out, "✗ Not running")
}

func TestStatusCommand_JSON(t *testing.T) {
	// Mock gateway
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := GatewayStatusResponse{Status: "running", Version: "v1.0.0"}
		_ = json.NewEncoder(w).Encode(status)
	}))
	defer server.Close()

	url := strings.TrimPrefix(server.URL, "http://")
	parts := strings.Split(url, ":")

	cmd := NewStatusCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{"--host", parts[0], "--port", parts[1], "--json"})

	err := cmd.Execute()
	assert.NoError(t, err)

	var resp GatewayStatusResponse
	err = json.Unmarshal(b.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "v1.0.0", resp.Version)
}
