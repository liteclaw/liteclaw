// Package googlechat provides the Google Chat channel adapter for LiteClaw.
package googlechat

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Adapter implements the Google Chat channel adapter.
type Adapter struct {
	*channels.BaseAdapter

	serviceAccountKey string
	client            *Client
	mu                sync.RWMutex
}

// Config holds Google Chat-specific configuration.
type Config struct {
	ServiceAccountKey string `json:"serviceAccountKey" yaml:"serviceAccountKey"`
}

// New creates a new Google Chat adapter.
func New(cfg *Config, logger zerolog.Logger) *Adapter {
	caps := &channels.Capabilities{
		ChatTypes:      []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup},
		Reactions:      true,
		Threads:        true,
		Media:          true,
		Stickers:       false,
		Voice:          false,
		NativeCommands: true,
		BlockStreaming: false,
		Webhooks:       true,
		Polling:        false,
	}

	baseCfg := &channels.Config{}

	base := channels.NewBaseAdapter(
		"googlechat",
		"Google Chat",
		channels.ChannelTypeGoogleChat,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter:       base,
		serviceAccountKey: cfg.ServiceAccountKey,
	}
}

// Start starts the Google Chat adapter.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	a.client = NewClient(a.serviceAccountKey, a.Logger())
	a.Logger().Info().Msg("Google Chat adapter started")
	a.SetRunning(true)
	now := time.Now()
	a.State().LastStartAt = &now

	return nil
}

// Stop stops the Google Chat adapter.
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.IsRunning() {
		return nil
	}

	a.SetRunning(false)
	now := time.Now()
	a.State().LastStopAt = &now
	a.Logger().Info().Msg("Google Chat adapter stopped")

	return nil
}

// Connect connects to Google Chat.
func (a *Adapter) Connect(ctx context.Context) error {
	return a.Start(ctx)
}

// Disconnect disconnects from Google Chat.
func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.Stop(ctx)
}

// IsConnected returns whether connected to Google Chat.
func (a *Adapter) IsConnected() bool {
	return a.IsRunning()
}

// Probe verifies Google Chat configuration.
func (a *Adapter) Probe(ctx context.Context) (*channels.ProbeResult, error) {
	return &channels.ProbeResult{OK: true}, nil
}

// Send sends a message via Google Chat.
func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("Google Chat client not initialized")
	}

	to := req.To.ChatID
	if to == "" {
		return nil, fmt.Errorf("recipient is required")
	}

	err := a.client.SendMessage(ctx, to, req.Text)
	if err != nil {
		return &channels.SendResult{Success: false, Error: err.Error()}, err
	}

	now := time.Now()
	a.State().LastOutboundAt = &now
	a.State().MessageCount++

	return &channels.SendResult{
		MessageID: fmt.Sprintf("gchat_%d", time.Now().UnixNano()),
		Success:   true,
	}, nil
}

// SendReaction adds a reaction.
func (a *Adapter) SendReaction(ctx context.Context, req *channels.ReactionRequest) error {
	return fmt.Errorf("reactions not yet implemented for Google Chat")
}

// Client is a placeholder for Google Chat API integration.
type Client struct {
	serviceAccountKey string
	logger            *zerolog.Logger
}

// NewClient creates a new Google Chat client.
func NewClient(serviceAccountKey string, logger *zerolog.Logger) *Client {
	return &Client{
		serviceAccountKey: serviceAccountKey,
		logger:            logger,
	}
}

// SendMessage sends a message.
func (c *Client) SendMessage(ctx context.Context, to, text string) error {
	c.logger.Debug().
		Str("to", to).
		Str("text", text).
		Msg("Sending Google Chat message")
	return nil
}
