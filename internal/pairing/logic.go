package pairing

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
)

// IsAllowed checks if an ID is in the allow list
func IsAllowed(channel, id string) (bool, error) {
	// TS logic: readChannelAllowFromStore(channel)
	// We need to read the store without locking for write? Or use read lock?
	// For simplicity, we can reuse withFileLock but just read.
	// Or better, make a read helper.

	allowed := false
	path := getAllowFromPath(channel)

	err := withFileLock(path, func() error {
		store, _, err := readJSON(path, AllowFromStoreCtx{Version: PairingVersion, AllowFrom: []string{}})
		if err != nil {
			return err
		}

		id = strings.TrimSpace(id)
		for _, v := range store.AllowFrom {
			if v == id {
				allowed = true
				break
			}
		}
		return nil
	})

	return allowed, err
}

// ListChannelPairingRequests lists pending requests for a channel
func ListChannelPairingRequests(channel string) ([]PairingRequest, error) {
	var requests []PairingRequest
	err := updatePairingStore(channel, func(store *PairingStoreCtx) (bool, error) {
		reqs := store.Requests
		now := time.Now()

		// Prune expired
		kept, removedExpired := pruneExpiredRequests(reqs, now)

		// Prune excess
		kept, removedExcess := pruneExcessRequests(kept, PairingPendingMax)

		store.Requests = kept
		requests = kept
		return removedExpired || removedExcess, nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by CreatedAt
	sort.Slice(requests, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, requests[i].CreatedAt)
		tj, _ := time.Parse(time.RFC3339, requests[j].CreatedAt)
		return ti.Before(tj)
	})

	return requests, nil
}

// UpsertChannelPairingRequest creates or updates a pairing request
func UpsertChannelPairingRequest(channel, id string, meta map[string]string) (string, bool, error) {
	var code string
	var created bool

	err := updatePairingStore(channel, func(store *PairingStoreCtx) (bool, error) {
		reqs := store.Requests
		now := time.Now()

		// Prune expired
		reqs, _ = pruneExpiredRequests(reqs, now)

		// Normalize ID
		id = strings.TrimSpace(id)

		// Check existing
		idx := -1
		existingCodes := make(map[string]bool)
		for i, r := range reqs {
			if r.ID == id {
				idx = i
			}
			existingCodes[r.Code] = true
		}

		nowStr := now.Format(time.RFC3339)

		if idx >= 0 {
			// Update existing
			existing := reqs[idx]
			code = existing.Code
			if code == "" {
				code = generateUniqueCode(existingCodes)
			}

			// Merge meta
			mergedMeta := existing.Meta
			if mergedMeta == nil {
				mergedMeta = make(map[string]string)
			}
			for k, v := range meta {
				mergedMeta[k] = v
			}

			reqs[idx] = PairingRequest{
				ID:         id,
				Code:       code,
				CreatedAt:  existing.CreatedAt, // Keep original creation time
				LastSeenAt: nowStr,
				Meta:       mergedMeta,
			}

			// Prune excess after update
			reqs, _ = pruneExcessRequests(reqs, PairingPendingMax)
			store.Requests = reqs
			created = false
			return true, nil
		}

		// Create new
		// Prune excess first to make room
		reqs, _ = pruneExcessRequests(reqs, PairingPendingMax)

		if PairingPendingMax > 0 && len(reqs) >= PairingPendingMax {
			// Should have been pruned, but strictly enforcing just in case
			store.Requests = reqs
			// Technically if full we accept the prune logic above which keeps latest.
			// But TS logic says if prune didn't make room (because we want to ADD one), we might need to drop oldest.
			// The TS logic: "if expiredRemoved || cappedRemoved" -> write.
			// If full, it might just return error or no code?
			// TS: "if REQS >= MAX ... return {code: "", created: false}" -> it rejects NEW requests if store is somehow full despite pruning?
			// Actually TS `pruneExcessRequests` slices the array.
			// Wait, TS logic:
			//   const { requests: capped, removed: cappedRemoved } = pruneExcessRequests(reqs, PAIRING_PENDING_MAX);
			//   reqs = capped;
			//   if (PAIRING_PENDING_MAX > 0 && reqs.length >= PAIRING_PENDING_MAX) { return ... }
			// This implies if AFTER pruning we are STILL full (which happens if MAX is small and we kept them all), we cannot add new.
			// But pruneExcessRequests takes `maxPending`. It returns `sorted.slice(-maxPending)`. So length is at most maxPending.
			// So `reqs.length` IS `maxPending` (if full).
			// If `reqs.length >= PAIRING_PENDING_MAX`, we cannot add another one without exceeding limit + 1?
			// TS Logic actually seems to effectively CAP the list. If list is full 3/3, and we want to add 1, we return failure?
			// Let's re-read TS carefully:
			// "reqs = capped; if (length >= MAX) { return code: "", created: false }"
			// Yes, it rejects new requests if queue is full.

			store.Requests = reqs
			return true, nil // Saved pruned state, but returned empty code
		}

		code = generateUniqueCode(existingCodes)
		reqs = append(reqs, PairingRequest{
			ID:         id,
			Code:       code,
			CreatedAt:  nowStr,
			LastSeenAt: nowStr,
			Meta:       meta,
		})

		store.Requests = reqs
		created = true
		return true, nil
	})

	if code == "" && !created {
		// Queue full
		return "", false, nil
	}

	return code, created, err
}

// ApproveChannelPairingCode approves a code
func ApproveChannelPairingCode(channel, code string) (*PairingRequest, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, nil
	}

	var approved *PairingRequest

	err := updatePairingStore(channel, func(store *PairingStoreCtx) (bool, error) {
		reqs := store.Requests
		now := time.Now()

		// Prune expired
		reqs, removed := pruneExpiredRequests(reqs, now)

		idx := -1
		for i, r := range reqs {
			if strings.ToUpper(r.Code) == code {
				idx = i
				break
			}
		}

		if idx < 0 {
			store.Requests = reqs
			return removed, nil // Store might have changed due to pruning
		}

		// Found!
		approved = &reqs[idx]

		// Remove from requests
		reqs = append(reqs[:idx], reqs[idx+1:]...)
		store.Requests = reqs

		return true, nil
	})

	if err != nil {
		return nil, err
	}

	if approved != nil {
		// Add to allow list
		if err := AddChannelAllow(channel, approved.ID); err != nil {
			return approved, fmt.Errorf("failed to add to allow list: %w", err)
		}
	}

	return approved, nil
}

// AddChannelAllow adds an ID to allow list
func AddChannelAllow(channel, id string) error {
	return updateAllowFromStore(channel, func(store *AllowFromStoreCtx) (bool, error) {
		id = strings.TrimSpace(id)
		if id == "" {
			return false, nil
		}

		// Check exists
		for _, allowed := range store.AllowFrom {
			if allowed == id {
				return false, nil
			}
		}

		store.AllowFrom = append(store.AllowFrom, id)
		return true, nil
	})
}

// Helpers

func pruneExpiredRequests(reqs []PairingRequest, now time.Time) ([]PairingRequest, bool) {
	var kept []PairingRequest
	removed := false
	for _, r := range reqs {
		createdAt, err := time.Parse(time.RFC3339, r.CreatedAt)
		if err != nil || now.Sub(createdAt) > PairingPendingTTL {
			removed = true
			continue
		}
		kept = append(kept, r)
	}
	return kept, removed
}

func pruneExcessRequests(reqs []PairingRequest, max int) ([]PairingRequest, bool) {
	if max <= 0 || len(reqs) <= max {
		return reqs, false
	}

	// Sort by LastSeenAt (or CreatedAt) ascending
	sorted := make([]PairingRequest, len(reqs))
	copy(sorted, reqs)

	sort.Slice(sorted, func(i, j int) bool {
		ti := parseTime(sorted[i].LastSeenAt, sorted[i].CreatedAt)
		tj := parseTime(sorted[j].LastSeenAt, sorted[j].CreatedAt)
		return ti.Before(tj)
	})

	// Keep last N
	return sorted[len(sorted)-max:], true
}

func parseTime(ts1, ts2 string) time.Time {
	t, err := time.Parse(time.RFC3339, ts1)
	if err == nil {
		return t
	}
	t, err = time.Parse(time.RFC3339, ts2)
	if err == nil {
		return t
	}
	return time.Unix(0, 0)
}

func generateUniqueCode(existing map[string]bool) string {
	for i := 0; i < 500; i++ {
		code := randomCode()
		if !existing[code] {
			return code
		}
	}
	// Fallback
	return randomCode()
}

func randomCode() string {
	b := make([]byte, PairingCodeLength)
	l := big.NewInt(int64(len(PairingCodeAlphabet)))
	for i := 0; i < PairingCodeLength; i++ {
		n, _ := rand.Int(rand.Reader, l)
		b[i] = PairingCodeAlphabet[n.Int64()]
	}
	return string(b)
}
