// Package channels provides the communication channel framework.
package channels

import (
	"context"

	"github.com/rs/zerolog"
)

// Adapter is the core interface that all channel implementations must satisfy.
// It acts as a translator between the external messaging platform and LiteClaw Gateway.
//
// Design Philosophy:
//   - Adapters are "translators" - they convert platform-specific formats to unified LiteClaw messages
//   - All adapters share the same lifecycle and interface
//   - The Gateway doesn't know about specific platforms, only about generic messages
type Adapter interface {
	// Metadata
	ID() string                  // Unique identifier (e.g., "telegram", "discord")
	Name() string                // Human-readable name
	Type() ChannelType           // Channel type enum
	Capabilities() *Capabilities // What this adapter supports

	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool

	// Connection
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	IsConnected() bool
	Probe(ctx context.Context) (*ProbeResult, error)

	// Messaging (outbound from Gateway to platform)
	Send(ctx context.Context, req *SendRequest) (*SendResult, error)
	SendReaction(ctx context.Context, req *ReactionRequest) error

	// State
	State() *RuntimeState
	SetHandler(handler MessageHandler)
}

// MessageHandler is called when the adapter receives a message from the platform.
// This is the bridge between the adapter and the Gateway.
type MessageHandler interface {
	// HandleIncoming is called when a message arrives from the platform.
	// The adapter translates platform-specific format to IncomingMessage.
	HandleIncoming(ctx context.Context, msg *IncomingMessage) error
}

// MessageHandlerFunc is a function adapter for MessageHandler.
type MessageHandlerFunc func(ctx context.Context, msg *IncomingMessage) error

func (f MessageHandlerFunc) HandleIncoming(ctx context.Context, msg *IncomingMessage) error {
	return f(ctx, msg)
}

// SendRequest is a unified request for sending messages.
type SendRequest struct {
	To          Destination  `json:"to"`
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments,omitempty"`
	ReplyTo     string       `json:"replyTo,omitempty"`
	ThreadID    string       `json:"threadId,omitempty"`
	AccountID   string       `json:"accountId,omitempty"` // For multi-account channels
}

// ReactionRequest is a request to add a reaction.
type ReactionRequest struct {
	ChatID    string `json:"chatId"`
	MessageID string `json:"messageId"`
	Emoji     string `json:"emoji"`
	Remove    bool   `json:"remove,omitempty"` // Remove instead of add
}

// BaseAdapter provides common functionality for all adapters.
type BaseAdapter struct {
	id           string
	name         string
	chanType     ChannelType
	capabilities *Capabilities
	config       *Config
	logger       zerolog.Logger
	handler      MessageHandler
	state        RuntimeState
}

// NewBaseAdapter creates a new base adapter.
func NewBaseAdapter(id, name string, chanType ChannelType, caps *Capabilities, cfg *Config, logger zerolog.Logger) *BaseAdapter {
	return &BaseAdapter{
		id:           id,
		name:         name,
		chanType:     chanType,
		capabilities: caps,
		config:       cfg,
		logger:       logger,
	}
}

func (a *BaseAdapter) ID() string                  { return a.id }
func (a *BaseAdapter) Name() string                { return a.name }
func (a *BaseAdapter) Type() ChannelType           { return a.chanType }
func (a *BaseAdapter) Capabilities() *Capabilities { return a.capabilities }
func (a *BaseAdapter) Config() *Config             { return a.config }

// Logger returns a pointer to the logger for calling pointer receiver methods.
func (a *BaseAdapter) Logger() *zerolog.Logger { return &a.logger }
func (a *BaseAdapter) State() *RuntimeState    { return &a.state }

func (a *BaseAdapter) SetHandler(handler MessageHandler) {
	a.handler = handler
}

func (a *BaseAdapter) Handler() MessageHandler {
	return a.handler
}

func (a *BaseAdapter) IsRunning() bool {
	return a.state.Running
}

func (a *BaseAdapter) SetRunning(running bool) {
	a.state.Running = running
}
