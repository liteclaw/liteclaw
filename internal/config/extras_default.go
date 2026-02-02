package config

import (
	"os"
	"path/filepath"
)

const defaultExtrasJSON = `{
  "logging": {
    "printSystemPrompt": false
  },
  "mcpServers": {
    "brave-search": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-brave-search"
      ],
      "env": {
        "BRAVE_API_KEY": "YOUR_BRAVE_API_KEY"
      }
    },
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem",
        "."
      ]
    },
    "playwright": {
      "command": "npx",
      "args": [
        "-y",
        "@playwright/mcp"
      ]
    }
  }
}
`

// EnsureExtrasFile creates the extras config with defaults if it doesn't exist.
func EnsureExtrasFile() error {
	extrasPath := ExtrasPath()
	if extrasPath == "" {
		return nil
	}
	if _, err := os.Stat(extrasPath); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(extrasPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(extrasPath, []byte(defaultExtrasJSON), 0600)
}
