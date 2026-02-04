package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// Client is a Matrix API client.
type Client struct {
	homeserver  string
	accessToken string
	http        *http.Client
	logger      *zerolog.Logger
	txnID       int64
}

// NewClient creates a new Matrix client.
func NewClient(homeserver, accessToken string, logger *zerolog.Logger) *Client {
	return &Client{
		homeserver:  homeserver,
		accessToken: accessToken,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

// MatrixEvent represents a Matrix event.
type MatrixEvent struct {
	Type           string      `json:"type"`
	EventID        string      `json:"event_id"`
	Sender         string      `json:"sender"`
	OriginServerTS int64       `json:"origin_server_ts"`
	Content        interface{} `json:"content"`
}

// RoomTimeline represents a room's timeline.
type RoomTimeline struct {
	Events []MatrixEvent `json:"events"`
}

// JoinedRoom represents a joined room in sync response.
type JoinedRoom struct {
	Timeline RoomTimeline `json:"timeline"`
}

// SyncRooms represents rooms in sync response.
type SyncRooms struct {
	Join map[string]JoinedRoom `json:"join"`
}

// SyncResponse represents a Matrix sync response.
type SyncResponse struct {
	NextBatch string    `json:"next_batch"`
	Rooms     SyncRooms `json:"rooms"`
}

// WhoAmI returns the current user ID.
func (c *Client) WhoAmI(ctx context.Context) (string, error) {
	resp, err := c.request(ctx, "GET", "/_matrix/client/v3/account/whoami", nil)
	if err != nil {
		return "", err
	}

	var result struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}

	return result.UserID, nil
}

// SendMessage sends a text message to a room.
func (c *Client) SendMessage(ctx context.Context, roomID, text string) (string, error) {
	c.txnID++
	txnID := fmt.Sprintf("%d", c.txnID)

	content := map[string]interface{}{
		"msgtype": "m.text",
		"body":    text,
	}

	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s", roomID, txnID)
	resp, err := c.request(ctx, "PUT", path, content)
	if err != nil {
		return "", err
	}

	var result struct {
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}

	return result.EventID, nil
}

// SendReaction sends a reaction to a message.
func (c *Client) SendReaction(ctx context.Context, roomID, eventID, emoji string) error {
	c.txnID++
	txnID := fmt.Sprintf("%d", c.txnID)

	content := map[string]interface{}{
		"m.relates_to": map[string]interface{}{
			"rel_type": "m.annotation",
			"event_id": eventID,
			"key":      emoji,
		},
	}

	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.reaction/%s", roomID, txnID)
	_, err := c.request(ctx, "PUT", path, content)
	return err
}

// Sync performs a Matrix sync.
func (c *Client) Sync(ctx context.Context, since string) (*SyncResponse, error) {
	path := "/_matrix/client/v3/sync?timeout=30000"
	if since != "" {
		path += "&since=" + since
	}

	resp, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result SyncResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// request makes a request to the Matrix API.
func (c *Client) request(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	url := c.homeserver + path

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("matrix API error: %s (status: %d)", string(respBody), resp.StatusCode)
	}

	return respBody, nil
}
