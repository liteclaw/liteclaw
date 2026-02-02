package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/rs/zerolog"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Config holds Feishu-specific configuration.
type Config struct {
	AppID     string `json:"appId" yaml:"appId"`
	AppSecret string `json:"appSecret" yaml:"appSecret"`
}

// Adapter implements the Feishu channel adapter.
type Adapter struct {
	*channels.BaseAdapter

	client    *lark.Client
	wsClient  *larkws.Client
	appID     string
	appSecret string

	mu sync.RWMutex
}

// New creates a new Feishu adapter.
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
		Polling:        false, // WebSocket
	}

	baseCfg := &channels.Config{
		Enabled: true,
	}

	base := channels.NewBaseAdapter(
		"feishu",
		"Feishu",
		channels.ChannelTypeFeishu,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter: base,
		appID:       cfg.AppID,
		appSecret:   cfg.AppSecret,
	}
}

// Start starts the Feishu adapter.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	// 1. Create API Client
	a.client = lark.NewClient(a.appID, a.appSecret, lark.WithLogReqAtDebug(true))

	// 2. Register Event Handler
	eventHandler := dispatcher.NewEventDispatcher("", "") // EncryptKey/VerificationToken empty for WS usually
	eventHandler.OnP2MessageReceiveV1(a.handleMessageReceive)

	// 3. Create WebSocket Client
	a.wsClient = larkws.NewClient(
		a.appID,
		a.appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelDebug),
	)

	// 4. Start WebSocket
	go func() {
		a.Logger().Info().Msg("Starting Feishu WebSocket client")
		if err := a.wsClient.Start(context.Background()); err != nil {
			a.Logger().Error().Err(err).Msg("Feishu WebSocket client stopped with error")
			a.SetRunning(false)
		}
	}()

	a.SetRunning(true)
	now := time.Now()
	a.State().LastStartAt = &now
	a.State().Mode = "websocket"

	a.Logger().Info().Str("appId", a.appID).Msg("Feishu adapter started")
	return nil
}

// Stop stops the adapter.
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.IsRunning() {
		return nil
	}

	// Currently larkws SDK implementation of Start() is blocking or not easily cancelable explicitly
	// without context cancellation?
	// Actually wsClient doesn't expose Stop()?
	// The SDK documentation says context cancellation should work or just let it die when process dies.
	// But usually we want graceful stop.
	// We will just mark running false.

	a.SetRunning(false)
	now := time.Now()
	a.State().LastStopAt = &now

	a.Logger().Info().Msg("Feishu adapter stopped")
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
	// Usually verify by calling a simple API like get app info or bot info.
	// But lark SDK doesn't have a simple "Ping" that's generic.
	// We assume OK if config is present.
	return &channels.ProbeResult{
		OK: true,
	}, nil
}

func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	// Use im.v1.messages.create

	contentMap := map[string]string{
		"text": req.Text,
	}
	contentBytes, _ := json.Marshal(contentMap)
	contentStr := string(contentBytes)

	msgReq := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeText).
			ReceiveId(req.To.ChatID).
			Content(contentStr).
			Uuid(fmt.Sprintf("%d", time.Now().UnixNano())). // generic simple uuid
			Build()).
		Build()

	resp, err := a.client.Im.Message.Create(ctx, msgReq)
	if err != nil {
		return &channels.SendResult{Success: false, Error: err.Error()}, err
	}

	if !resp.Success() {
		return &channels.SendResult{Success: false, Error: fmt.Sprintf("code:%d msg:%s", resp.Code, resp.Msg)}, fmt.Errorf("feishu api error: %s", resp.Msg)
	}

	return &channels.SendResult{
		MessageID: *resp.Data.MessageId,
		Success:   true,
	}, nil
}

func (a *Adapter) SendReaction(ctx context.Context, req *channels.ReactionRequest) error {
	return fmt.Errorf("feishu reaction not implemented")
}

// Event Handler
func (a *Adapter) handleMessageReceive(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	// fmt.Println(larkcore.Prettify(event))

	if event.Event == nil || event.Event.Message == nil {
		return nil
	}

	msg := event.Event.Message
	sender := event.Event.Sender

	// Extract Content
	// Content is a JSON string: "{\"text\":\"hello\"}"
	var contentText string
	var contentMap map[string]interface{}
	if err := json.Unmarshal([]byte(*msg.Content), &contentMap); err == nil {
		if text, ok := contentMap["text"].(string); ok {
			contentText = text
		}
	}
	if contentText == "" {
		// Fallback or other types
		contentText = *msg.Content // keep raw if unknown
	}

	chatType := "group"
	if *msg.ChatType == "p2p" {
		chatType = "direct"
	}

	incoming := &channels.IncomingMessage{
		ID:          *msg.MessageId,
		ChannelType: "feishu",
		ChatID:      *msg.ChatId,             // Reply to ChatId
		SenderID:    *sender.SenderId.OpenId, // Use OpenId as generic sender id
		SenderName:  "unknown",               // Feishu doesn't provide name in message event sender struct by default
		Text:        contentText,
		Timestamp:   time.Now().Unix(), // or event timestamp
		ChatType:    chatType,
		// Meta: map[string]string{"user_id": *sender.SenderId.UserId},
	}

	// Can optimize to fetch user info if needed.

	a.Logger().Info().Str("content", contentText).Str("sender", incoming.SenderID).Msg("Received Feishu message")

	if err := a.Handler().HandleIncoming(ctx, incoming); err != nil {
		a.Logger().Error().Err(err).Msg("Failed to handle Feishu message")
	}

	return nil
}
