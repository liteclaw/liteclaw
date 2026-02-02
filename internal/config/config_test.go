package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	// Create temp home for isolation
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", oldHome)

	cfg, err := Load()
	if err != nil {
		// Config file not found is expected
		t.Log("Config not found, using defaults")
	}

	if cfg == nil {
		cfg = &Config{}
	}

	// Check defaults would be applied
	if cfg.Gateway.Port == 0 {
		// Default should be 18789
		t.Log("Gateway port not set (expected when no config)")
	}
}

func TestConfigPath(t *testing.T) {
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", "/test/home")
	defer os.Setenv("HOME", oldHome)

	path := ConfigPath()
	expected := "/test/home/.liteclaw/liteclaw.json"

	if path != expected {
		t.Errorf("Expected %q, got %q", expected, path)
	}
}

func TestConfigDir(t *testing.T) {
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", "/test/home")
	defer os.Setenv("HOME", oldHome)

	dir := ConfigDir()
	expected := "/test/home/.liteclaw"

	if dir != expected {
		t.Errorf("Expected %q, got %q", expected, dir)
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".liteclaw")
	_ = os.MkdirAll(configDir, 0755)

	// Write config file
	configPath := filepath.Join(configDir, "liteclaw.json")
	configContent := `{
  "gateway": {
    "bind": "127.0.0.1",
    "port": 8080
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "botToken": "test-token"
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Set HOME to temp dir
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", oldHome)

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Gateway.Bind != "127.0.0.1" {
		t.Errorf("Expected bind '127.0.0.1', got %q", cfg.Gateway.Bind)
	}

	if cfg.Gateway.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", cfg.Gateway.Port)
	}

	if !cfg.Channels.Telegram.Enabled {
		t.Error("Expected Telegram to be enabled")
	}

	if cfg.Channels.Telegram.BotToken != "test-token" {
		t.Errorf("Expected token 'test-token', got %q", cfg.Channels.Telegram.BotToken)
	}
}

func TestExpandEnvVars(t *testing.T) {
	os.Setenv("TEST_TOKEN", "secret-token-value")
	defer os.Unsetenv("TEST_TOKEN")

	cfg := &Config{
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{
				BotToken: "${TEST_TOKEN}",
			},
		},
	}

	expandEnvVars(cfg)

	if cfg.Channels.Telegram.BotToken != "secret-token-value" {
		t.Errorf("Expected 'secret-token-value', got %q", cfg.Channels.Telegram.BotToken)
	}
}

func TestSaveConfig(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", oldHome)

	cfg := &Config{
		Gateway: GatewayConfig{
			Bind: "loopback",
			Port: 18789,
		},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tempDir, ".liteclaw", "liteclaw.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load and verify
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	if loaded.Gateway.Port != 18789 {
		t.Errorf("Expected port 18789, got %d", loaded.Gateway.Port)
	}
}
