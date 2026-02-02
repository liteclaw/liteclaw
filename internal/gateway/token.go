package gateway

import (
	"os"

	"github.com/liteclaw/liteclaw/internal/config"
)

// LoadClawdbotToken attempts to find the LiteClaw Gateway token.
// It checks:
// 1. LITECLAW_GATEWAY_TOKEN environment variable
// 2. Configuration file via config.Load()
func LoadClawdbotToken() (string, error) {
	// 1. Check environment variables
	if token := os.Getenv("LITECLAW_GATEWAY_TOKEN"); token != "" {
		return token, nil
	}

	// 2. Load from configuration file using the unified config loader
	cfg, err := config.Load()
	if err != nil {
		return "", nil // Ignore errors, token is optional
	}

	return cfg.Gateway.Auth.Token, nil
}
