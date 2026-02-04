package pairing

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) string {
	tempDir := t.TempDir()
	_ = os.Setenv("LITECLAW_STATE_DIR", tempDir)
	return tempDir
}

func TestPairingFlow(t *testing.T) {
	setupTest(t)
	channel := "test-channel"
	userID := "user-123"

	// 1. Initially NOT allowed
	allowed, err := IsAllowed(channel, userID)
	require.NoError(t, err)
	assert.False(t, allowed)

	// 2. Upsert request
	code, created, err := UpsertChannelPairingRequest(channel, userID, map[string]string{"name": "Test User"})
	require.NoError(t, err)
	assert.True(t, created)
	assert.NotEmpty(t, code)
	assert.Equal(t, 8, len(code))

	// 3. List requests
	reqs, err := ListChannelPairingRequests(channel)
	require.NoError(t, err)
	assert.Len(t, reqs, 1)
	assert.Equal(t, userID, reqs[0].ID)
	assert.Equal(t, code, reqs[0].Code)

	// 4. Approve code
	approved, err := ApproveChannelPairingCode(channel, code)
	require.NoError(t, err)
	assert.NotNil(t, approved)
	assert.Equal(t, userID, approved.ID)

	// 5. Now it MUST be allowed
	allowed, err = IsAllowed(channel, userID)
	require.NoError(t, err)
	assert.True(t, allowed)

	// 6. Request list should be empty
	reqs, err = ListChannelPairingRequests(channel)
	require.NoError(t, err)
	assert.Len(t, reqs, 0)
}

func TestPairingPruning(t *testing.T) {
	setupTest(t)
	channel := "pruning-channel"

	// 1. Max pending is 3 (defined in store.go)
	for i := 0; i < 5; i++ {
		userID := fmt.Sprintf("user-%d", i)
		code, created, err := UpsertChannelPairingRequest(channel, userID, nil)
		require.NoError(t, err)
		if i < 3 {
			assert.True(t, created)
			assert.NotEmpty(t, code)
		} else {
			assert.False(t, created)
			assert.Empty(t, code)
		}
	}

	// 2. Should only have the first 3 requests
	reqs, err := ListChannelPairingRequests(channel)
	require.NoError(t, err)
	assert.Len(t, reqs, 3)
	assert.Equal(t, "user-0", reqs[0].ID)
	assert.Equal(t, "user-1", reqs[1].ID)
	assert.Equal(t, "user-2", reqs[2].ID)
}

func TestPairingExpiration(t *testing.T) {
	setupTest(t)
	channel := "exp-channel"

	// Ensure directory exists
	path := getPairingPath(channel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))

	// Mock an old request by writing to file directly
	oldTime := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	store := PairingStoreCtx{
		Version: PairingVersion,
		Requests: []PairingRequest{
			{ID: "old-user", Code: "OLDCODE", CreatedAt: oldTime, LastSeenAt: oldTime},
		},
	}
	require.NoError(t, writeJSON(path, store))

	// Pruning should remove it
	reqs, err := ListChannelPairingRequests(channel)
	require.NoError(t, err)
	assert.Len(t, reqs, 0)
}

func TestAddChannelAllow(t *testing.T) {
	setupTest(t)
	channel := "allow-channel"
	userID := "manual-user"

	err := AddChannelAllow(channel, userID)
	require.NoError(t, err)

	allowed, err := IsAllowed(channel, userID)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestGenerateUniqueCode(t *testing.T) {
	existing := map[string]bool{
		"CODE1": true,
		"CODE2": true,
	}
	code1 := generateUniqueCode(existing)
	assert.NotEmpty(t, code1)
	assert.NotEqual(t, "CODE1", code1)
	assert.NotEqual(t, "CODE2", code1)
}
