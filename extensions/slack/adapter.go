// Package slack provides the Slack channel adapter for LiteClaw.
package slack

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Adapter implements the Slack channel adapter.
type Adapter struct {
	*channels.BaseAdapter

	token    string
	botID    string
	botName  string
	client   *Client
	wsCancel context.CancelFunc
	mu       sync.RWMutex
}

// Config holds Slack-specific configuration.
type Config struct {
	Token         string `json:"token" yaml:"token"`       // Bot token (xoxb-...)
	AppToken      string `json:"appToken" yaml:"appToken"` // App-level token (xapp-...) for Socket Mode
	SigningSecret string `json:"signingSecret,omitempty" yaml:"signingSecret,omitempty"`
}

// New creates a new Slack adapter.
func New(cfg *Config, logger zerolog.Logger) *Adapter {
	caps := &channels.Capabilities{
		ChatTypes:      []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup, channels.ChatTypeChannel, channels.ChatTypeThread},
		Reactions:      true,
		Threads:        true,
		Media:          true,
		Stickers:       false, // Slack uses custom emoji instead
		Voice:          false,
		NativeCommands: true, // Slash commands
		BlockStreaming: false,
		Webhooks:       true,
		Polling:        false, // Uses Socket Mode (WebSocket)
	}

	baseCfg := &channels.Config{
		Token: cfg.Token,
	}

	base := channels.NewBaseAdapter(
		"slack",
		"Slack",
		channels.ChannelTypeSlack,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter: base,
		token:       cfg.Token,
	}
}

// Start starts the Slack adapter.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	if a.token == "" {
		return fmt.Errorf("slack token not configured")
	}

	// Create client
	a.client = NewClient(a.token, a.Logger())

	// Probe to verify token
	probe, err := a.Probe(ctx)
	if err != nil {
		return fmt.Errorf("failed to probe slack: %w", err)
	}
	if !probe.OK {
		return fmt.Errorf("slack probe failed: %s", probe.Error)
	}

	a.botID = probe.BotID
	a.botName = probe.BotName

	a.Logger().Info().
		Str("botId", a.botID).
		Str("botName", a.botName).
		Msg("Slack adapter connected")

	a.SetRunning(true)
	now := time.Now()
	state := a.State()
	state.LastStartAt = &now
	state.Mode = "socket"

	return nil
}

// Stop stops the Slack adapter.
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

	a.SetRunning(false)
	now := time.Now()
	a.State().LastStopAt = &now

	a.Logger().Info().Msg("Slack adapter stopped")
	return nil
}

// Connect connects to Slack.
func (a *Adapter) Connect(ctx context.Context) error {
	return a.Start(ctx)
}

// Disconnect disconnects from Slack.
func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.Stop(ctx)
}

// IsConnected returns whether connected to Slack.
func (a *Adapter) IsConnected() bool {
	return a.IsRunning()
}

// Probe verifies the Slack token.
func (a *Adapter) Probe(ctx context.Context) (*channels.ProbeResult, error) {
	if a.token == "" {
		return &channels.ProbeResult{OK: false, Error: "no token configured"}, nil
	}

	start := time.Now()
	client := NewClient(a.token, a.Logger())
	auth, err := client.AuthTest(ctx)
	if err != nil {
		return &channels.ProbeResult{
			OK:        false,
			Error:     err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}, nil
	}

	return &channels.ProbeResult{
		OK:        true,
		BotID:     auth.BotID,
		BotName:   auth.User,
		Username:  auth.User,
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

// Send sends a message via Slack.
func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("slack client not initialized")
	}

	channel := req.To.ChatID
	if channel == "" {
		return nil, fmt.Errorf("channel is required")
	}

	opts := &PostMessageOptions{
		ThreadTS: req.ThreadID,
	}

	ts, err := a.client.PostMessage(ctx, channel, req.Text, opts)
	if err != nil {
		return &channels.SendResult{Success: false, Error: err.Error()}, err
	}

	// Update state
	now := time.Now()
	a.State().LastOutboundAt = &now
	a.State().MessageCount++

	return &channels.SendResult{
		MessageID: ts,
		Success:   true,
	}, nil
}

// SendReaction adds a reaction to a message.
func (a *Adapter) SendReaction(ctx context.Context, req *channels.ReactionRequest) error {
	if a.client == nil {
		return fmt.Errorf("slack client not initialized")
	}

	if req.Remove {
		return a.client.RemoveReaction(ctx, req.ChatID, req.MessageID, req.Emoji)
	}
	return a.client.AddReaction(ctx, req.ChatID, req.MessageID, req.Emoji)
}
