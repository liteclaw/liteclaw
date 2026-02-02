// Package agent provides the AI agent core for LiteClaw.
// This package handles LLM interactions, tool execution, and session management.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/liteclaw/liteclaw/internal/agent/llm"
	"github.com/liteclaw/liteclaw/internal/agent/policy"
	"github.com/liteclaw/liteclaw/internal/agent/tools"
	mcp "github.com/liteclaw/liteclaw/mcp"
)

// Agent represents an AI agent instance.
type Agent struct {
	ID              string
	Name            string
	Model           string
	Provider        llm.Provider
	Tools           []tools.Tool
	SystemPrompt    string
	Policy          policy.ToolPolicy
	Stream          bool
	MaxTokens       int
	Temperature     float64
	LogSystemPrompt bool
	MCPManager      *mcp.Manager
	Verbose         bool

	mu       sync.RWMutex
	sessions map[string]*Session
}

// Session represents an active agent session.
type Session struct {
	ID           string
	AgentID      string
	Messages     []Message
	ToolCalls    []ToolCall
	StartedAt    int64
	LastActiveAt int64
}

// Message represents a conversation message.
type Message struct {
	Role       string         `json:"role"` // "user", "assistant", "system", "tool"
	Content    string         `json:"content"`
	ToolCalls  []llm.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool invocation.
type ToolCall struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Params   map[string]interface{} `json:"params"`
	Result   interface{}            `json:"result"`
	Error    string                 `json:"error,omitempty"`
	Duration int64                  `json:"duration"` // milliseconds
}

// New creates a new Agent.
func New(id, name, model string, provider llm.Provider) *Agent {
	return &Agent{
		ID:       id,
		Name:     name,
		Model:    model,
		Provider: provider,
		Tools:    []tools.Tool{},
		sessions: make(map[string]*Session),
	}
}

// RegisterTools registers tools for this agent.
func (a *Agent) RegisterTools(t ...tools.Tool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Tools = append(a.Tools, t...)
}

// RegisterDefaultTools registers all standard tools with the agent.
func (a *Agent) RegisterDefaultTools(opts *tools.RegistryOptions) {
	registry := tools.NewDefaultRegistry(opts)
	a.RegisterTools(registry.All()...)
}

// RegisterToolsFromRegistry registers all tools from a registry.
func (a *Agent) RegisterToolsFromRegistry(registry *tools.Registry) {
	a.RegisterTools(registry.All()...)
}

// ExtractToolNames returns the names of all valid tools registered to the agent,
// filtered by the current ToolPolicy.
func (a *Agent) ExtractToolNames() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	matcher := a.Policy.Compile()
	var names []string
	for _, t := range a.Tools {
		if matcher(t.Name()) {
			names = append(names, t.Name())
		}
	}
	return names
}

// Run runs the agent with the given input message.
func (a *Agent) Run(ctx context.Context, sessionID string, input string) (<-chan StreamEvent, error) {
	events := make(chan StreamEvent, 100)

	go func() {
		defer close(events)

		// Get or create session
		session := a.getOrCreateSession(sessionID)

		// Add user message
		session.Messages = append(session.Messages, Message{
			Role:    "user",
			Content: input,
		})

		// Dynamic MCP Tool Selection
		var dynamicTools []tools.Tool
		systemPromptPrefix := a.SystemPrompt
		if a.MCPManager != nil {
			selected := a.MCPManager.SelectTools(input, 100)
			for _, st := range selected {
				adapter := mcp.NewToolAdapter(a.MCPManager, st.ServerName, st.Tool)
				dynamicTools = append(dynamicTools, adapter)
			}
			if len(selected) > 0 {
				systemPromptPrefix = fmt.Sprintf("%s\n\n[MCP Tools Available: %d dynamically selected for query relevance]", systemPromptPrefix, len(selected))
			}
		}

		const MaxTurns = 10
		for turn := 0; turn < MaxTurns; turn++ {
			// Build chat request
			reqTools := a.convertTools()
			// Add dynamic MCP tools
			for _, dt := range dynamicTools {
				reqTools = append(reqTools, llm.ToolDef{
					Name:        dt.Name(),
					Description: dt.Description(),
					Parameters:  dt.Parameters(),
				})
			}

			req := &llm.ChatRequest{
				Model:        a.Model,
				Messages:     a.convertMessages(session.Messages),
				Tools:        reqTools,
				SystemPrompt: systemPromptPrefix,
				MaxTokens:    a.MaxTokens,
				Temperature:  a.Temperature,
			}
			// Log Request
			if a.Verbose {
				if reqBytes, err := json.MarshalIndent(req.Messages, "", "  "); err == nil {
					if a.LogSystemPrompt {
						fmt.Printf("\n=== [LLM Request] Model: %s ===\nSystem Prompt:\n%s\n\nMessages:\n%s\n==================================\n", req.Model, req.SystemPrompt, string(reqBytes))
					} else {
						fmt.Printf("\n=== [LLM Request] Model: %s ===\nSystem Prompt: [HIDDEN]\n\nMessages:\n%s\n==================================\n", req.Model, string(reqBytes))
					}
				}
			}

			var fullResponse string
			var toolCalls []llm.ToolCall

			if a.Stream {
				// Stream response from LLM
				stream, err := a.Provider.ChatStream(ctx, req)
				if err != nil {
					events <- StreamEvent{Type: "error", Error: err.Error()}
					return
				}

				// We need to accumulate tool calls across chunks because they are streamed in fragments.
				// Map index -> ToolCall builder
				toolCallBuilders := make(map[int]*llm.ToolCall)

				// Process stream
				for chunk := range stream {
					if chunk.Error != "" {
						events <- StreamEvent{Type: "error", Error: chunk.Error}
						return
					}

					if chunk.Content != "" {
						fullResponse += chunk.Content
						// Ensure clean line endings for staircase effect prevention
						safeContent := strings.ReplaceAll(chunk.Content, "\n", "\r\n")
						events <- StreamEvent{Type: "text", Content: safeContent}
					}

					// Accumulate tool calls logic
					if len(chunk.ToolCalls) > 0 {
						for _, tc := range chunk.ToolCalls {
							idx := tc.Index
							builder, exists := toolCallBuilders[idx]
							if !exists {
								builder = &llm.ToolCall{
									Index: idx,
								}
								toolCallBuilders[idx] = builder
							}

							// Merge fields
							if tc.ID != "" {
								builder.ID = tc.ID
							}
							if tc.Name != "" {
								builder.Name = tc.Name
							}
							if tc.RawArguments != "" {
								builder.RawArguments += tc.RawArguments
							}

							// If this is the start of a new tool call, notify UI
							if tc.ID != "" {
								events <- StreamEvent{Type: "tool_call", ToolCall: builder}
							}
						}
					}
				}

				// Finalize Tool Calls from Builders
				maxIndex := -1
				for idx := range toolCallBuilders {
					if idx > maxIndex {
						maxIndex = idx
					}
				}

				for i := 0; i <= maxIndex; i++ {
					if builder, ok := toolCallBuilders[i]; ok {
						// Parse JSON arguments now that we have the full string
						var args map[string]interface{}
						if builder.RawArguments != "" {
							if err := json.Unmarshal([]byte(builder.RawArguments), &args); err != nil {
								args = map[string]interface{}{"raw": builder.RawArguments, "error": err.Error()}
							}
						}
						builder.Arguments = args
						toolCalls = append(toolCalls, *builder)
					}
				}
			} else {
				// Non-Streaming Implementation
				resp, err := a.Provider.Chat(ctx, req)
				if err != nil {
					events <- StreamEvent{Type: "error", Error: err.Error()}
					return
				}

				fullResponse = resp.Content
				toolCalls = resp.ToolCalls

				// Emit full text event
				if fullResponse != "" {
					events <- StreamEvent{Type: "text", Content: fullResponse}
				}
				// Emit tool call events for consistency
				for i := range toolCalls {
					// Need to pass a pointer to the tool call in the slice
					events <- StreamEvent{Type: "tool_call", ToolCall: &toolCalls[i]}
				}
			}

			// Log Response
			if a.Verbose {
				fmt.Printf("\n=== [LLM Response] ===\nText: %s\nToolCalls: %d\n======================\n", fullResponse, len(toolCalls))
			}

			// Append Assistant Message to History
			// FIX: Do not append empty messages
			if fullResponse != "" || len(toolCalls) > 0 {
				// Strip <think> tags from history to ensure cleaner context and avoid validation errors
				// especially with providers that might be strict about content/tool_call mix.
				historyContent := fullResponse
				if strings.Contains(historyContent, "<think>") {
					re := regexp.MustCompile(`(?s)<think>.*?</think>`)
					historyContent = re.ReplaceAllString(historyContent, "")
					historyContent = strings.TrimSpace(historyContent)
				}

				assistantMsg := Message{
					Role:      "assistant",
					Content:   historyContent,
					ToolCalls: toolCalls,
				}
				session.Messages = append(session.Messages, assistantMsg)
			} else {
				fmt.Println("Warning: Received empty response from LLM, skipping history append.")
			}

			// If no tool calls, we are done
			if len(toolCalls) == 0 {
				break
			}

			// Execute Tools
			for _, tc := range toolCalls {
				result, err := a.executeTool(ctx, tc)

				// Notify UI of result
				toolResult := ToolCallResult{ID: tc.ID, Result: result}
				if err != nil {
					toolResult.Error = err.Error()
				}
				events <- StreamEvent{Type: "tool_result", ToolResult: &toolResult}

				// Convert result to string for LLM
				var resultStr string
				if err != nil {
					resultStr = fmt.Sprintf("Error: %s", err.Error())
				} else {
					// Is result a string or object?
					if s, ok := result.(string); ok {
						resultStr = s
					} else if resultJSON, errJSON := json.Marshal(result); errJSON == nil {
						resultStr = string(resultJSON)
					} else {
						resultStr = fmt.Sprintf("%v", result)
					}
				}

				// Append Tool Message to History
				session.Messages = append(session.Messages, Message{
					Role:       "tool",
					Content:    resultStr,
					ToolCallID: tc.ID,
				})
			}
			// Loop continues to next turn -> sending history with tool results back to LLM
		}

		events <- StreamEvent{Type: "done"}
	}()

	return events, nil
}

func (a *Agent) getOrCreateSession(id string) *Session {
	a.mu.Lock()
	defer a.mu.Unlock()

	if session, ok := a.sessions[id]; ok {
		return session
	}

	session := &Session{
		ID:        id,
		AgentID:   a.ID,
		Messages:  []Message{},
		ToolCalls: []ToolCall{},
	}
	a.sessions[id] = session
	return session
}

func (a *Agent) convertMessages(msgs []Message) []llm.Message {
	result := make([]llm.Message, len(msgs))
	for i, m := range msgs {
		result[i] = llm.Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCalls:  m.ToolCalls,
			ToolCallID: m.ToolCallID,
		}
	}
	return result
}

func (a *Agent) convertTools() []llm.ToolDef {
	matcher := a.Policy.Compile()
	var result []llm.ToolDef
	for _, t := range a.Tools {
		if matcher(t.Name()) {
			result = append(result, llm.ToolDef{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			})
		}
	}
	return result
}

func (a *Agent) executeTool(ctx context.Context, tc llm.ToolCall) (interface{}, error) {
	// 1. Check registered tools
	for _, t := range a.Tools {
		if t.Name() == tc.Name {
			return t.Execute(ctx, tc.Arguments)
		}
	}

	// 2. Check for MCP tools if prefix matches
	if strings.HasPrefix(tc.Name, "mcp_") && a.MCPManager != nil {
		parts := strings.Split(tc.Name, "_")
		if len(parts) >= 3 {
			serverName := parts[1]
			toolName := strings.Join(parts[2:], "_")
			result, err := a.MCPManager.CallTool(ctx, serverName, toolName, tc.Arguments)
			if err != nil {
				return nil, err
			}
			return result, nil
		}
	}

	return nil, nil
}

// StreamEvent represents a streaming event from the agent.
type StreamEvent struct {
	Type       string          `json:"type"` // "text", "tool_call", "tool_result", "error", "done"
	Content    string          `json:"content,omitempty"`
	ToolCall   *llm.ToolCall   `json:"toolCall,omitempty"`
	ToolResult *ToolCallResult `json:"toolResult,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// ToolCallResult represents the result of a tool execution.
type ToolCallResult struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}
