package gateway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/liteclaw/liteclaw/internal/config"
)

// SessionEntry represents metadata for a chat session.
type SessionEntry struct {
	SessionID     string `json:"sessionId"`
	Key           string `json:"key"`
	DisplayName   string `json:"displayName,omitempty"`
	Channel       string `json:"channel,omitempty"`
	ChatType      string `json:"chatType,omitempty"` // "direct" or "group"
	UpdatedAt     int64  `json:"updatedAt"`          // Unix timestamp in ms
	ThinkingLevel string `json:"thinkingLevel,omitempty"`
}

// Message represents a single chat message.
type Message struct {
	Role      string                   `json:"role"`
	Content   []map[string]interface{} `json:"content"`
	Timestamp int64                    `json:"timestamp"`
}

// TranscriptEntry is the structure used in .jsonl files.
type TranscriptEntry struct {
	Type      string   `json:"type"` // "message" or "session"
	ID        string   `json:"id,omitempty"`
	Timestamp string   `json:"timestamp"`
	Message   *Message `json:"message,omitempty"`
	Version   int      `json:"version,omitempty"`
}

// SessionManager handles session persistence.
type SessionManager struct {
	mu           sync.RWMutex
	baseDir      string
	sessionsFile string
	sessions     map[string]*SessionEntry
}

func NewSessionManager(baseDir string) *SessionManager {
	if baseDir == "" {
		baseDir = filepath.Join(config.StateDir(), "agents", "main", "sessions")
	}
	_ = os.MkdirAll(baseDir, 0755)

	sm := &SessionManager{
		baseDir:      baseDir,
		sessionsFile: filepath.Join(baseDir, "sessions.json"),
		sessions:     make(map[string]*SessionEntry),
	}
	sm.loadSessions()
	return sm
}

func (sm *SessionManager) loadSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	data, err := os.ReadFile(sm.sessionsFile)
	if err != nil {
		return
	}

	var sessions map[string]*SessionEntry
	if err := json.Unmarshal(data, &sessions); err == nil {
		sm.sessions = sessions
	}
}

func (sm *SessionManager) saveSessions() {
	data, err := json.MarshalIndent(sm.sessions, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(sm.sessionsFile, data, 0644)
}

func (sm *SessionManager) GetOrCreateSession(sessionKey string) *SessionEntry {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if entry, ok := sm.sessions[sessionKey]; ok {
		return entry
	}

	entry := &SessionEntry{
		SessionID:     uuid.New().String(),
		Key:           sessionKey,
		UpdatedAt:     time.Now().UnixMilli(),
		ThinkingLevel: "off",
	}
	sm.sessions[sessionKey] = entry
	sm.saveSessions()
	return entry
}

func (sm *SessionManager) AddMessage(sessionKey string, role string, text string) error {
	entry := sm.GetOrCreateSession(sessionKey)

	sm.mu.Lock()
	entry.UpdatedAt = time.Now().UnixMilli()
	sm.saveSessions()
	sm.mu.Unlock()

	msg := &Message{
		Role: role,
		Content: []map[string]interface{}{
			{
				"type": "text",
				"text": text,
			},
		},
		Timestamp: time.Now().UnixMilli(),
	}

	transcriptPath := filepath.Join(sm.baseDir, fmt.Sprintf("%s.jsonl", entry.SessionID))

	// Create header if file is new
	if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
		header := TranscriptEntry{
			Type:      "session",
			Version:   1,
			ID:        entry.SessionID,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		headerBytes, _ := json.Marshal(header)
		_ = os.WriteFile(transcriptPath, append(headerBytes, '\n'), 0644)
	}

	transcriptEntry := TranscriptEntry{
		Type:      "message",
		ID:        uuid.New().String()[:8],
		Timestamp: time.Now().Format(time.RFC3339),
		Message:   msg,
	}
	entryBytes, _ := json.Marshal(transcriptEntry)

	f, err := os.OpenFile(transcriptPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(entryBytes, '\n'))
	return err
}

func (sm *SessionManager) GetHistory(sessionKey string) ([]Message, error) {
	entry := sm.GetOrCreateSession(sessionKey)
	transcriptPath := filepath.Join(sm.baseDir, fmt.Sprintf("%s.jsonl", entry.SessionID))

	file, err := os.Open(transcriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Message{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var messages []Message
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var transcript TranscriptEntry
		if err := json.Unmarshal(scanner.Bytes(), &transcript); err != nil {
			continue
		}
		if transcript.Type == "message" && transcript.Message != nil {
			messages = append(messages, *transcript.Message)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

func (sm *SessionManager) ListSessions() []*SessionEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var list []*SessionEntry
	for _, s := range sm.sessions {
		list = append(list, s)
	}
	return list
}

// SessionCount returns the number of active sessions.
func (sm *SessionManager) SessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// ResolveDeliveryTarget finds the last active session to use as a default delivery target.
// It mimics the TS logic of looking up the "main" session (or effectively the last used session).
func (sm *SessionManager) ResolveDeliveryTarget() (channel string, target string, found bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var lastSession *SessionEntry
	var maxTime int64

	for _, s := range sm.sessions {
		// Ignore cron sessions or internal sessions if any
		if s.Key == "main" || len(s.Key) > 5 && s.Key[:5] == "cron:" {
			continue
		}

		// Simple heuristic: if key contains ":", explicitly parse it.
		// e.g. "telegram:123456"
		if s.UpdatedAt > maxTime {
			maxTime = s.UpdatedAt
			lastSession = s
		}
	}

	if lastSession != nil {
		// Parse key to get channel and target
		// Assuming format "channel:target"
		parts := splitKey(lastSession.Key)
		if len(parts) == 2 {
			return parts[0], parts[1], true
		}
	}

	return "", "", false
}

func splitKey(key string) []string {
	// Simple split helper
	// In real app, import strings
	// But simply:
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return nil
}
