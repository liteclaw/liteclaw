package dingtalk

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/rs/zerolog"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Config holds DingTalk-specific configuration.
type Config struct {
	AppKey    string `json:"appKey" yaml:"appKey"`
	AppSecret string `json:"appSecret" yaml:"appSecret"`
}

// Adapter implements the DingTalk channel adapter.
type Adapter struct {
	*channels.BaseAdapter

	appKey    string
	appSecret string
	stream    *client.StreamClient

	mu sync.RWMutex
}

// New creates a new DingTalk adapter.
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
		Webhooks:       false,
		Polling:        false, // Stream (WebSocket)
	}

	baseCfg := &channels.Config{
		Enabled: true,
	}

	base := channels.NewBaseAdapter(
		"dingtalk",
		"DingTalk",
		channels.ChannelTypeDingTalk,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter: base,
		appKey:      cfg.AppKey,
		appSecret:   cfg.AppSecret,
	}
}

// Start starts the DingTalk adapter.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	// Disable internal SDK logger or redirect?
	// The SDK uses a global logger.SetLogger(logger.Logger)
	logger.SetLogger(&DingLogger{Logger: a.Logger()})

	// Create Stream Client
	stream := client.NewStreamClient(
		client.WithAppCredential(client.NewAppCredentialConfig(a.appKey, a.appSecret)),
		client.WithUserAgent(client.NewDingtalkGoSDKUserAgent()),
	)

	// Register bot callback
	stream.RegisterChatBotCallbackRouter(a.handleChatBotMessage)

	// Start
	if err := stream.Start(ctx); err != nil {
		return fmt.Errorf("failed to start dingtalk stream: %w", err)
	}
	a.stream = stream

	a.SetRunning(true)
	now := time.Now()
	a.State().LastStartAt = &now
	a.State().Mode = "websocket"

	a.Logger().Info().Str("appKey", a.appKey).Msg("DingTalk adapter started")
	return nil
}

// Stop stops the adapter.
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.IsRunning() {
		return nil
	}

	if a.stream != nil {
		a.stream.Close()
	}

	a.SetRunning(false)
	now := time.Now()
	a.State().LastStopAt = &now

	a.Logger().Info().Msg("DingTalk adapter stopped")
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
	// API call to verify key?
	// Just minimal:
	return &channels.ProbeResult{
		OK: true,
	}, nil
}

func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	// IMPORTANT: DingTalk Stream Mode bot sends messages via HTTP (Webhook or Open API), NOT WebSocket.
	// We use access token and simple webhook logic if applicable.

	// token, err := a.getAccessToken(ctx) // Unused for now
	// if err != nil { ... }

	// Fallback to webhook if chatID looks like one
	if strings.HasPrefix(req.To.ChatID, "https://oapi.dingtalk.com") {
		return a.sendViaWebhook(ctx, req.To.ChatID, req.Text)
	}

	return &channels.SendResult{Success: false, Error: "sending via API not fully implemented"}, fmt.Errorf("complex sending not implemented")
}

func (a *Adapter) sendViaWebhook(ctx context.Context, webhook string, text string) (*channels.SendResult, error) {
	client := resty.New()

	body := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": text,
		},
	}

	res, err := client.R().SetBody(body).Post(webhook)
	if err != nil {
		return &channels.SendResult{Success: false, Error: err.Error()}, err
	}
	if res.IsError() {
		return &channels.SendResult{Success: false, Error: res.String()}, fmt.Errorf("server error: %s", res.Status())
	}

	return &channels.SendResult{Success: true, MessageID: "webhook-reply"}, nil
}

/*
func (a *Adapter) getAccessToken(ctx context.Context) (string, error) {
	// https://api.dingtalk.com/v1.0/oauth2/accessToken
	// AppKey, AppSecret
	client := resty.New()
	type TokenResp struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
	}

	var result TokenResp
	resp, err := client.R().
		SetBody(map[string]string{
			"appKey":    a.appKey,
			"appSecret": a.appSecret,
		}).
		SetResult(&result).
		Post("https://api.dingtalk.com/v1.0/oauth2/accessToken")

	if err != nil {
		return "", err
	}
	if resp.IsError() || result.AccessToken == "" {
		return "", fmt.Errorf("failed to get token: %s", resp.String())
	}
	return result.AccessToken, nil
}
*/

func (a *Adapter) SendReaction(ctx context.Context, req *channels.ReactionRequest) error {
	return fmt.Errorf("not implemented")
}

// Callback Handler
func (a *Adapter) handleChatBotMessage(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
	a.Logger().Info().Str("content", data.Text.Content).Str("sender", data.SenderNick).Msg("Received DingTalk message")

	// Store SessionWebhook as ChatID to enable simple reply
	// SessionWebhook is unique to this conversation/user session but EXPIRES.
	// For a quick reply bot, this works. For async long jobs, it might fail.
	// But it's the easiest integration path.

	replyID := data.SessionWebhook
	if replyID == "" {
		replyID = data.ConversationId // Fallback (but won't work with sendViaWebhook without logic)
	}

	incoming := &channels.IncomingMessage{
		ID:          data.MsgId,
		ChannelType: "dingtalk",
		ChatID:      replyID,
		SenderID:    data.SenderStaffId,
		SenderName:  data.SenderNick,
		Text:        strings.TrimSpace(data.Text.Content),
		Timestamp:   data.CreateAt / 1000,
		// ChatType set below
	}

	// DingTalk "ConversationType": "1"(Private), "2"(Group)
	// DingTalk "ConversationType": "1"(Private), "2"(Group)
	switch data.ConversationType {
	case "1":
		incoming.ChatType = "direct"
	case "2":
		incoming.ChatType = "group"
	default:
		incoming.ChatType = "group" // Default fallback
	}

	if err := a.Handler().HandleIncoming(ctx, incoming); err != nil {
		a.Logger().Error().Err(err).Msg("Failed to handle DingTalk message")
	}

	// Return empty response (ack)
	return []byte(""), nil
}

// Logger shim
type DingLogger struct {
	Logger *zerolog.Logger
}

func (l *DingLogger) Debugf(format string, args ...interface{}) {
	// l.Logger.Debug().Msgf(format, args...)
}
func (l *DingLogger) Infof(format string, args ...interface{}) {
	l.Logger.Info().Msgf(format, args...)
}
func (l *DingLogger) Errorf(format string, args ...interface{}) {
	l.Logger.Error().Msgf(format, args...)
}
func (l *DingLogger) Warningf(format string, args ...interface{}) {
	l.Logger.Warn().Msgf(format, args...)
}
func (l *DingLogger) Warnf(format string, args ...interface{}) {
	l.Logger.Warn().Msgf(format, args...)
}

// Implement Fatalf to satisfy interface
func (l *DingLogger) Fatalf(format string, args ...interface{}) {
	l.Logger.Fatal().Msgf(format, args...)
}
func (l *DingLogger) Panicf(format string, args ...interface{}) {
	l.Logger.Panic().Msgf(format, args...)
}

// Add Panicf / Fatalf if interface requires it?
// The interface logger.Logger in SDK has: Debugf, Infof, Warnf, Errorf.
// That seems sufficient.
