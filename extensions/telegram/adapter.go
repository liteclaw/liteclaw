package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Adapter implements the Telegram channel adapter.
type Adapter struct {
	*channels.BaseAdapter

	token      string
	botID      string
	botName    string
	client     *Client
	pollCancel context.CancelFunc
	mu         sync.RWMutex
}

// Config holds Telegram-specific configuration.
type Config struct {
	Token      string `json:"token" yaml:"token"`
	TokenFile  string `json:"tokenFile,omitempty" yaml:"tokenFile,omitempty"`
	WebhookURL string `json:"webhookUrl,omitempty" yaml:"webhookUrl,omitempty"`
	Proxy      string `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	ParseMode  string `json:"parseMode,omitempty" yaml:"parseMode,omitempty"` // "Markdown" or "HTML"
}

// New creates a new Telegram adapter.
func New(cfg *Config, logger zerolog.Logger) *Adapter {
	caps := &channels.Capabilities{
		ChatTypes:      []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup, channels.ChatTypeChannel, channels.ChatTypeThread},
		Reactions:      true,
		Threads:        true,
		Media:          true,
		Stickers:       true,
		Voice:          false,
		NativeCommands: true,
		BlockStreaming: true,
		Webhooks:       true,
		Polling:        true,
	}

	baseCfg := &channels.Config{
		Token:      cfg.Token,
		TokenFile:  cfg.TokenFile,
		WebhookURL: cfg.WebhookURL,
		Proxy:      cfg.Proxy,
	}

	base := channels.NewBaseAdapter(
		"telegram",
		"Telegram",
		channels.ChannelTypeTelegram,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter: base,
		token:       cfg.Token,
	}
}

// Start starts the Telegram adapter.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	if err := a.initClient(ctx); err != nil {
		return err
	}

	// Start polling
	pollCtx, cancel := context.WithCancel(context.Background())
	a.pollCancel = cancel

	go a.pollUpdates(pollCtx)

	a.SetRunning(true)
	now := time.Now()
	state := a.State()
	state.LastStartAt = &now
	state.Mode = "polling"

	return nil
}

// Stop stops the Telegram adapter.
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.IsRunning() {
		return nil
	}

	if a.pollCancel != nil {
		a.pollCancel()
		a.pollCancel = nil
	}

	a.SetRunning(false)
	now := time.Now()
	a.State().LastStopAt = &now

	a.Logger().Info().Msg("Telegram adapter stopped")
	return nil
}

// Connect connects to Telegram (same as Start for polling mode).
func (a *Adapter) Connect(ctx context.Context) error {
	return a.Start(ctx)
}

// Disconnect disconnects from Telegram.
func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.Stop(ctx)
}

// IsConnected returns whether connected to Telegram.
func (a *Adapter) IsConnected() bool {
	return a.IsRunning()
}

// Probe verifies the Telegram token.
func (a *Adapter) Probe(ctx context.Context) (*channels.ProbeResult, error) {
	if a.token == "" {
		return &channels.ProbeResult{OK: false, Error: "no token configured"}, nil
	}

	start := time.Now()
	client := NewClient(a.token, a.Logger())
	me, err := client.GetMe(ctx)
	if err != nil {
		return &channels.ProbeResult{
			OK:        false,
			Error:     err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}, nil
	}

	return &channels.ProbeResult{
		OK:        true,
		BotID:     fmt.Sprintf("%d", me.ID),
		BotName:   me.FirstName,
		Username:  me.Username,
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

// Send sends a message via Telegram.
func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("telegram client not initialized")
	}

	chatID := req.To.ChatID
	if chatID == "" {
		return nil, fmt.Errorf("chatId is required")
	}

	opts := &SendMessageOptions{
		ReplyToMessageID: parseMessageID(req.ReplyTo),
		ThreadID:         parseMessageID(req.ThreadID),
	}

	msgID, err := a.client.SendMessage(ctx, chatID, req.Text, opts)
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
		return fmt.Errorf("telegram client not initialized")
	}

	return a.client.SetReaction(ctx, req.ChatID, req.MessageID, req.Emoji, req.Remove)
}

// pollUpdates polls for updates from Telegram.
func (a *Adapter) pollUpdates(ctx context.Context) {
	offset := a.loadOffset()
	timeout := 30

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, err := a.client.GetUpdates(ctx, offset, timeout)
		if err != nil {
			if strings.Contains(err.Error(), "context canceled") {
				// Normal shutdown
				return
			}
			a.Logger().Error().Err(err).Msg("Failed to get updates")
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			a.saveOffset(offset)

			if update.Message != nil {
				a.handleMessage(ctx, update.Message)
			}
		}
	}
}

// handleMessage handles an incoming Telegram message.
func (a *Adapter) handleMessage(ctx context.Context, msg *TelegramMessage) {
	handler := a.Handler()
	if handler == nil {
		return
	}

	// Translate Telegram message to unified format
	incoming := &channels.IncomingMessage{
		ID:          fmt.Sprintf("%d", msg.MessageID),
		ChannelType: "telegram",
		ChatID:      fmt.Sprintf("%d", msg.Chat.ID),
		SenderID:    fmt.Sprintf("%d", msg.From.ID),
		SenderName:  buildSenderName(msg.From),
		Text:        msg.Text,
		Timestamp:   msg.Date,
	}

	// Determine chat type
	switch msg.Chat.Type {
	case "private":
		incoming.ChatType = "direct"
	case "group", "supergroup":
		incoming.ChatType = "group"
	case "channel":
		incoming.ChatType = "channel"
	}

	// Handle thread
	if msg.MessageThreadID > 0 {
		incoming.ThreadID = fmt.Sprintf("%d", msg.MessageThreadID)
	}

	// Handle reply
	if msg.ReplyToMessage != nil {
		incoming.ReplyTo = fmt.Sprintf("%d", msg.ReplyToMessage.MessageID)
	}

	// Update state
	now := time.Now()
	a.State().LastInboundAt = &now

	// Forward to handler (Gateway)
	if err := handler.HandleIncoming(ctx, incoming); err != nil {
		a.Logger().Error().Err(err).Msg("Failed to handle incoming message")
	}
}

func buildSenderName(from *TelegramUser) string {
	if from == nil {
		return "Unknown"
	}
	name := strings.TrimSpace(from.FirstName + " " + from.LastName)
	if name == "" {
		name = from.Username
	}
	return name
}

func parseMessageID(id string) int64 {
	if id == "" {
		return 0
	}
	var n int64
	_, _ = fmt.Sscanf(id, "%d", &n)
	return n
}

// Persistence helpers

func (a *Adapter) getOffsetFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".telegram_offset"
	}
	return filepath.Join(home, ".liteclaw", "telegram_offset")
}

func (a *Adapter) loadOffset() int64 {
	data, err := os.ReadFile(a.getOffsetFile())
	if err != nil {
		return 0
	}
	var offset int64
	_, _ = fmt.Sscanf(string(data), "%d", &offset)
	return offset
}

func (a *Adapter) saveOffset(offset int64) {
	dir := filepath.Dir(a.getOffsetFile())
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(a.getOffsetFile(), []byte(fmt.Sprintf("%d", offset)), 0644)
}

// InitClient initializes the Telegram client without starting polling.
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
		return fmt.Errorf("telegram token not configured")
	}

	// Create client
	a.client = NewClient(a.token, a.Logger())

	// Probe to verify token
	probe, err := a.Probe(ctx)
	if err != nil {
		return fmt.Errorf("failed to probe telegram: %w", err)
	}
	if !probe.OK {
		return fmt.Errorf("telegram probe failed: %s", probe.Error)
	}

	a.botID = probe.BotID
	a.botName = probe.BotName

	a.Logger().Info().
		Str("botId", a.botID).
		Str("botName", a.botName).
		Msg("Telegram adapter connected")

	return nil
}
