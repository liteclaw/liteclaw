package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const (
	apiBaseURL = "https://discord.com/api/v10"
	gatewayURL = "wss://gateway.discord.gg/?v=10&encoding=json"
)

// Client is a Discord API client.
type Client struct {
	token     string
	http      *http.Client
	logger    *zerolog.Logger
	ws        *websocket.Conn
	wsMu      sync.Mutex
	sequence  *int64
	heartbeat time.Duration
	// sessionID is reserved for future resume functionality
}

// NewClient creates a new Discord client.
func NewClient(token string, logger *zerolog.Logger) *Client {
	return &Client{
		token: token,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// DiscordUser represents a Discord user.
type DiscordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Bot           bool   `json:"bot,omitempty"`
	Avatar        string `json:"avatar,omitempty"`
}

// DiscordThread represents a Discord thread.
type DiscordThread struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// MessageReference represents a message reference.
type MessageReference struct {
	MessageID string `json:"message_id,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
	GuildID   string `json:"guild_id,omitempty"`
}

// DiscordMessage represents a Discord message.
type DiscordMessage struct {
	ID               string            `json:"id"`
	ChannelID        string            `json:"channel_id"`
	GuildID          string            `json:"guild_id,omitempty"`
	Author           *DiscordUser      `json:"author"`
	Content          string            `json:"content"`
	Timestamp        time.Time         `json:"timestamp"`
	Thread           *DiscordThread    `json:"thread,omitempty"`
	MessageReference *MessageReference `json:"message_reference,omitempty"`
}

// GatewayEvent represents a Discord gateway event.
type GatewayEvent struct {
	Op   int             `json:"op"`
	Data interface{}     `json:"d"`
	Seq  *int64          `json:"s,omitempty"`
	Type string          `json:"t,omitempty"`
	Raw  json.RawMessage `json:"-"` // Raw data for parsing
}

// gatewayPayload for sending/receiving
type gatewayPayload struct {
	Op   int             `json:"op"`
	Data json.RawMessage `json:"d,omitempty"`
	Seq  *int64          `json:"s,omitempty"`
	Type string          `json:"t,omitempty"`
}

// GetCurrentUser returns the current bot user.
func (c *Client) GetCurrentUser(ctx context.Context) (*DiscordUser, error) {
	resp, err := c.request(ctx, "GET", "/users/@me", nil)
	if err != nil {
		return nil, err
	}

	var user DiscordUser
	if err := json.Unmarshal(resp, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// SendMessageOptions holds options for sending messages.
type SendMessageOptions struct {
	MessageReference *MessageReference `json:"message_reference,omitempty"`
	TTS              bool              `json:"tts,omitempty"`
}

// SendMessage sends a text message to a channel.
func (c *Client) SendMessage(ctx context.Context, channelID, content string, opts *SendMessageOptions) (string, error) {
	payload := map[string]interface{}{
		"content": content,
	}

	if opts != nil && opts.MessageReference != nil {
		payload["message_reference"] = opts.MessageReference
	}

	resp, err := c.request(ctx, "POST", "/channels/"+channelID+"/messages", payload)
	if err != nil {
		return "", err
	}

	var msg DiscordMessage
	if err := json.Unmarshal(resp, &msg); err != nil {
		return "", err
	}

	return msg.ID, nil
}

// CreateReaction adds a reaction to a message.
func (c *Client) CreateReaction(ctx context.Context, channelID, messageID, emoji string) error {
	// URL encode the emoji
	encodedEmoji := url.PathEscape(emoji)
	_, err := c.request(ctx, "PUT", "/channels/"+channelID+"/messages/"+messageID+"/reactions/"+encodedEmoji+"/@me", nil)
	return err
}

// DeleteReaction removes a reaction from a message.
func (c *Client) DeleteReaction(ctx context.Context, channelID, messageID, emoji string) error {
	encodedEmoji := url.PathEscape(emoji)
	_, err := c.request(ctx, "DELETE", "/channels/"+channelID+"/messages/"+messageID+"/reactions/"+encodedEmoji+"/@me", nil)
	return err
}

// ConnectWebSocket connects to the Discord gateway.
func (c *Client) ConnectWebSocket(ctx context.Context, handler func(*GatewayEvent)) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, gatewayURL, nil)
	if err != nil {
		return err
	}

	c.wsMu.Lock()
	c.ws = conn
	c.wsMu.Unlock()

	// Read HELLO
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	var hello gatewayPayload
	if err := json.Unmarshal(msg, &hello); err != nil {
		return err
	}

	if hello.Op != 10 {
		return fmt.Errorf("expected HELLO (op 10), got op %d", hello.Op)
	}

	// Parse heartbeat interval
	var helloData struct {
		HeartbeatInterval int `json:"heartbeat_interval"`
	}
	if err := json.Unmarshal(hello.Data, &helloData); err != nil {
		return err
	}
	c.heartbeat = time.Duration(helloData.HeartbeatInterval) * time.Millisecond

	// Send IDENTIFY
	identify := map[string]interface{}{
		"token":   c.token,
		"intents": DefaultIntents,
		"properties": map[string]string{
			"os":      "linux",
			"browser": "liteclaw",
			"device":  "liteclaw",
		},
	}
	identifyData, _ := json.Marshal(identify)
	identifyPayload := gatewayPayload{
		Op:   2,
		Data: identifyData,
	}
	if err := conn.WriteJSON(identifyPayload); err != nil {
		return err
	}

	// Start heartbeat
	go c.heartbeatLoop(ctx)

	// Read events
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var payload gatewayPayload
		if err := json.Unmarshal(msg, &payload); err != nil {
			c.logger.Error().Err(err).Msg("Failed to parse gateway payload")
			continue
		}

		// Update sequence number
		if payload.Seq != nil {
			c.sequence = payload.Seq
		}

		// Handle dispatch events
		if payload.Op == 0 && payload.Type != "" {
			event := &GatewayEvent{
				Op:   payload.Op,
				Type: payload.Type,
				Seq:  payload.Seq,
			}

			// Parse MESSAGE_CREATE specifically
			if payload.Type == "MESSAGE_CREATE" {
				var msg DiscordMessage
				if err := json.Unmarshal(payload.Data, &msg); err == nil {
					event.Data = &msg
				}
			}

			handler(event)
		}
	}
}

// heartbeatLoop sends heartbeats at the specified interval.
func (c *Client) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.wsMu.Lock()
			if c.ws != nil {
				payload := gatewayPayload{Op: 1}
				if c.sequence != nil {
					seqData, _ := json.Marshal(*c.sequence)
					payload.Data = seqData
				}
				_ = c.ws.WriteJSON(payload)
			}
			c.wsMu.Unlock()
		}
	}
}

// Close closes the WebSocket connection.
func (c *Client) Close() {
	c.wsMu.Lock()
	defer c.wsMu.Unlock()

	if c.ws != nil {
		_ = c.ws.Close()
		c.ws = nil
	}
}

// request makes an HTTP request to the Discord API.
func (c *Client) request(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	url := apiBaseURL + path

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

	req.Header.Set("Authorization", "Bot "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("discord API error: %s (status: %d)", string(respBody), resp.StatusCode)
	}

	return respBody, nil
}
