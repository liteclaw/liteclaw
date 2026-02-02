// Package matrix provides the Matrix channel adapter for LiteClaw.
// Matrix is an open, decentralized communication protocol.
package matrix

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Adapter implements the Matrix channel adapter.
type Adapter struct {
	*channels.BaseAdapter

	homeserver  string
	userID      string
	accessToken string
	client      *Client
	syncCancel  context.CancelFunc
	mu          sync.RWMutex
}

// Config holds Matrix-specific configuration.
type Config struct {
	Homeserver  string `json:"homeserver" yaml:"homeserver"` // e.g., "https://matrix.org"
	UserID      string `json:"userId" yaml:"userId"`         // e.g., "@bot:matrix.org"
	AccessToken string `json:"accessToken" yaml:"accessToken"`
}

// New creates a new Matrix adapter.
func New(cfg *Config, logger zerolog.Logger) *Adapter {
	caps := &channels.Capabilities{
		ChatTypes:      []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup, channels.ChatTypeChannel},
		Reactions:      true,
		Threads:        true,
		Media:          true,
		Stickers:       false,
		Voice:          true, // Matrix supports VoIP
		NativeCommands: false,
		BlockStreaming: false,
		Webhooks:       false,
		Polling:        true, // Uses Matrix sync
	}

	baseCfg := &channels.Config{
		Token: cfg.AccessToken,
	}

	base := channels.NewBaseAdapter(
		"matrix",
		"Matrix",
		channels.ChannelTypeMatrix,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter: base,
		homeserver:  cfg.Homeserver,
		userID:      cfg.UserID,
		accessToken: cfg.AccessToken,
	}
}

// Start starts the Matrix adapter.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	if a.accessToken == "" {
		return fmt.Errorf("matrix access token not configured")
	}

	a.client = NewClient(a.homeserver, a.accessToken, a.Logger())

	// Probe to verify connection
	probe, err := a.Probe(ctx)
	if err != nil {
		return fmt.Errorf("failed to probe matrix: %w", err)
	}
	if !probe.OK {
		return fmt.Errorf("matrix probe failed: %s", probe.Error)
	}

	a.Logger().Info().
		Str("userId", a.userID).
		Str("homeserver", a.homeserver).
		Msg("Matrix adapter connected")

	// Start sync loop
	syncCtx, cancel := context.WithCancel(context.Background())
	a.syncCancel = cancel

	go a.syncLoop(syncCtx)

	a.SetRunning(true)
	now := time.Now()
	a.State().LastStartAt = &now
	a.State().Mode = "sync"

	return nil
}

// Stop stops the Matrix adapter.
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.IsRunning() {
		return nil
	}

	if a.syncCancel != nil {
		a.syncCancel()
		a.syncCancel = nil
	}

	a.SetRunning(false)
	now := time.Now()
	a.State().LastStopAt = &now

	a.Logger().Info().Msg("Matrix adapter stopped")
	return nil
}

// Connect connects to Matrix.
func (a *Adapter) Connect(ctx context.Context) error {
	return a.Start(ctx)
}

// Disconnect disconnects from Matrix.
func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.Stop(ctx)
}

// IsConnected returns whether connected to Matrix.
func (a *Adapter) IsConnected() bool {
	return a.IsRunning()
}

// Probe verifies the Matrix connection.
func (a *Adapter) Probe(ctx context.Context) (*channels.ProbeResult, error) {
	if a.accessToken == "" {
		return &channels.ProbeResult{OK: false, Error: "no access token configured"}, nil
	}

	start := time.Now()
	client := NewClient(a.homeserver, a.accessToken, a.Logger())

	userID, err := client.WhoAmI(ctx)
	if err != nil {
		return &channels.ProbeResult{
			OK:        false,
			Error:     err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}, nil
	}

	return &channels.ProbeResult{
		OK:        true,
		BotID:     userID,
		Username:  userID,
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

// Send sends a message via Matrix.
func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("matrix client not initialized")
	}

	roomID := req.To.ChatID
	if roomID == "" {
		return nil, fmt.Errorf("roomId is required")
	}

	eventID, err := a.client.SendMessage(ctx, roomID, req.Text)
	if err != nil {
		return &channels.SendResult{Success: false, Error: err.Error()}, err
	}

	// Update state
	now := time.Now()
	a.State().LastOutboundAt = &now
	a.State().MessageCount++

	return &channels.SendResult{
		MessageID: eventID,
		Success:   true,
	}, nil
}

// SendReaction adds a reaction to a message.
func (a *Adapter) SendReaction(ctx context.Context, req *channels.ReactionRequest) error {
	if a.client == nil {
		return fmt.Errorf("matrix client not initialized")
	}

	return a.client.SendReaction(ctx, req.ChatID, req.MessageID, req.Emoji)
}

// syncLoop runs the Matrix sync loop.
func (a *Adapter) syncLoop(ctx context.Context) {
	nextBatch := ""

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		resp, err := a.client.Sync(ctx, nextBatch)
		if err != nil {
			a.Logger().Error().Err(err).Msg("Matrix sync failed")
			time.Sleep(5 * time.Second)
			continue
		}

		nextBatch = resp.NextBatch

		// Process room events
		for roomID, room := range resp.Rooms.Join {
			for _, event := range room.Timeline.Events {
				a.handleEvent(ctx, roomID, &event)
			}
		}
	}
}

// handleEvent handles a Matrix event.
func (a *Adapter) handleEvent(ctx context.Context, roomID string, event *MatrixEvent) {
	handler := a.Handler()
	if handler == nil {
		return
	}

	// Only handle m.room.message events
	if event.Type != "m.room.message" {
		return
	}

	// Skip our own messages
	if event.Sender == a.userID {
		return
	}

	content, ok := event.Content.(map[string]interface{})
	if !ok {
		return
	}

	body, _ := content["body"].(string)
	msgtype, _ := content["msgtype"].(string)

	// Only handle text messages for now
	if msgtype != "m.text" {
		return
	}

	incoming := &channels.IncomingMessage{
		ID:          event.EventID,
		ChannelType: "matrix",
		ChatID:      roomID,
		SenderID:    event.Sender,
		SenderName:  event.Sender,
		Text:        body,
		Timestamp:   event.OriginServerTS / 1000,
		ChatType:    "group",
	}

	// Update state
	now := time.Now()
	a.State().LastInboundAt = &now

	if err := handler.HandleIncoming(ctx, incoming); err != nil {
		a.Logger().Error().Err(err).Msg("Failed to handle Matrix message")
	}
}
