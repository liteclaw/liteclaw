// Package discord provides the Discord channel adapter for LiteClaw.
package discord

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Adapter implements the Discord channel adapter.
type Adapter struct {
	*channels.BaseAdapter

	token    string
	botID    string
	botName  string
	client   *Client
	wsCancel context.CancelFunc
	mu       sync.RWMutex
	config   *Config
}

// Config holds Discord-specific configuration.
// Config holds Discord-specific configuration.
type Config struct {
	Token       string                 `json:"token" yaml:"token"`
	TokenFile   string                 `json:"tokenFile,omitempty" yaml:"tokenFile,omitempty"`
	GuildID     string                 `json:"guildId,omitempty" yaml:"guildId,omitempty"` // Legacy/Optional
	Intents     int                    `json:"intents,omitempty" yaml:"intents,omitempty"`
	GroupPolicy string                 `json:"groupPolicy,omitempty" yaml:"groupPolicy,omitempty"`
	Guilds      map[string]GuildConfig `json:"guilds,omitempty" yaml:"guilds,omitempty"`
}

type GuildConfig struct {
	Slug     string                   `json:"slug" yaml:"slug"`
	Channels map[string]ChannelConfig `json:"channels" yaml:"channels"`
}

type ChannelConfig struct {
	Allow   bool `json:"allow" yaml:"allow"`
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// Default Discord intents (privileged intents require approval from Discord)
const (
	IntentGuilds                 = 1 << 0
	IntentGuildMessages          = 1 << 9
	IntentGuildMessageReactions  = 1 << 10
	IntentDirectMessages         = 1 << 12
	IntentDirectMessageReactions = 1 << 13
	IntentMessageContent         = 1 << 15 // Privileged - needs approval

	DefaultIntents = IntentGuilds | IntentGuildMessages | IntentGuildMessageReactions |
		IntentDirectMessages | IntentDirectMessageReactions | IntentMessageContent
)

// New creates a new Discord adapter.
func New(cfg *Config, logger zerolog.Logger) *Adapter {
	caps := &channels.Capabilities{
		ChatTypes:      []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup, channels.ChatTypeChannel, channels.ChatTypeThread},
		Reactions:      true,
		Threads:        true,
		Media:          true,
		Stickers:       true,
		Voice:          true,
		NativeCommands: true,
		BlockStreaming: false,
		Webhooks:       true,
		Polling:        false, // Discord uses WebSocket
	}

	baseCfg := &channels.Config{
		Token:     cfg.Token,
		TokenFile: cfg.TokenFile,
	}

	base := channels.NewBaseAdapter(
		"discord",
		"Discord",
		channels.ChannelTypeDiscord,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter: base,
		token:       cfg.Token,
		config:      cfg,
	}
}

// Start starts the Discord adapter.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	if err := a.initClient(ctx); err != nil {
		return err
	}

	// Start WebSocket connection
	wsCtx, cancel := context.WithCancel(context.Background())
	a.wsCancel = cancel

	go a.runWebSocket(wsCtx)

	a.SetRunning(true)
	now := time.Now()
	state := a.State()
	state.LastStartAt = &now
	state.Mode = "websocket"

	return nil
}

// Stop stops the Discord adapter.
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.IsRunning() {
		return nil
	}

	if a.wsCancel != nil {
		a.wsCancel()
		a.wsCancel = nil
	}

	if a.client != nil {
		a.client.Close()
	}

	a.SetRunning(false)
	now := time.Now()
	a.State().LastStopAt = &now

	a.Logger().Info().Msg("Discord adapter stopped")
	return nil
}

// Connect connects to Discord.
func (a *Adapter) Connect(ctx context.Context) error {
	return a.Start(ctx)
}

// Disconnect disconnects from Discord.
func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.Stop(ctx)
}

// IsConnected returns whether connected to Discord.
func (a *Adapter) IsConnected() bool {
	return a.IsRunning()
}

// Probe verifies the Discord token.
func (a *Adapter) Probe(ctx context.Context) (*channels.ProbeResult, error) {
	if a.token == "" {
		return &channels.ProbeResult{OK: false, Error: "no token configured"}, nil
	}

	start := time.Now()
	client := NewClient(a.token, a.Logger())
	user, err := client.GetCurrentUser(ctx)
	if err != nil {
		return &channels.ProbeResult{
			OK:        false,
			Error:     err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}, nil
	}

	return &channels.ProbeResult{
		OK:        true,
		BotID:     user.ID,
		BotName:   user.Username,
		Username:  user.Username + "#" + user.Discriminator,
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

// Send sends a message via Discord.
func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("discord client not initialized")
	}

	channelID := req.To.ChatID
	if channelID == "" {
		return nil, fmt.Errorf("channelId is required")
	}

	opts := &SendMessageOptions{}
	if req.ReplyTo != "" {
		opts.MessageReference = &MessageReference{
			MessageID: req.ReplyTo,
		}
	}

	msgID, err := a.client.SendMessage(ctx, channelID, req.Text, opts)
	if err != nil {
		return &channels.SendResult{Success: false, Error: err.Error()}, err
	}

	// Update state
	now := time.Now()
	a.State().LastOutboundAt = &now
	a.State().MessageCount++

	return &channels.SendResult{
		MessageID: msgID,
		Success:   true,
	}, nil
}

// SendReaction adds a reaction to a message.
func (a *Adapter) SendReaction(ctx context.Context, req *channels.ReactionRequest) error {
	if a.client == nil {
		return fmt.Errorf("discord client not initialized")
	}

	if req.Remove {
		return a.client.DeleteReaction(ctx, req.ChatID, req.MessageID, req.Emoji)
	}
	return a.client.CreateReaction(ctx, req.ChatID, req.MessageID, req.Emoji)
}

// runWebSocket runs the Discord WebSocket connection.
func (a *Adapter) runWebSocket(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := a.client.ConnectWebSocket(ctx, a.handleEvent)
		if err != nil {
			a.Logger().Error().Err(err).Msg("WebSocket connection error")
			a.State().LastError = err.Error()
		}

		// Wait before reconnecting
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// handleEvent handles a Discord gateway event.
func (a *Adapter) handleEvent(event *GatewayEvent) {
	handler := a.Handler()
	if handler == nil {
		return
	}

	// Only handle MESSAGE_CREATE events
	if event.Type != "MESSAGE_CREATE" {
		return
	}

	msg, ok := event.Data.(*DiscordMessage)
	if !ok || msg == nil {
		return
	}

	// Ignore messages from bots (including ourselves)
	if msg.Author.Bot {
		return
	}

	// Translate Discord message to unified format
	incoming := &channels.IncomingMessage{
		ID:          msg.ID,
		ChannelType: "discord",
		ChatID:      msg.ChannelID,
		SenderID:    msg.Author.ID,
		SenderName:  msg.Author.Username,
		Text:        msg.Content,
		Timestamp:   msg.Timestamp.Unix(),
	}

	// Determine chat type based on guild presence
	if msg.GuildID != "" {
		incoming.ChatType = "group"
	} else {
		incoming.ChatType = "direct"
	}

	// Handle thread
	if msg.Thread != nil {
		incoming.ThreadID = msg.Thread.ID
	}

	// Handle reply
	if msg.MessageReference != nil {
		incoming.ReplyTo = msg.MessageReference.MessageID
	}

	// Update state
	now := time.Now()
	a.State().LastInboundAt = &now

	// Forward to handler (Gateway)
	ctx := context.Background()
	if err := handler.HandleIncoming(ctx, incoming); err != nil {
		a.Logger().Error().Err(err).Msg("Failed to handle incoming message")
	}
}

// InitClient initializes the Discord client without starting WebSocket.
// This is useful for CLI commands that only need to send messages.
func (a *Adapter) InitClient(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.initClient(ctx)
}

func (a *Adapter) initClient(ctx context.Context) error {
	if a.client != nil {
		return nil
	}

	if a.token == "" {
		return fmt.Errorf("discord token not configured")
	}

	// Create client
	a.client = NewClient(a.token, a.Logger())

	// Probe to verify token
	probe, err := a.Probe(ctx)
	if err != nil {
		return fmt.Errorf("failed to probe discord: %w", err)
	}
	if !probe.OK {
		return fmt.Errorf("discord probe failed: %s", probe.Error)
	}

	a.botID = probe.BotID
	a.botName = probe.BotName

	a.Logger().Info().
		Str("botId", a.botID).
		Str("botName", a.botName).
		Msg("Discord adapter connected")

	return nil
}
