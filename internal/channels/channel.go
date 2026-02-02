// Package channels provides the communication channel framework.
package channels

import (
	"context"
)

// Channel is the legacy interface for communication channels.
// Deprecated: Use Adapter interface instead.
type Channel interface {
	// Name returns the channel name.
	Name() string

	// Type returns the channel type (telegram, discord, slack, web).
	Type() string

	// Start starts the channel.
	Start(ctx context.Context) error

	// Stop stops the channel.
	Stop(ctx context.Context) error

	// IsConnected returns whether the channel is connected.
	IsConnected() bool

	// SendMessage sends a message to a destination.
	SendMessage(ctx context.Context, dest Destination, msg *Message) error
}

// Destination represents a message destination.
type Destination struct {
	ChannelType string `json:"channelType"`
	ChatID      string `json:"chatId"`
	ThreadID    string `json:"threadId,omitempty"`
	UserID      string `json:"userId,omitempty"`
}

// Message represents a message to send.
type Message struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments,omitempty"`
	ReplyTo     string       `json:"replyTo,omitempty"`
}

// Attachment represents a message attachment.
type Attachment struct {
	Type     string `json:"type"` // "image", "file", "audio", "video"
	URL      string `json:"url,omitempty"`
	Path     string `json:"path,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Name     string `json:"name,omitempty"`
}

// IncomingMessage represents an incoming message from a channel.
type IncomingMessage struct {
	ID          string       `json:"id"`
	ChannelType string       `json:"channelType"`
	ChatID      string       `json:"chatId"`
	ChatType    string       `json:"chatType,omitempty"` // "direct", "group", "channel"
	ThreadID    string       `json:"threadId,omitempty"`
	SenderID    string       `json:"senderId"`
	SenderName  string       `json:"senderName"`
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Timestamp   int64        `json:"timestamp"`
	ReplyTo     string       `json:"replyTo,omitempty"`
}

// Handler is the interface for handling incoming messages.
// Deprecated: Use MessageHandler instead.
type Handler interface {
	HandleMessage(ctx context.Context, msg *IncomingMessage) error
}

// HandlerFunc is a function that implements Handler.
type HandlerFunc func(ctx context.Context, msg *IncomingMessage) error

// HandleMessage implements Handler.
func (f HandlerFunc) HandleMessage(ctx context.Context, msg *IncomingMessage) error {
	return f(ctx, msg)
}

// Manager manages all channels.
// Deprecated: Use Registry instead.
type Manager struct {
	channels map[string]Channel
	handler  Handler
}

// NewManager creates a new channel manager.
func NewManager(handler Handler) *Manager {
	return &Manager{
		channels: make(map[string]Channel),
		handler:  handler,
	}
}

// Register registers a channel.
func (m *Manager) Register(ch Channel) {
	m.channels[ch.Name()] = ch
}

// Get returns a channel by name.
func (m *Manager) Get(name string) (Channel, bool) {
	ch, ok := m.channels[name]
	return ch, ok
}

// All returns all channels.
func (m *Manager) All() []Channel {
	result := make([]Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		result = append(result, ch)
	}
	return result
}

// StartAll starts all registered channels.
func (m *Manager) StartAll(ctx context.Context) error {
	for _, ch := range m.channels {
		if err := ch.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// StopAll stops all registered channels.
func (m *Manager) StopAll(ctx context.Context) error {
	for _, ch := range m.channels {
		if err := ch.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Status returns the status of all channels.
func (m *Manager) Status() []ChannelStatus {
	var status []ChannelStatus
	for _, ch := range m.channels {
		status = append(status, ChannelStatus{
			Name:      ch.Name(),
			Type:      ch.Type(),
			Connected: ch.IsConnected(),
		})
	}
	return status
}

// ChannelStatus represents a channel's status.
type ChannelStatus struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Connected bool   `json:"connected"`
}
