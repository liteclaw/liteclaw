package pairing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liteclaw/liteclaw/internal/config"
)

const (
	PairingCodeLength   = 8
	PairingCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	PairingPendingTTL   = 60 * time.Minute
	PairingPendingMax   = 3
	PairingVersion      = 1
	DefaultOAuthDir     = "oauth"
)

type PairingRequest struct {
	ID         string            `json:"id"`
	Code       string            `json:"code"`
	CreatedAt  string            `json:"createdAt"`
	LastSeenAt string            `json:"lastSeenAt"`
	Meta       map[string]string `json:"meta,omitempty"`
}

type PairingStoreCtx struct {
	Version  int              `json:"version"`
	Requests []PairingRequest `json:"requests"`
}

type AllowFromStoreCtx struct {
	Version   int      `json:"version"`
	AllowFrom []string `json:"allowFrom"`
}

// GetOAuthDir returns the directory where pairing/auth files are stored
// Path: ~/.liteclaw/oauth/
func GetOAuthDir() string {
	return filepath.Join(config.StateDir(), DefaultOAuthDir)
}

func getPairingPath(channel string) string {
	safeKey := safeChannelKey(channel)
	return filepath.Join(GetOAuthDir(), safeKey+"-pairing.json")
}

func getAllowFromPath(channel string) string {
	safeKey := safeChannelKey(channel)
	return filepath.Join(GetOAuthDir(), safeKey+"-allowFrom.json")
}

func safeChannelKey(channel string) string {
	raw := strings.TrimSpace(strings.ToLower(channel))
	safe := strings.ReplaceAll(raw, string(filepath.Separator), "_")
	safe = strings.ReplaceAll(safe, "..", "_")
	return safe
}

// withFileLock is implemented in platform-specific files:
// - store_unix.go (Linux, macOS)
// - store_windows.go (Windows)

func readJSON[T any](path string, fallback T) (T, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fallback, false, nil
	}
	if err != nil {
		return fallback, false, err
	}

	var val T
	if err := json.Unmarshal(data, &val); err != nil {
		return fallback, true, nil // corrupt file, return fallback
	}
	return val, true, nil
}

func writeJSON(path string, data interface{}) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	// Write atomic
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Helper to update store logic
func updatePairingStore(channel string, updateFn func(*PairingStoreCtx) (bool, error)) error {
	path := getPairingPath(channel)
	return withFileLock(path, func() error {
		store, _, err := readJSON(path, PairingStoreCtx{Version: PairingVersion, Requests: []PairingRequest{}})
		if err != nil {
			return err
		}

		changed, err := updateFn(&store)
		if err != nil {
			return err
		}
		if changed {
			return writeJSON(path, store)
		}
		return nil
	})
}

// Helper to update allow store logic
func updateAllowFromStore(channel string, updateFn func(*AllowFromStoreCtx) (bool, error)) error {
	path := getAllowFromPath(channel)
	return withFileLock(path, func() error {
		store, _, err := readJSON(path, AllowFromStoreCtx{Version: PairingVersion, AllowFrom: []string{}})
		if err != nil {
			return err
		}

		changed, err := updateFn(&store)
		if err != nil {
			return err
		}
		if changed {
			return writeJSON(path, store)
		}
		return nil
	})
}
