// Package session provides session management for agent conversations.
package session

import (
	"sync"
	"time"
)

// Session represents an active conversation session.
type Session struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agentId"`
	ChannelType  string    `json:"channelType"`
	ChatID       string    `json:"chatId"`
	ThreadID     string    `json:"threadId,omitempty"`
	UserID       string    `json:"userId"`
	State        State     `json:"state"`
	Messages     []Message `json:"messages"`
	CreatedAt    time.Time `json:"createdAt"`
	LastActiveAt time.Time `json:"lastActiveAt"`
	Metadata     Metadata  `json:"metadata"`
}

// State represents the session state.
type State string

const (
	StateIdle      State = "idle"
	StateWaiting   State = "waiting"
	StateThinking  State = "thinking"
	StateStreaming State = "streaming"
)

// Message represents a conversation message.
type Message struct {
	ID        string     `json:"id"`
	Role      string     `json:"role"` // user, assistant, system, tool
	Content   string     `json:"content"`
	Timestamp time.Time  `json:"timestamp"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
}

// ToolCall represents a tool invocation in a message.
type ToolCall struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Args     map[string]interface{} `json:"args"`
	Result   interface{}            `json:"result,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Duration int64                  `json:"durationMs"`
}

// Metadata holds session metadata.
type Metadata struct {
	Model       string            `json:"model,omitempty"`
	TotalTokens int               `json:"totalTokens,omitempty"`
	Custom      map[string]string `json:"custom,omitempty"`
}

// Manager manages sessions.
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a new session manager.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// Get retrieves a session by ID.
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// GetOrCreate gets an existing session or creates a new one.
func (m *Manager) GetOrCreate(id string, create func() *Session) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[id]; ok {
		return s
	}

	s := create()
	s.ID = id
	s.CreatedAt = time.Now()
	s.LastActiveAt = time.Now()
	s.State = StateIdle
	m.sessions[id] = s
	return s
}

// Update updates a session.
func (m *Manager) Update(id string, fn func(*Session)) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return false
	}

	fn(s)
	s.LastActiveAt = time.Now()
	return true
}

// Delete removes a session.
func (m *Manager) Delete(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; !ok {
		return false
	}

	delete(m.sessions, id)
	return true
}

// List returns all sessions.
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// Count returns the number of sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// Cleanup removes stale sessions.
func (m *Manager) Cleanup(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, s := range m.sessions {
		if s.LastActiveAt.Before(cutoff) {
			delete(m.sessions, id)
			removed++
		}
	}

	return removed
}

// BuildSessionKey generates a session key from channel info.
func BuildSessionKey(channelType, chatID, threadID, userID string) string {
	key := channelType + ":" + chatID
	if threadID != "" {
		key += ":" + threadID
	}
	if userID != "" {
		key += ":" + userID
	}
	return key
}
