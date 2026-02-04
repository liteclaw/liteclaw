package wecom

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/xen0n/go-workwx"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Adapter implements the WeCom (Enterprise WeChat) channel adapter.
type Adapter struct {
	*channels.BaseAdapter

	callbackHandler http.Handler

	botID          string
	token          string
	encodingAESKey string

	mu sync.RWMutex

	// sync.Map for storing intelligent bot response URLs
	// Key: UserID (string), Value: ResponseURL (string)
	responseURLs sync.Map
}

// New creates a new WeCom adapter.
func New(cfg *Config, logger zerolog.Logger) *Adapter {
	caps := &channels.Capabilities{
		ChatTypes:      []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup},
		Reactions:      false,
		Threads:        false,
		Media:          false,
		Stickers:       false,
		Voice:          false,
		NativeCommands: true,
		BlockStreaming: true,
		Webhooks:       true,
		Polling:        false,
	}

	baseCfg := &channels.Config{
		Enabled: true,
	}

	base := channels.NewBaseAdapter(
		"wecom",
		"WeCom",
		channels.ChannelTypeWeCom,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter:    base,
		botID:          cfg.BotID,
		token:          cfg.Token,
		encodingAESKey: cfg.EncodingAESKey,
	}
}

// Start starts the WeCom adapter.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	// Initialize Callback Handler (Receive Capability)
	if a.token != "" && a.encodingAESKey != "" {
		// NewHTTPHandler expects RxMessageHandler
		handler, err := workwx.NewHTTPHandler(a.token, a.encodingAESKey, a)
		if err != nil {
			return fmt.Errorf("failed to create callback handler: %w", err)
		}

		a.callbackHandler = handler
		a.Logger().Info().Msg("WeCom callback handler initialized")
	} else {
		// Only warn if Token/AESKey missing
		// For Intelligent Bot, these are essential for receiving messages and response URLs
		a.Logger().Warn().Msg("WeCom Token or EncodingAESKey missing, callbacks disabled")
		return fmt.Errorf("token and encodingAesKey are required for WeCom Intelligent Bot")
	}

	a.SetRunning(true)
	now := time.Now()
	a.State().LastStartAt = &now
	a.State().Mode = "webhook"

	a.Logger().Info().Str("botId", a.botID).Msg("WeCom adapter started")
	return nil
}

// Stop stops the adapter.
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.IsRunning() {
		return nil
	}

	a.SetRunning(false)
	now := time.Now()
	a.State().LastStopAt = &now

	a.Logger().Info().Msg("WeCom adapter stopped")
	return nil
}

func (a *Adapter) IsConnected() bool {
	return a.IsRunning()
}

func (a *Adapter) Connect(ctx context.Context) error {
	return a.Start(ctx)
}

func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.Stop(ctx)
}

func (a *Adapter) Probe(ctx context.Context) (*channels.ProbeResult, error) {
	// No explicit ping in workwx, assume OK if configured
	return &channels.ProbeResult{
		OK: true,
	}, nil
}

func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	// Strategy: Use Intelligent Bot Response URL (Webhook mode)
	// We look up the response URL based on the ChatID (which is the UserID)
	// Stored during message reception
	urlVal, ok := a.responseURLs.Load(req.To.ChatID)
	if !ok {
		return &channels.SendResult{Success: false, Error: "no response_url found"}, fmt.Errorf("no pending response url for user %s", req.To.ChatID)
	}
	responseURL := urlVal.(string)

	// Clean up text (remove <think> tags) before sending
	cleanText := req.Text

	// Always remove <think>...</think> blocks from response
	re := regexp.MustCompile(`(?s)<think>.*?</think>`) // (?s) makes . match newlines
	cleanText = re.ReplaceAllString(cleanText, "")
	cleanText = strings.TrimSpace(cleanText)

	// Send POST request to response_url
	// Format: { "msgtype": "markdown", "markdown": { "content": "..." } }
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": cleanText,
		},
	}

	jsonData, _ := json.Marshal(payload)
	a.Logger().Info().Str("url", responseURL).Str("payload", string(jsonData)).Msg("Sending WeCom Reply")

	resp, err := http.Post(responseURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		a.Logger().Error().Err(err).Msg("WeCom Reply Failed (Network)")
		return &channels.SendResult{Success: false, Error: err.Error()}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	// Log the FULL response body from WeCom API
	a.Logger().Info().Int("status", resp.StatusCode).Str("response", string(body)).Msg("WeCom API Response")

	if resp.StatusCode != http.StatusOK {
		return &channels.SendResult{Success: false, Error: string(body)}, fmt.Errorf("api error: %s", string(body))
	}

	// Clean up URL? Maybe keep it for follow-ups?
	// The docs say response_url is valid for a short time or one-time?
	// Usually one-time for "Passive Reply", but here it's "Async Reply"?
	// "Intelligent Bot" usually allows 15s-60s validity.
	// We'll keep it for now.

	return &channels.SendResult{
		MessageID: fmt.Sprintf("%d", time.Now().UnixNano()),
		Success:   true,
	}, nil
}

func (a *Adapter) SendReaction(ctx context.Context, req *channels.ReactionRequest) error {
	return fmt.Errorf("wecom reaction not implemented")
}

// HandleWebhook is an HTTP handler for WeCom callbacks.
// It bridges the Echo context to the go-workwx HTTP handler.
// HandleWebhook is an HTTP handler for WeCom callbacks.
// It bridges the Echo context to the go-workwx HTTP handler.
func (a *Adapter) HandleWebhook(c echo.Context) error {
	// 1. Read Body
	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}
	// Restore body for any subsequent handler
	c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	a.Logger().Info().Str("body", string(bodyBytes)).Msg("WeCom Webhook Payload")

	// 2. Check content type or detect JSON
	isJSON := len(bodyBytes) > 0 && bodyBytes[0] == '{'

	if isJSON {
		return a.handleJSONWebhook(c, bodyBytes)
	}

	if a.callbackHandler == nil {
		return c.String(http.StatusServiceUnavailable, "Callback handler not initialized")
	}

	a.callbackHandler.ServeHTTP(c.Response().Writer, c.Request())
	return nil
}

// handleJSONWebhook handles the special JSON format callbacks from Intelligent Bot.
func (a *Adapter) handleJSONWebhook(c echo.Context, body []byte) error {
	// Parse JSON
	var payload struct {
		Encrypt string `json:"encrypt"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		a.Logger().Error().Err(err).Msg("Failed to parse WeCom JSON body")
		return c.NoContent(http.StatusBadRequest)
	}

	// Verify Signature
	signature := c.QueryParam("msg_signature")
	timestamp := c.QueryParam("timestamp")
	nonce := c.QueryParam("nonce")

	if !a.verifySignature(a.token, timestamp, nonce, payload.Encrypt, signature) {
		a.Logger().Error().Msg("WeCom signature verification failed")
		return c.NoContent(http.StatusForbidden)
	}

	// Decrypt
	msg, err := a.decrypt(payload.Encrypt)
	if err != nil {
		a.Logger().Error().Err(err).Msg("WeCom decryption failed")
		return c.NoContent(http.StatusBadRequest)
	}

	a.Logger().Info().Str("decrypted", string(msg)).Msg("WeCom Decrypted Message")

	// Parse Payload
	// Intelligent Bot returns JSON inside encrypted payload
	// XML Bot returns XML inside encrypted payload
	// Attempt JSON first
	if len(msg) > 0 && msg[0] == '{' {
		// handle JSON format
		var jsonMsg struct {
			MsgID   string `json:"msgid"`
			MsgType string `json:"msgtype"`
			From    struct {
				UserID string `json:"userid"`
			} `json:"from"`
			Text struct {
				Content string `json:"content"`
			} `json:"text"`
			ResponseURL string `json:"response_url"`
		}
		if err := json.Unmarshal(msg, &jsonMsg); err != nil {
			a.Logger().Error().Err(err).Msg("Failed to unmarshal JSON inner message")
			return c.NoContent(http.StatusBadRequest)
		}

		// Store response_url for async reply
		if jsonMsg.ResponseURL != "" {
			a.responseURLs.Store(jsonMsg.From.UserID, jsonMsg.ResponseURL)
			a.Logger().Info().Str("user", jsonMsg.From.UserID).Msg("Stored WeCom response URL")
		}

		// Convert to RxMessage format for compatibility or handle directly
		// Ideally we should create a new handling path or convert.
		// Let's create a fake RxMessage for now to reuse logic, OR map directly to IncomingMessage
		// Since RxMessage is strictly XML-based structs in many fields, it might be cleaner to map directly.

		incoming := &channels.IncomingMessage{
			ID:          jsonMsg.MsgID,
			ChannelType: "wecom",
			ChatID:      jsonMsg.From.UserID,
			SenderID:    jsonMsg.From.UserID,
			SenderName:  jsonMsg.From.UserID,
			Text:        jsonMsg.Text.Content,
			Timestamp:   time.Now().Unix(),
			ChatType:    "direct",
		}

		a.Logger().Info().Str("content", incoming.Text).Str("sender", incoming.SenderID).Msg("Received WeCom JSON message")

		// ASYNC PROCESSING: WeCom requires response within 5 seconds.
		// We spawn a goroutine to handle the agent logic and reply via the response_url later.
		go func() {
			// Use background context so it doesn't get cancelled when HTTP request finishes
			ctx := context.Background()
			if err := a.Handler().HandleIncoming(ctx, incoming); err != nil {
				a.Logger().Error().Err(err).Msg("Failed to handle message asynchronously")
			}
		}()

		// Return 200 OK immediately
		return c.String(http.StatusOK, "success")
	}

	// Parse XML (WeCom usually sends XML inside the encrypted payload even if envelope is JSON)
	var rxMsg workwx.RxMessage
	if err := xml.Unmarshal(msg, &rxMsg); err != nil {
		a.Logger().Error().Err(err).Msg("Failed to unmarshal XML message")
		return c.NoContent(http.StatusBadRequest)
	}

	// Handle
	go func() {
		if err := a.handleWorkwxMessage(&rxMsg); err != nil {
			a.Logger().Error().Err(err).Msg("Failed to handle message asynchronously")
		}
	}()

	return c.String(http.StatusOK, "success")
}

// verifySignature checks the SHA1 signature
func (a *Adapter) verifySignature(token, timestamp, nonce, encrypt, signature string) bool {
	params := []string{token, timestamp, nonce, encrypt}
	sort.Strings(params)
	str := strings.Join(params, "")
	sha := sha1.Sum([]byte(str))
	calculated := hex.EncodeToString(sha[:])
	return calculated == signature
}

// decrypt decrypts the WeCom message
func (a *Adapter) decrypt(encryptedText string) ([]byte, error) {
	aesKey, err := base64.StdEncoding.DecodeString(a.encodingAESKey + "=")
	if err != nil {
		return nil, err
	}

	cipherText, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}

	if len(cipherText) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// AES-CBC
	iv := aesKey[:16]
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(cipherText, cipherText)

	// PKCS7 Unpadding
	text := pkcs7Unpad(cipherText)

	// WeCom Protocol: Random(16) + MsgLen(4) + Msg + ReceiveID
	// Safe extraction
	content := text[16:]
	if len(content) < 4 {
		return nil, fmt.Errorf("content too short")
	}
	xmlLen := int(binary.BigEndian.Uint32(content[:4]))
	if len(content) < 4+xmlLen {
		return nil, fmt.Errorf("invalid xml length")
	}
	xmlMsg := content[4 : 4+xmlLen]

	return xmlMsg, nil
}

func pkcs7Unpad(data []byte) []byte {
	length := len(data)
	if length == 0 {
		return data
	}
	padding := int(data[length-1])
	if padding > length || padding == 0 {
		return data
	}
	return data[:length-padding]
}

// OnIncomingMessage implements workwx.RxMessageHandler
func (a *Adapter) OnIncomingMessage(msg *workwx.RxMessage) error {
	a.Logger().Info().Str("type", string(msg.MsgType)).Msg("WeCom OnIncomingMessage triggered")
	return a.handleWorkwxMessage(msg)
}

// handleWorkwxMessage processes incoming messages from WeCom.
func (a *Adapter) handleWorkwxMessage(msg *workwx.RxMessage) error {
	// Only handle text messages for now
	if msg.MsgType != workwx.MessageTypeText {
		a.Logger().Info().Str("type", string(msg.MsgType)).Msg("Ignored non-text message")
		return nil
	}

	ctx := context.Background()

	// Extract Content
	text := ""
	if textExtras, ok := msg.Text(); ok {
		text = textExtras.GetContent()
	}

	incoming := &channels.IncomingMessage{
		ID:          fmt.Sprintf("%d", msg.MsgID),
		ChannelType: "wecom",
		ChatID:      msg.FromUserID,
		SenderID:    msg.FromUserID,
		SenderName:  msg.FromUserID,
		Text:        text,
		Timestamp:   time.Now().Unix(),
		ChatType:    "direct",
	}

	a.Logger().Info().Str("content", text).Str("sender", incoming.SenderID).Msg("Received WeCom message")

	if err := a.Handler().HandleIncoming(ctx, incoming); err != nil {
		a.Logger().Error().Err(err).Msg("Failed to handle WeCom message")
		return err
	}

	return nil
}
