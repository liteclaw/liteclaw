// Package test provides test utilities and helpers for LiteClaw tests.
package test

import (
	"context"
	"sync"

	"github.com/liteclaw/liteclaw/internal/agent/llm"
)

// MockLLMProvider is a mock implementation of llm.Provider for testing.
type MockLLMProvider struct {
	name      string
	responses []llm.ChatResponse
	mu        sync.Mutex
	callCount int
	requests  []llm.ChatRequest
}

// NewMockLLMProvider creates a new mock LLM provider.
func NewMockLLMProvider() *MockLLMProvider {
	return &MockLLMProvider{
		name:      "mock",
		responses: make([]llm.ChatResponse, 0),
		requests:  make([]llm.ChatRequest, 0),
	}
}

// Name returns the provider name.
func (p *MockLLMProvider) Name() string {
	return p.name
}

// SetResponse adds a response to the queue.
func (p *MockLLMProvider) SetResponse(resp llm.ChatResponse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.responses = append(p.responses, resp)
}

// SetTextResponse adds a simple text response.
func (p *MockLLMProvider) SetTextResponse(text string) {
	p.SetResponse(llm.ChatResponse{
		Content:      text,
		FinishReason: "stop",
	})
}

// SetToolCallResponse adds a tool call response.
func (p *MockLLMProvider) SetToolCallResponse(toolCalls []llm.ToolCall) {
	p.SetResponse(llm.ChatResponse{
		ToolCalls:    toolCalls,
		FinishReason: "tool_calls",
	})
}

// Chat returns the next queued response.
func (p *MockLLMProvider) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.callCount++
	p.requests = append(p.requests, *req)

	if len(p.responses) == 0 {
		return &llm.ChatResponse{
			Content:      "Mock response",
			FinishReason: "stop",
		}, nil
	}

	resp := p.responses[0]
	p.responses = p.responses[1:]
	return &resp, nil
}

// ChatStream returns a streaming response.
func (p *MockLLMProvider) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	p.mu.Lock()
	p.callCount++
	p.requests = append(p.requests, *req)

	var content string
	if len(p.responses) > 0 {
		content = p.responses[0].Content
		p.responses = p.responses[1:]
	} else {
		content = "Mock streaming response"
	}
	p.mu.Unlock()

	chunks := make(chan llm.StreamChunk, 10)

	go func() {
		defer close(chunks)

		// Split content into chunks
		for i := 0; i < len(content); i += 10 {
			end := i + 10
			if end > len(content) {
				end = len(content)
			}
			chunks <- llm.StreamChunk{Content: content[i:end]}
		}
		chunks <- llm.StreamChunk{Done: true}
	}()

	return chunks, nil
}

// Models returns mock models.
func (p *MockLLMProvider) Models(ctx context.Context) ([]string, error) {
	return []string{"mock-model-1", "mock-model-2"}, nil
}

// CallCount returns the number of calls made.
func (p *MockLLMProvider) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callCount
}

// Requests returns all recorded requests.
func (p *MockLLMProvider) Requests() []llm.ChatRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]llm.ChatRequest, len(p.requests))
	copy(result, p.requests)
	return result
}

// Reset clears recorded data.
func (p *MockLLMProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callCount = 0
	p.requests = make([]llm.ChatRequest, 0)
	p.responses = make([]llm.ChatResponse, 0)
}
