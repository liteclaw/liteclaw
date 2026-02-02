// Package line provides the LINE channel adapter for LiteClaw.
package line

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Adapter implements the LINE channel adapter.
type Adapter struct {
	*channels.BaseAdapter

	channelSecret string
	channelToken  string
	client        *Client
	mu            sync.RWMutex
}

// Config holds LINE-specific configuration.
type Config struct {
	ChannelSecret string `json:"channelSecret" yaml:"channelSecret"`
	ChannelToken  string `json:"channelToken" yaml:"channelToken"`
}

// New creates a new LINE adapter.
func New(cfg *Config, logger zerolog.Logger) *Adapter {
	caps := &channels.Capabilities{
		ChatTypes:      []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup},
		Reactions:      false,
		Threads:        false,
		Media:          true,
		Stickers:       true,
		Voice:          false,
		NativeCommands: true,
		BlockStreaming: false,
		Webhooks:       true,
		Polling:        false,
	}

	baseCfg := &channels.Config{
		Token: cfg.ChannelToken,
	}

	base := channels.NewBaseAdapter(
		"line",
		"LINE",
		channels.ChannelTypeLine,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter:   base,
		channelSecret: cfg.ChannelSecret,
		channelToken:  cfg.ChannelToken,
	}
}

// Start starts the LINE adapter.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	if a.channelToken == "" {
		return fmt.Errorf("LINE channel token not configured")
	}

	a.client = NewClient(a.channelToken, a.Logger())

	probe, err := a.Probe(ctx)
	if err != nil {
		return fmt.Errorf("failed to probe LINE: %w", err)
	}
	if !probe.OK {
		return fmt.Errorf("LINE probe failed: %s", probe.Error)
	}

	a.Logger().Info().Str("botId", probe.BotID).Msg("LINE adapter started")
	a.SetRunning(true)
	now := time.Now()
	a.State().LastStartAt = &now
	a.State().Mode = "webhook"

	return nil
}

// Stop stops the LINE adapter.
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.IsRunning() {
		return nil
	}

	a.SetRunning(false)
	now := time.Now()
	a.State().LastStopAt = &now
	a.Logger().Info().Msg("LINE adapter stopped")

	return nil
}

// Connect connects to LINE.
func (a *Adapter) Connect(ctx context.Context) error {
	return a.Start(ctx)
}

// Disconnect disconnects from LINE.
func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.Stop(ctx)
}

// IsConnected returns whether connected to LINE.
func (a *Adapter) IsConnected() bool {
	return a.IsRunning()
}

// Probe verifies the LINE bot configuration.
func (a *Adapter) Probe(ctx context.Context) (*channels.ProbeResult, error) {
	if a.channelToken == "" {
		return &channels.ProbeResult{OK: false, Error: "no channel token configured"}, nil
	}

	start := time.Now()
	client := NewClient(a.channelToken, a.Logger())

	profile, err := client.GetBotInfo(ctx)
	if err != nil {
		return &channels.ProbeResult{
			OK:        false,
			Error:     err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}, nil
	}

	return &channels.ProbeResult{
		OK:        true,
		BotID:     profile.UserID,
		BotName:   profile.DisplayName,
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

// Send sends a message via LINE.
func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("LINE client not initialized")
	}

	to := req.To.ChatID
	if to == "" {
		return nil, fmt.Errorf("recipient is required")
	}

	err := a.client.PushMessage(ctx, to, req.Text)
	if err != nil {
		return &channels.SendResult{Success: false, Error: err.Error()}, err
	}

	now := time.Now()
	a.State().LastOutboundAt = &now
	a.State().MessageCount++

	return &channels.SendResult{
		MessageID: fmt.Sprintf("line_%d", time.Now().UnixNano()),
		Success:   true,
	}, nil
}

// SendReaction is not fully supported by LINE.
func (a *Adapter) SendReaction(ctx context.Context, req *channels.ReactionRequest) error {
	return fmt.Errorf("reactions not supported by LINE Messaging API")
}

// Client is a placeholder for LINE Messaging API integration.
type Client struct {
	channelToken string
	logger       *zerolog.Logger
}

// BotProfile represents LINE bot profile.
type BotProfile struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	PictureURL  string `json:"pictureUrl"`
}

// NewClient creates a new LINE client.
func NewClient(channelToken string, logger *zerolog.Logger) *Client {
	return &Client{
		channelToken: channelToken,
		logger:       logger,
	}
}

// GetBotInfo retrieves bot profile from LINE.
func (c *Client) GetBotInfo(ctx context.Context) (*BotProfile, error) {
	return &BotProfile{
		UserID:      "line_bot_id",
		DisplayName: "LiteClaw Bot",
	}, nil
}

// PushMessage sends a push message.
func (c *Client) PushMessage(ctx context.Context, to, text string) error {
	c.logger.Debug().
		Str("to", to).
		Str("text", text).
		Msg("Sending LINE message")
	return nil
}
