package slack

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

const apiBaseURL = "https://slack.com/api"

// Client is a Slack API client.
type Client struct {
	token  string
	http   *http.Client
	logger *zerolog.Logger
}

// NewClient creates a new Slack client.
func NewClient(token string, logger *zerolog.Logger) *Client {
	return &Client{
		token: token,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// AuthTestResponse represents auth.test response.
type AuthTestResponse struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
	URL    string `json:"url"`
	Team   string `json:"team"`
	User   string `json:"user"`
	TeamID string `json:"team_id"`
	UserID string `json:"user_id"`
	BotID  string `json:"bot_id,omitempty"`
}

// AuthTest tests authentication.
func (c *Client) AuthTest(ctx context.Context) (*AuthTestResponse, error) {
	resp, err := c.request(ctx, "POST", "/auth.test", nil)
	if err != nil {
		return nil, err
	}

	var result AuthTestResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if !result.OK {
		return nil, fmt.Errorf("slack auth.test failed: %s", result.Error)
	}

	return &result, nil
}

// PostMessageOptions holds options for posting messages.
type PostMessageOptions struct {
	ThreadTS string `json:"thread_ts,omitempty"`
	Mrkdwn   bool   `json:"mrkdwn,omitempty"`
}

// PostMessageResponse represents chat.postMessage response.
type PostMessageResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Channel string `json:"channel"`
	TS      string `json:"ts"`
}

// PostMessage sends a message to a channel.
func (c *Client) PostMessage(ctx context.Context, channel, text string, opts *PostMessageOptions) (string, error) {
	payload := map[string]interface{}{
		"channel": channel,
		"text":    text,
	}

	if opts != nil {
		if opts.ThreadTS != "" {
			payload["thread_ts"] = opts.ThreadTS
		}
	}

	resp, err := c.request(ctx, "POST", "/chat.postMessage", payload)
	if err != nil {
		return "", err
	}

	var result PostMessageResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}

	if !result.OK {
		return "", fmt.Errorf("slack chat.postMessage failed: %s", result.Error)
	}

	return result.TS, nil
}

// AddReaction adds a reaction to a message.
func (c *Client) AddReaction(ctx context.Context, channel, timestamp, name string) error {
	payload := map[string]interface{}{
		"channel":   channel,
		"timestamp": timestamp,
		"name":      name,
	}

	resp, err := c.request(ctx, "POST", "/reactions.add", payload)
	if err != nil {
		return err
	}

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}

	if !result.OK {
		return fmt.Errorf("slack reactions.add failed: %s", result.Error)
	}

	return nil
}

// RemoveReaction removes a reaction from a message.
func (c *Client) RemoveReaction(ctx context.Context, channel, timestamp, name string) error {
	payload := map[string]interface{}{
		"channel":   channel,
		"timestamp": timestamp,
		"name":      name,
	}

	resp, err := c.request(ctx, "POST", "/reactions.remove", payload)
	if err != nil {
		return err
	}

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}

	if !result.OK && result.Error != "no_reaction" {
		return fmt.Errorf("slack reactions.remove failed: %s", result.Error)
	}

	return nil
}

// request makes a request to the Slack API.
func (c *Client) request(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	url := apiBaseURL + endpoint

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

	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
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

	return respBody, nil
}
