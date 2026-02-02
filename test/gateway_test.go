// Package test provides integration and e2e tests for LiteClaw.
package test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/liteclaw/liteclaw/internal/channels"
	testhelpers "github.com/liteclaw/liteclaw/test/helpers"
)

// TestGatewayE2E tests the gateway server end-to-end.
func TestGatewayE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Create isolated test environment
	tempHome := testhelpers.NewTempHome(t)
	defer tempHome.Cleanup()

	// Write test config
	tempHome.WriteConfig(t, `
gateway:
  host: "127.0.0.1"
  port: 0
logging:
  level: debug
`)

	// TODO: Start gateway and test endpoints
	// This requires starting the gateway in a goroutine
}

// TestMockServerHelper tests the mock server helper.
func TestMockServerHelper(t *testing.T) {
	server := testhelpers.NewMockServer()
	defer server.Close()

	// Register a handler
	server.HandleJSON("GET", "/api/test", http.StatusOK, `{"status": "ok"}`)

	// Make request
	resp, err := http.Get(server.URL + "/api/test")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check recorded requests
	requests := server.Requests()
	if len(requests) != 1 {
		t.Errorf("Expected 1 request, got %d", len(requests))
	}

	if requests[0].Method != "GET" {
		t.Errorf("Expected GET, got %s", requests[0].Method)
	}

	if requests[0].Path != "/api/test" {
		t.Errorf("Expected /api/test, got %s", requests[0].Path)
	}
}

// TestMockChannelHelper tests the mock channel helper.
func TestMockChannelHelper(t *testing.T) {
	channel := testhelpers.NewMockChannel("test", "mock")

	// Start channel
	ctx := context.Background()
	if err := channel.Start(ctx); err != nil {
		t.Fatalf("Failed to start channel: %v", err)
	}

	if !channel.IsConnected() {
		t.Error("Channel should be connected")
	}

	// Send message using channels types
	err := channel.SendMessage(ctx, channels.Destination{
		ChannelType: "mock",
		ChatID:      "123",
	}, &channels.Message{Text: "Hello!"})
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Verify message was recorded
	messages := channel.SentMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	channel.AssertMessageSent(t, "Hello!")

	// Stop channel
	if err := channel.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop channel: %v", err)
	}

	if channel.IsConnected() {
		t.Error("Channel should not be connected")
	}
}

// TestTempHomeHelper tests the temp home helper.
func TestTempHomeHelper(t *testing.T) {
	tempHome := testhelpers.NewTempHome(t)
	defer tempHome.Cleanup()

	// Check config dir exists
	configDir := tempHome.ConfigDir()
	if configDir == "" {
		t.Error("Config dir should not be empty")
	}

	// Write a config file
	configPath := tempHome.WriteConfig(t, "test: value")
	if configPath == "" {
		t.Error("Config path should not be empty")
	}

	// Create a file
	filePath := tempHome.CreateFile(t, "test/file.txt", "content")
	if filePath == "" {
		t.Error("File path should not be empty")
	}
}

// TestGetFreePort tests the GetFreePort helper.
func TestGetFreePort(t *testing.T) {
	port := testhelpers.GetFreePort(t)
	if port <= 0 {
		t.Errorf("Expected positive port, got %d", port)
	}

	// Second call should return different port
	port2 := testhelpers.GetFreePort(t)
	if port == port2 {
		t.Log("Ports might be the same due to reuse, which is acceptable")
	}
}

// TestPollHelper tests the Poll helper.
func TestPollHelper(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	count := 0
	result := testhelpers.Poll(t, ctx, 50*time.Millisecond, func() bool {
		count++
		return count >= 3
	})

	if !result {
		t.Error("Poll should have succeeded")
	}

	if count < 3 {
		t.Errorf("Expected at least 3 polls, got %d", count)
	}
}
