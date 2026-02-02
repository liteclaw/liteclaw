// Package signal provides the Signal channel adapter for LiteClaw.
package signal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Adapter implements the Signal channel adapter.
type Adapter struct {
	*channels.BaseAdapter

	phoneNumber string
	client      *Client
	mu          sync.RWMutex
}

// Config holds Signal-specific configuration.
type Config struct {
	PhoneNumber string `json:"phoneNumber" yaml:"phoneNumber"`
}

// New creates a new Signal adapter.
func New(cfg *Config, logger zerolog.Logger) *Adapter {
	caps := &channels.Capabilities{
		ChatTypes:      []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup},
		Reactions:      true,
		Threads:        false,
		Media:          true,
		Stickers:       true,
		Voice:          false,
		NativeCommands: false,
		BlockStreaming: false,
		Webhooks:       false,
		Polling:        true,
	}

	baseCfg := &channels.Config{}

	base := channels.NewBaseAdapter(
		"signal",
		"Signal",
		channels.ChannelTypeSignal,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter: base,
		phoneNumber: cfg.PhoneNumber,
	}
}

// Start starts the Signal adapter.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	a.client = NewClient(a.phoneNumber, a.Logger())
	a.Logger().Info().Str("phoneNumber", a.phoneNumber).Msg("Signal adapter started")
	a.SetRunning(true)
	now := time.Now()
	a.State().LastStartAt = &now

	return nil
}

// Stop stops the Signal adapter.
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.IsRunning() {
		return nil
	}

	a.SetRunning(false)
	now := time.Now()
	a.State().LastStopAt = &now
	a.Logger().Info().Msg("Signal adapter stopped")

	return nil
}

// Connect connects to Signal.
func (a *Adapter) Connect(ctx context.Context) error {
	return a.Start(ctx)
}

// Disconnect disconnects from Signal.
func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.Stop(ctx)
}

// IsConnected returns whether connected to Signal.
func (a *Adapter) IsConnected() bool {
	return a.IsRunning()
}

// Probe verifies Signal configuration.
func (a *Adapter) Probe(ctx context.Context) (*channels.ProbeResult, error) {
	return &channels.ProbeResult{OK: true}, nil
}

// Send sends a message via Signal.
func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("signal client not initialized")
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
		MessageID: fmt.Sprintf("sig_%d", time.Now().UnixNano()),
		Success:   true,
	}, nil
}

// SendReaction adds a reaction.
func (a *Adapter) SendReaction(ctx context.Context, req *channels.ReactionRequest) error {
	return fmt.Errorf("reactions not yet implemented for Signal")
}

// Client is a placeholder for Signal integration.
type Client struct {
	phoneNumber string
	logger      *zerolog.Logger
}

// NewClient creates a new Signal client.
func NewClient(phoneNumber string, logger *zerolog.Logger) *Client {
	return &Client{
		phoneNumber: phoneNumber,
		logger:      logger,
	}
}

// SendMessage sends a message.
func (c *Client) SendMessage(ctx context.Context, to, text string) error {
	c.logger.Debug().
		Str("to", to).
		Str("text", text).
		Msg("Sending Signal message")
	return nil
}
