// Package llm provides LLM provider implementations.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	client  *openai.Client
	name    string
	Verbose bool
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	client := openai.NewClient(apiKey)
	return &OpenAIProvider{
		client: client,
		name:   "openai",
	}
}

// NewOpenAIProviderWithConfig creates an OpenAI provider with custom config.
func NewOpenAIProviderWithConfig(apiKey, baseURL string) *OpenAIProvider {
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}

	// Create HTTP client that bypasses proxy for local/internal networks
	config.HTTPClient = &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			Proxy: nil, // Bypass system proxy
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	client := openai.NewClientWithConfig(config)
	return &OpenAIProvider{
		client: client,
		name:   "openai",
	}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return p.name
}

// Chat sends a chat completion request.
func (p *OpenAIProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	messages := convertToOpenAIMessages(req.Messages, req.SystemPrompt)
	tools := convertToOpenAITools(req.Tools)

	chatReq := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
	}

	if len(tools) > 0 {
		chatReq.Tools = tools
		chatReq.ToolChoice = "auto"
	}

	// DEBUG: Log first 200 chars of last message for visibility
	if p.Verbose && len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		fmt.Printf("[LLM %s] Request last message: %s: %.100s...\n", req.Model, lastMsg.Role, lastMsg.Content)
	}

	if req.MaxTokens > 0 {
		chatReq.MaxTokens = req.MaxTokens
	}

	if req.Temperature > 0 {
		chatReq.Temperature = float32(req.Temperature)
	}

	resp, err := p.client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices returned")
	}

	choice := resp.Choices[0]

	result := &ChatResponse{
		Content:      choice.Message.Content,
		FinishReason: string(choice.FinishReason),
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	// Convert tool calls
	for _, tc := range choice.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: parseArguments(tc.Function.Arguments),
		})
	}

	// Support legacy function calls if tool calls are empty
	if len(result.ToolCalls) == 0 && choice.Message.FunctionCall != nil {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        "call_" + choice.Message.FunctionCall.Name,
			Name:      choice.Message.FunctionCall.Name,
			Arguments: parseArguments(choice.Message.FunctionCall.Arguments),
		})
	}

	return result, nil
}

// ChatStream sends a streaming chat completion request.
func (p *OpenAIProvider) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	messages := convertToOpenAIMessages(req.Messages, req.SystemPrompt)
	tools := convertToOpenAITools(req.Tools)

	chatReq := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   true,
	}

	if len(tools) > 0 {
		chatReq.Tools = tools
		chatReq.ToolChoice = "auto"
	}

	if req.MaxTokens > 0 {
		chatReq.MaxTokens = req.MaxTokens
	}

	if req.Temperature > 0 {
		chatReq.Temperature = float32(req.Temperature)
	}

	stream, err := p.client.CreateChatCompletionStream(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	chunks := make(chan StreamChunk, 100)

	go func() {
		defer close(chunks)
		defer func() { _ = stream.Close() }()

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				chunks <- StreamChunk{Done: true}
				return
			}
			if err != nil {
				chunks <- StreamChunk{Error: err.Error()}
				return
			}

			if len(resp.Choices) > 0 {
				delta := resp.Choices[0].Delta
				chunk := StreamChunk{
					Content: delta.Content,
				}

				// Handle tool calls in streaming
				for _, tc := range delta.ToolCalls {
					chunk.ToolCalls = append(chunk.ToolCalls, ToolCall{
						ID:           tc.ID,
						Index:        *tc.Index,
						Name:         tc.Function.Name,
						RawArguments: tc.Function.Arguments,
						// Arguments: nil, // Don't parse here, inconsistent
					})
				}

				chunks <- chunk
			}
		}
	}()

	return chunks, nil
}

// Models returns available models.
func (p *OpenAIProvider) Models(ctx context.Context) ([]string, error) {
	resp, err := p.client.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	var models []string
	for _, m := range resp.Models {
		models = append(models, m.ID)
	}
	return models, nil
}

func convertToOpenAIMessages(msgs []Message, systemPrompt string) []openai.ChatCompletionMessage {
	var result []openai.ChatCompletionMessage

	if systemPrompt != "" {
		result = append(result, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		})
	}

	for _, m := range msgs {
		msg := openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}
		if m.ToolCallID != "" {
			msg.ToolCallID = m.ToolCallID
		}

		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				// We need to re-serialize arguments for OpenAI
				// The ToolCall struct in this package has Arguments as map[string]interface{}
				// But openai.ToolCall expects Arguments as string (JSON)

				var argsStr string
				if tc.RawArguments != "" {
					argsStr = tc.RawArguments
				} else {
					argsBytes, _ := json.Marshal(tc.Arguments)
					argsStr = string(argsBytes)
				}

				msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: argsStr,
					},
				})
			}
		}

		result = append(result, msg)
	}

	return result
}

func convertToOpenAITools(tools []ToolDef) []openai.Tool {
	var result []openai.Tool
	for _, t := range tools {
		result = append(result, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return result
}

func parseArguments(args string) map[string]interface{} {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(args), &result); err != nil {
		return map[string]interface{}{
			"error": err.Error(),
			"raw":   args,
		}
	}
	return result
}
