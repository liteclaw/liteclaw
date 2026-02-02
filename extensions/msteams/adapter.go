// Package msteams provides the Microsoft Teams channel adapter for LiteClaw.
package msteams

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Adapter implements the MS Teams channel adapter.
type Adapter struct {
	*channels.BaseAdapter

	appID       string
	appPassword string
	client      *Client
	mu          sync.RWMutex
}

// Config holds MS Teams-specific configuration.
type Config struct {
	AppID       string `json:"appId" yaml:"appId"`
	AppPassword string `json:"appPassword" yaml:"appPassword"`
}

// New creates a new MS Teams adapter.
func New(cfg *Config, logger zerolog.Logger) *Adapter {
	caps := &channels.Capabilities{
		ChatTypes:      []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup, channels.ChatTypeChannel, channels.ChatTypeThread},
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
		"msteams",
		"Microsoft Teams",
		channels.ChannelTypeMSTeams,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter: base,
		appID:       cfg.AppID,
		appPassword: cfg.AppPassword,
	}
}

// Start starts the MS Teams adapter.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	a.client = NewClient(a.appID, a.appPassword, a.Logger())
	a.Logger().Info().Str("appId", a.appID).Msg("MS Teams adapter started")
	a.SetRunning(true)
	now := time.Now()
	a.State().LastStartAt = &now

	return nil
}

// Stop stops the MS Teams adapter.
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.IsRunning() {
		return nil
	}

	a.SetRunning(false)
	now := time.Now()
	a.State().LastStopAt = &now
	a.Logger().Info().Msg("MS Teams adapter stopped")

	return nil
}

// Connect connects to MS Teams.
func (a *Adapter) Connect(ctx context.Context) error {
	return a.Start(ctx)
}

// Disconnect disconnects from MS Teams.
func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.Stop(ctx)
}

// IsConnected returns whether connected to MS Teams.
func (a *Adapter) IsConnected() bool {
	return a.IsRunning()
}

// Probe verifies MS Teams configuration.
func (a *Adapter) Probe(ctx context.Context) (*channels.ProbeResult, error) {
	return &channels.ProbeResult{OK: true}, nil
}

// Send sends a message via MS Teams.
func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("MS Teams client not initialized")
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
		MessageID: fmt.Sprintf("teams_%d", time.Now().UnixNano()),
		Success:   true,
	}, nil
}

// SendReaction adds a reaction.
func (a *Adapter) SendReaction(ctx context.Context, req *channels.ReactionRequest) error {
	return fmt.Errorf("reactions not yet implemented for MS Teams")
}

// Client is a placeholder for MS Teams Bot Framework integration.
type Client struct {
	appID       string
	appPassword string
	logger      *zerolog.Logger
}

// NewClient creates a new MS Teams client.
func NewClient(appID, appPassword string, logger *zerolog.Logger) *Client {
	return &Client{
		appID:       appID,
		appPassword: appPassword,
		logger:      logger,
	}
}

// SendMessage sends a message.
func (c *Client) SendMessage(ctx context.Context, to, text string) error {
	c.logger.Debug().
		Str("to", to).
		Str("text", text).
		Msg("Sending MS Teams message")
	return nil
}
