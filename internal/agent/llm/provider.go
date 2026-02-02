// Package llm provides LLM provider abstractions.
package llm

import (
	"context"
)

// Provider is the interface for LLM providers.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// Chat sends a chat completion request and returns the full response.
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// ChatStream sends a streaming chat completion request.
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error)

	// Models returns a list of available models.
	Models(ctx context.Context) ([]string, error)
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	Model        string    `json:"model"`
	Messages     []Message `json:"messages"`
	Tools        []ToolDef `json:"tools,omitempty"`
	SystemPrompt string    `json:"systemPrompt,omitempty"`
	MaxTokens    int       `json:"maxTokens,omitempty"`
	Temperature  float64   `json:"temperature,omitempty"`
	TopP         float64   `json:"topP,omitempty"`
	Stop         []string  `json:"stop,omitempty"`
}

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"toolCalls,omitempty"`
	FinishReason string     `json:"finishReason"`
	Usage        Usage      `json:"usage"`
}

// StreamChunk represents a streaming chunk.
type StreamChunk struct {
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	Done      bool       `json:"done"`
	Error     string     `json:"error,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"toolCalls,omitempty"`
	ToolCallID string     `json:"toolCallId,omitempty"`
}

// ToolDef represents a tool definition.
type ToolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"` // JSON Schema
}

// ToolCall represents an LLM's request to call a tool.
type ToolCall struct {
	ID           string                 `json:"id"`
	Index        int                    `json:"index"` // Used for streaming correlation
	Name         string                 `json:"name"`
	Arguments    map[string]interface{} `json:"arguments"`
	RawArguments string                 `json:"rawArguments,omitempty"` // Used for streaming accumulation
}

// Usage represents token usage.
type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}
