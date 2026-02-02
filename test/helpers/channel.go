// Package test provides test utilities and helpers for LiteClaw tests.
package test

import (
	"context"
	"sync"
	"testing"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// MockChannel is a mock implementation of channels.Channel for testing.
type MockChannel struct {
	name      string
	chanType  string
	connected bool
	mu        sync.RWMutex
	messages  []channels.Message
	handler   channels.Handler
}

// NewMockChannel creates a new mock channel.
func NewMockChannel(name, chanType string) *MockChannel {
	return &MockChannel{
		name:     name,
		chanType: chanType,
		messages: make([]channels.Message, 0),
	}
}

// Name returns the channel name.
func (c *MockChannel) Name() string {
	return c.name
}

// Type returns the channel type.
func (c *MockChannel) Type() string {
	return c.chanType
}

// Start starts the mock channel.
func (c *MockChannel) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = true
	return nil
}

// Stop stops the mock channel.
func (c *MockChannel) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// IsConnected returns whether the channel is connected.
func (c *MockChannel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// SendMessage records a sent message.
func (c *MockChannel) SendMessage(ctx context.Context, dest channels.Destination, msg *channels.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, *msg)
	return nil
}

// SetHandler sets the message handler.
func (c *MockChannel) SetHandler(handler channels.Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// SentMessages returns all sent messages.
func (c *MockChannel) SentMessages() []channels.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]channels.Message, len(c.messages))
	copy(result, c.messages)
	return result
}

// SimulateIncoming simulates an incoming message.
func (c *MockChannel) SimulateIncoming(ctx context.Context, msg *channels.IncomingMessage) error {
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()

	if handler != nil {
		return handler.HandleMessage(ctx, msg)
	}
	return nil
}

// Reset clears all recorded messages.
func (c *MockChannel) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = make([]channels.Message, 0)
}

// AssertMessageSent asserts that a message was sent.
func (c *MockChannel) AssertMessageSent(t *testing.T, text string) {
	t.Helper()

	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, msg := range c.messages {
		if msg.Text == text {
			return
		}
	}

	t.Errorf("Expected message %q to be sent, but it wasn't. Sent messages: %v", text, c.messages)
}

// AssertNoMessagesSent asserts that no messages were sent.
func (c *MockChannel) AssertNoMessagesSent(t *testing.T) {
	t.Helper()

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.messages) > 0 {
		t.Errorf("Expected no messages to be sent, but got: %v", c.messages)
	}
}
