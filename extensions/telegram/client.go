package telegram

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

const apiBaseURL = "https://api.telegram.org/bot"

// Client is a Telegram Bot API client.
type Client struct {
	token  string
	http   *http.Client
	logger *zerolog.Logger
}

// NewClient creates a new Telegram client.
func NewClient(token string, logger *zerolog.Logger) *Client {
	return &Client{
		token: token,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

// TelegramUser represents a Telegram user.
type TelegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// TelegramChat represents a Telegram chat.
type TelegramChat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"` // "private", "group", "supergroup", "channel"
	Title string `json:"title,omitempty"`
}

// TelegramMessage represents a Telegram message.
type TelegramMessage struct {
	MessageID       int64            `json:"message_id"`
	From            *TelegramUser    `json:"from,omitempty"`
	Chat            *TelegramChat    `json:"chat"`
	Date            int64            `json:"date"`
	Text            string           `json:"text,omitempty"`
	MessageThreadID int64            `json:"message_thread_id,omitempty"`
	ReplyToMessage  *TelegramMessage `json:"reply_to_message,omitempty"`
}

// TelegramUpdate represents a Telegram update.
type TelegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *TelegramMessage `json:"message,omitempty"`
}

// APIResponse represents a Telegram API response.
type APIResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
}

// GetMe returns information about the bot.
func (c *Client) GetMe(ctx context.Context) (*TelegramUser, error) {
	resp, err := c.request(ctx, "getMe", nil)
	if err != nil {
		return nil, err
	}

	var user TelegramUser
	if err := json.Unmarshal(resp.Result, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// SendMessageOptions holds options for sending messages.
type SendMessageOptions struct {
	ParseMode        string `json:"parse_mode,omitempty"`
	ReplyToMessageID int64  `json:"reply_to_message_id,omitempty"`
	ThreadID         int64  `json:"message_thread_id,omitempty"`
}

// SendMessage sends a text message.
func (c *Client) SendMessage(ctx context.Context, chatID, text string, opts *SendMessageOptions) (string, error) {
	params := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	if opts != nil {
		if opts.ParseMode != "" {
			params["parse_mode"] = opts.ParseMode
		}
		if opts.ReplyToMessageID > 0 {
			params["reply_to_message_id"] = opts.ReplyToMessageID
		}
		if opts.ThreadID > 0 {
			params["message_thread_id"] = opts.ThreadID
		}
	}

	resp, err := c.request(ctx, "sendMessage", params)
	if err != nil {
		return "", err
	}

	var msg TelegramMessage
	if err := json.Unmarshal(resp.Result, &msg); err != nil {
		return "", err
	}

	return fmt.Sprintf("%d", msg.MessageID), nil
}

// GetUpdates gets updates via long polling.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout int) ([]TelegramUpdate, error) {
	params := map[string]interface{}{
		"offset":  offset,
		"timeout": timeout,
		"allowed_updates": []string{
			"message",
			"edited_message",
			"channel_post",
			"callback_query",
			"message_reaction",
		},
	}

	resp, err := c.request(ctx, "getUpdates", params)
	if err != nil {
		return nil, err
	}

	var updates []TelegramUpdate
	if err := json.Unmarshal(resp.Result, &updates); err != nil {
		return nil, err
	}

	return updates, nil
}

// SetReaction sets a reaction on a message.
func (c *Client) SetReaction(ctx context.Context, chatID, messageID, emoji string, remove bool) error {
	reaction := []map[string]interface{}{}
	if !remove && emoji != "" {
		reaction = append(reaction, map[string]interface{}{
			"type":  "emoji",
			"emoji": emoji,
		})
	}

	params := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"reaction":   reaction,
	}

	_, err := c.request(ctx, "setMessageReaction", params)
	return err
}

// request makes a request to the Telegram API.
func (c *Client) request(ctx context.Context, method string, params map[string]interface{}) (*APIResponse, error) {
	url := apiBaseURL + c.token + "/" + method

	var body io.Reader
	if params != nil {
		jsonBody, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}

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

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, err
	}

	if !apiResp.OK {
		return nil, fmt.Errorf("telegram API error: %s (code: %d)", apiResp.Description, apiResp.ErrorCode)
	}

	return &apiResp, nil
}
