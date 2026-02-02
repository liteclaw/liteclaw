package extensions

import (
	"context"
	"os"
	"testing"

	"github.com/liteclaw/liteclaw/internal/channels"
	"github.com/rs/zerolog"
)

func TestBaseExtension(t *testing.T) {
	ext := NewBaseExtension("test-id", "Test Name", "Test description", "1.0.0")

	if ext.ID() != "test-id" {
		t.Errorf("Expected ID 'test-id', got %q", ext.ID())
	}

	if ext.Name() != "Test Name" {
		t.Errorf("Expected Name 'Test Name', got %q", ext.Name())
	}

	if ext.Description() != "Test description" {
		t.Errorf("Expected Description 'Test description', got %q", ext.Description())
	}

	if ext.Version() != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %q", ext.Version())
	}
}

func TestAPIRegisterChannel(t *testing.T) {
	logger := zerolog.New(os.Stdout)
	api := NewAPI(logger, nil)

	// Create a mock channel
	mockChannel := &mockChannel{name: "test-channel", chanType: "test"}

	api.RegisterChannel(mockChannel)

	// Verify channel was registered
	if len(api.channels) != 1 {
		t.Errorf("Expected 1 channel, got %d", len(api.channels))
	}

	if api.channels[0].Name() != "test-channel" {
		t.Errorf("Expected channel name 'test-channel', got %q", api.channels[0].Name())
	}
}

func TestAPIRegisterTool(t *testing.T) {
	logger := zerolog.New(os.Stdout)
	api := NewAPI(logger, nil)

	api.RegisterTool("test-ext", "test-tool", "A test tool", nil)

	if len(api.tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(api.tools))
	}

	if api.tools[0].Name != "test-tool" {
		t.Errorf("Expected tool name 'test-tool', got %q", api.tools[0].Name)
	}

	if api.tools[0].ExtensionID != "test-ext" {
		t.Errorf("Expected extension ID 'test-ext', got %q", api.tools[0].ExtensionID)
	}
}

func TestManagerRegister(t *testing.T) {
	logger := zerolog.New(os.Stdout)
	api := NewAPI(logger, nil)
	manager := NewManager(logger, api)

	ext := &testExtension{
		BaseExtension: NewBaseExtension("test", "Test Extension", "Test", "1.0.0"),
	}

	if err := manager.Register(ext); err != nil {
		t.Fatalf("Failed to register extension: %v", err)
	}

	// Try to register again - should fail
	if err := manager.Register(ext); err == nil {
		t.Error("Expected error when registering duplicate extension")
	}

	// Check extension is listed
	list := manager.List()
	if len(list) != 1 {
		t.Errorf("Expected 1 extension, got %d", len(list))
	}

	// Get by ID
	got, ok := manager.Get("test")
	if !ok {
		t.Error("Expected to find extension by ID")
	}

	if got.Name() != "Test Extension" {
		t.Errorf("Expected 'Test Extension', got %q", got.Name())
	}
}

func TestManagerUnloadAll(t *testing.T) {
	logger := zerolog.New(os.Stdout)
	api := NewAPI(logger, nil)
	manager := NewManager(logger, api)

	ext := &testExtension{
		BaseExtension: NewBaseExtension("test", "Test Extension", "Test", "1.0.0"),
	}

	_ = manager.Register(ext)

	// Unload all
	if err := manager.UnloadAll(context.Background()); err != nil {
		t.Fatalf("Failed to unload: %v", err)
	}

	// Check no extensions left
	if len(manager.List()) != 0 {
		t.Error("Expected no extensions after unload")
	}

	// Check unregister was called
	if !ext.unregistered {
		t.Error("Expected Unregister to be called")
	}
}

// Test helper implementations

type testExtension struct {
	*BaseExtension
	registered   bool
	unregistered bool
}

func (e *testExtension) Register(api *API) error {
	e.registered = true
	return nil
}

func (e *testExtension) Unregister() error {
	e.unregistered = true
	return nil
}

type mockChannel struct {
	name     string
	chanType string
}

func (c *mockChannel) Name() string                    { return c.name }
func (c *mockChannel) Type() string                    { return c.chanType }
func (c *mockChannel) Start(ctx context.Context) error { return nil }
func (c *mockChannel) Stop(ctx context.Context) error  { return nil }
func (c *mockChannel) IsConnected() bool               { return true }
func (c *mockChannel) SendMessage(ctx context.Context, dest channels.Destination, msg *channels.Message) error {
	return nil
}
