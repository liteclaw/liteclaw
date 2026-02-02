// Package tools provides agent tool implementations.
package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GatewayClient defines the interface for gateway communication.
// This allows tools to optionally use actual gateway calls.
type GatewayClient interface {
	ListSessions(ctx context.Context, params map[string]interface{}) ([]map[string]interface{}, error)
	GetSessionHistory(ctx context.Context, sessionKey string, limit int) ([]map[string]interface{}, error)
	SendMessage(ctx context.Context, sessionKey, message string, opts map[string]interface{}) (map[string]interface{}, error)
	WaitForRun(ctx context.Context, runID string, timeoutMs int) (map[string]interface{}, error)
}

// SessionsListTool lists available sessions.
type SessionsListTool struct {
	// AgentSessionKey is the current agent's session key.
	AgentSessionKey string
	// Gateway is the optional gateway client.
	Gateway GatewayClient
}

// NewSessionsListTool creates a new sessions list tool.
func NewSessionsListTool() *SessionsListTool {
	return &SessionsListTool{}
}

// Name returns the tool name.
func (t *SessionsListTool) Name() string {
	return "sessions_list"
}

// Description returns the tool description.
func (t *SessionsListTool) Description() string {
	return `List available agent sessions.
Shows session keys, labels, status, and metadata.
Use to discover sessions for inter-agent communication.`
}

// Parameters returns the JSON Schema for parameters.
func (t *SessionsListTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"includeGlobal": map[string]interface{}{
				"type":        "boolean",
				"description": "Include global sessions (default: true)",
			},
			"agentId": map[string]interface{}{
				"type":        "string",
				"description": "Filter by agent ID",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of sessions to return (default: 50)",
			},
		},
	}
}

// SessionInfo represents session information.
type SessionInfo struct {
	Key       string `json:"key"`
	Label     string `json:"label,omitempty"`
	AgentID   string `json:"agentId,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// SessionsListResult represents the sessions list result.
type SessionsListResult struct {
	Sessions []SessionInfo `json:"sessions"`
	Count    int           `json:"count"`
}

// Execute lists sessions.
func (t *SessionsListTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// If gateway client is available, use it
	if t.Gateway != nil {
		callParams := map[string]interface{}{}
		if includeGlobal, ok := params["includeGlobal"].(bool); ok {
			callParams["includeGlobal"] = includeGlobal
		}
		if agentID, ok := params["agentId"].(string); ok && agentID != "" {
			callParams["agentId"] = agentID
		}
		if limit, ok := params["limit"].(float64); ok && limit > 0 {
			callParams["limit"] = int(limit)
		}

		sessions, err := t.Gateway.ListSessions(ctx, callParams)
		if err != nil {
			return nil, fmt.Errorf("gateway error: %w", err)
		}

		result := &SessionsListResult{
			Sessions: make([]SessionInfo, 0, len(sessions)),
			Count:    len(sessions),
		}
		for _, s := range sessions {
			info := SessionInfo{
				Key:    fmt.Sprint(s["key"]),
				Status: "active",
			}
			if label, ok := s["label"].(string); ok {
				info.Label = label
			}
			if agentID, ok := s["agentId"].(string); ok {
				info.AgentID = agentID
			}
			result.Sessions = append(result.Sessions, info)
		}
		return result, nil
	}

	// Fallback: return empty list
	return &SessionsListResult{
		Sessions: []SessionInfo{},
		Count:    0,
	}, nil
}

// SessionsSendTool sends messages to other sessions.
type SessionsSendTool struct {
	// AgentSessionKey is the current agent's session key.
	AgentSessionKey string
	// AgentChannel is the messaging channel.
	AgentChannel string
}

// NewSessionsSendTool creates a new sessions send tool.
func NewSessionsSendTool() *SessionsSendTool {
	return &SessionsSendTool{}
}

// Name returns the tool name.
func (t *SessionsSendTool) Name() string {
	return "sessions_send"
}

// Description returns the tool description.
func (t *SessionsSendTool) Description() string {
	return `Send a message into another session.
Use sessionKey or label to identify the target.
Enables inter-agent communication and coordination.`
}

// Parameters returns the JSON Schema for parameters.
func (t *SessionsSendTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"sessionKey": map[string]interface{}{
				"type":        "string",
				"description": "Target session key",
			},
			"label": map[string]interface{}{
				"type":        "string",
				"description": "Target session label (alternative to sessionKey)",
			},
			"agentId": map[string]interface{}{
				"type":        "string",
				"description": "Target agent ID (used with label)",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Message to send",
			},
			"timeoutSeconds": map[string]interface{}{
				"type":        "integer",
				"description": "Wait timeout in seconds (0 = fire and forget)",
			},
		},
		"required": []string{"message"},
	}
}

// SessionsSendResult represents the send result.
type SessionsSendResult struct {
	RunID      string `json:"runId"`
	Status     string `json:"status"`
	SessionKey string `json:"sessionKey,omitempty"`
	Reply      string `json:"reply,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Execute sends to a session.
func (t *SessionsSendTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}

	sessionKey, _ := params["sessionKey"].(string)
	label, _ := params["label"].(string)

	if sessionKey == "" && label == "" {
		return nil, fmt.Errorf("sessionKey or label is required")
	}

	// Note: This would integrate with the gateway to send messages
	// For now, return a placeholder
	return &SessionsSendResult{
		RunID:      uuid.New().String(),
		Status:     "pending",
		SessionKey: sessionKey,
	}, nil
}

// SessionsSpawnTool spawns new agent sessions.
type SessionsSpawnTool struct {
	// AgentSessionKey is the current agent's session key.
	AgentSessionKey string
	// AgentChannel is the messaging channel.
	AgentChannel string
}

// NewSessionsSpawnTool creates a new sessions spawn tool.
func NewSessionsSpawnTool() *SessionsSpawnTool {
	return &SessionsSpawnTool{}
}

// Name returns the tool name.
func (t *SessionsSpawnTool) Name() string {
	return "sessions_spawn"
}

// Description returns the tool description.
func (t *SessionsSpawnTool) Description() string {
	return `Spawn a new agent session (subagent).
Creates an isolated session for parallel task execution.
The spawned session runs independently and can be communicated with via sessions_send.`
}

// Parameters returns the JSON Schema for parameters.
func (t *SessionsSpawnTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agentId": map[string]interface{}{
				"type":        "string",
				"description": "Agent ID for the spawned session",
			},
			"label": map[string]interface{}{
				"type":        "string",
				"description": "Label for the spawned session",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Initial message/task for the spawned agent",
			},
			"model": map[string]interface{}{
				"type":        "string",
				"description": "Model to use (provider/model format)",
			},
			"systemPrompt": map[string]interface{}{
				"type":        "string",
				"description": "Custom system prompt for the spawned agent",
			},
		},
		"required": []string{"message"},
	}
}

// SessionsSpawnResult represents the spawn result.
type SessionsSpawnResult struct {
	RunID      string `json:"runId"`
	SessionKey string `json:"sessionKey"`
	Label      string `json:"label,omitempty"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// Execute spawns a session.
func (t *SessionsSpawnTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}

	label, _ := params["label"].(string)
	agentID, _ := params["agentId"].(string)

	// Note: This would integrate with the gateway to spawn sessions
	// For now, return a placeholder
	sessionKey := fmt.Sprintf("spawn_%s_%d", agentID, time.Now().UnixNano())

	return &SessionsSpawnResult{
		RunID:      uuid.New().String(),
		SessionKey: sessionKey,
		Label:      label,
		Status:     "spawned",
	}, nil
}

// SessionsHistoryTool retrieves session history.
type SessionsHistoryTool struct {
	// AgentSessionKey is the current agent's session key.
	AgentSessionKey string
}

// NewSessionsHistoryTool creates a new sessions history tool.
func NewSessionsHistoryTool() *SessionsHistoryTool {
	return &SessionsHistoryTool{}
}

// Name returns the tool name.
func (t *SessionsHistoryTool) Name() string {
	return "sessions_history"
}

// Description returns the tool description.
func (t *SessionsHistoryTool) Description() string {
	return `Retrieve message history from a session.
Use to review past conversation context.`
}

// Parameters returns the JSON Schema for parameters.
func (t *SessionsHistoryTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"sessionKey": map[string]interface{}{
				"type":        "string",
				"description": "Target session key",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum messages to retrieve (default: 20)",
			},
		},
		"required": []string{"sessionKey"},
	}
}

// HistoryMessage represents a message in history.
type HistoryMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

// SessionsHistoryResult represents the history result.
type SessionsHistoryResult struct {
	SessionKey string           `json:"sessionKey"`
	Messages   []HistoryMessage `json:"messages"`
	Count      int              `json:"count"`
}

// Execute retrieves session history.
func (t *SessionsHistoryTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sessionKey, _ := params["sessionKey"].(string)
	if sessionKey == "" {
		return nil, fmt.Errorf("sessionKey is required")
	}

	// Note: This would integrate with the gateway to get history
	// For now, return a placeholder
	return &SessionsHistoryResult{
		SessionKey: sessionKey,
		Messages:   []HistoryMessage{},
		Count:      0,
	}, nil
}
