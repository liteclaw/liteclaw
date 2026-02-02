package qq

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi"
	"github.com/tencent-connect/botgo/token"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Config holds QQ-specific configuration.
// Config holds QQ-specific configuration.
type Config struct {
	AppID     uint64 `json:"appId" yaml:"appId"`
	AppSecret string `json:"appSecret" yaml:"appSecret"`
	Sandbox   bool   `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
}

// Adapter implements the QQ channel adapter using botgo sdk.
type Adapter struct {
	*channels.BaseAdapter

	api     openapi.OpenAPI
	appID   uint64
	secret  string
	sandbox bool

	mu sync.RWMutex
}

// New creates a new QQ adapter.
func New(cfg *Config, logger zerolog.Logger) *Adapter {
	caps := &channels.Capabilities{
		ChatTypes:      []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup, channels.ChatTypeChannel},
		Reactions:      false,
		Threads:        false,
		Media:          false, // TODO: Implement media upload
		Stickers:       false,
		Voice:          false,
		NativeCommands: true,
		BlockStreaming: true,
		Webhooks:       false,
		Polling:        false, // Using WebSocket
	}

	baseCfg := &channels.Config{
		Enabled: true,
		// Token is not used for QQ anymore
	}

	base := channels.NewBaseAdapter(
		"qq",
		"QQ",
		channels.ChannelTypeQQ,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter: base,
		appID:       cfg.AppID,
		secret:      cfg.AppSecret,
		sandbox:     cfg.Sandbox,
	}
}

// Start starts the QQ adapter by establishing WebSocket connection.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	// Botgo token setup
	tokenSource := token.NewQQBotTokenSource(&token.QQBotCredentials{
		AppID:     strconv.FormatUint(a.appID, 10),
		AppSecret: a.secret,
	})

	// Create API client with sandbox option
	appIDStr := strconv.FormatUint(a.appID, 10)
	if a.sandbox {
		a.api = botgo.NewSandboxOpenAPI(appIDStr, tokenSource).WithTimeout(3 * time.Second)
	} else {
		a.api = botgo.NewOpenAPI(appIDStr, tokenSource).WithTimeout(3 * time.Second)
	}

	wsInfo, err := a.api.WS(ctx, nil, "")
	if err != nil {
		return fmt.Errorf("failed to get websocket info: %w", err)
	}

	// Define Intents - Register ALL handlers (Guild, Group, C2C)
	intent := event.RegisterHandlers(
		a.atMessageEventHandler(),
		a.directMessageEventHandler(),
		a.groupAtMessageEventHandler(),
		a.c2cMessageEventHandler(),
	)

	// Start WebSocket
	go func() {
		a.Logger().Info().Msg("Starting QQ WebSocket session manager")
		if err := botgo.NewSessionManager().Start(wsInfo, tokenSource, &intent); err != nil {
			a.Logger().Error().Err(err).Msg("QQ WebSocket session ended with error")
			a.SetRunning(false)
		}
	}()

	a.SetRunning(true)
	now := time.Now()
	a.State().LastStartAt = &now
	a.State().Mode = "websocket"

	a.Logger().Info().Uint64("appId", a.appID).Bool("sandbox", a.sandbox).Msg("QQ adapter started")
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

	a.Logger().Info().Msg("QQ adapter stopped")
	return nil
}

// Connect implements Adapter interface
func (a *Adapter) Connect(ctx context.Context) error {
	return a.Start(ctx)
}

// Disconnect implements Adapter interface
func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.Stop(ctx)
}

// IsConnected implements Adapter interface
func (a *Adapter) IsConnected() bool {
	return a.IsRunning()
}

// Probe checks if we can call the API ("Me" endpoint)
func (a *Adapter) Probe(ctx context.Context) (*channels.ProbeResult, error) {
	start := time.Now()

	me, err := a.api.Me(ctx)
	if err != nil {
		return &channels.ProbeResult{
			OK:        false,
			Error:     err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}, nil
	}

	return &channels.ProbeResult{
		OK:        true,
		BotID:     me.ID,
		BotName:   me.Username,
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

// Send sends a message to a channel or user.
func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	msgToPost := &dto.MessageToCreate{
		Content: req.Text,
		MsgID:   req.ReplyTo,
	}

	var sentMsg *dto.Message
	var err error

	// Determine sending method based on ChatID prefix
	if strings.HasPrefix(req.To.ChatID, "group:") {
		// QQ Group (群聊)
		groupID := strings.TrimPrefix(req.To.ChatID, "group:")
		sentMsg, err = a.api.PostGroupMessage(ctx, groupID, msgToPost)

	} else if strings.HasPrefix(req.To.ChatID, "c2c:") {
		// QQ C2C (单聊)
		userID := strings.TrimPrefix(req.To.ChatID, "c2c:")
		sentMsg, err = a.api.PostC2CMessage(ctx, userID, msgToPost)

	} else {
		// Default: Guild Channel (频道)
		sentMsg, err = a.api.PostMessage(ctx, req.To.ChatID, msgToPost)
		if err != nil {
			// Fallback: Try Guild Direct Message logic
			// If ChatID is actually a GuildID for DM...
			dummyDM := &dto.DirectMessage{
				GuildID: req.To.ChatID,
			}

			sentMsgDirect, errDirect := a.api.PostDirectMessage(ctx, dummyDM, msgToPost)
			if errDirect == nil {
				sentMsg = sentMsgDirect
				err = nil
			} else {
				a.Logger().Error().Err(err).Msg("Failed to send QQ message")
			}
		}
	}

	if err != nil {
		return &channels.SendResult{Success: false, Error: err.Error()}, err
	}

	return &channels.SendResult{
		MessageID: sentMsg.ID,
		Success:   true,
	}, nil
}

// SendReaction implements Adapter interface
func (a *Adapter) SendReaction(ctx context.Context, req *channels.ReactionRequest) error {
	return fmt.Errorf("QQ reactions not fully implemented yet")
}

// Event Handlers

func (a *Adapter) atMessageEventHandler() event.ATMessageEventHandler {
	return func(e *dto.WSPayload, data *dto.WSATMessageData) error {
		a.Logger().Info().Str("content", data.Content).Str("author", data.Author.Username).Msg("Received QQ Guild @Message")
		a.handleIncoming(data.ID, data.ChannelID, data.Author.ID, data.Author.Username, data.Content, "channel")
		return nil
	}
}

func (a *Adapter) directMessageEventHandler() event.DirectMessageEventHandler {
	return func(e *dto.WSPayload, data *dto.WSDirectMessageData) error {
		a.Logger().Info().Str("content", data.Content).Str("author", data.Author.Username).Msg("Received QQ Guild DM")
		a.handleIncoming(data.ID, data.GuildID, data.Author.ID, data.Author.Username, data.Content, "direct")
		return nil
	}
}

func (a *Adapter) groupAtMessageEventHandler() event.GroupATMessageEventHandler {
	return func(e *dto.WSPayload, data *dto.WSGroupATMessageData) error {
		a.Logger().Info().Str("content", data.Content).Str("group", data.GroupID).Msg("Received QQ Group @Message")
		// Prefix ChatID with 'group:' to distinguish from ChannelID
		senderName := "unknown"
		if data.Author != nil {
			senderName = data.Author.Username
		}
		a.handleIncoming(data.ID, "group:"+data.GroupID, data.Author.ID, senderName, data.Content, "group")
		return nil
	}
}

func (a *Adapter) c2cMessageEventHandler() event.C2CMessageEventHandler {
	return func(e *dto.WSPayload, data *dto.WSC2CMessageData) error {
		a.Logger().Info().Str("content", data.Content).Str("sender", data.Author.ID).Msg("Received QQ C2C Message")
		// Prefix ChatID with 'c2c:'
		senderName := "unknown"
		if data.Author != nil {
			senderName = data.Author.Username
		}
		a.handleIncoming(data.ID, "c2c:"+data.Author.ID, data.Author.ID, senderName, data.Content, "direct")
		return nil
	}
}

// Unified handler
func (a *Adapter) handleIncoming(msgID, chatID, senderID, senderName, content, chatType string) {
	ts := time.Now().Unix()
	incoming := &channels.IncomingMessage{
		ID:          msgID,
		ChannelType: "qq",
		ChatID:      chatID,
		SenderID:    senderID,
		SenderName:  senderName,
		Text:        content,
		Timestamp:   ts,
		ChatType:    chatType,
	}

	if err := a.Handler().HandleIncoming(context.Background(), incoming); err != nil {
		a.Logger().Error().Err(err).Msg("Failed to handle QQ message")
	}
}
