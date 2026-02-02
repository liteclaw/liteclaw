package agent

import (
	"context"
	"testing"

	"github.com/liteclaw/liteclaw/internal/agent/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockProvider is a mock implementation of llm.Provider
type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockProvider) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*llm.ChatResponse), args.Error(1)
}

func (m *MockProvider) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(<-chan llm.StreamChunk), args.Error(1)
}

func (m *MockProvider) Models(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func TestAgent_Init(t *testing.T) {
	p := new(MockProvider)
	a := New("test-agent", "LiteClaw", "test-model", p)

	assert.Equal(t, "test-agent", a.ID)
	assert.Equal(t, "LiteClaw", a.Name)
	assert.Equal(t, "test-model", a.Model)
	assert.Equal(t, p, a.Provider)
}

func TestAgent_Run_Simple(t *testing.T) {
	p := new(MockProvider)
	a := New("test-agent", "LiteClaw", "test-model", p)
	a.Stream = true

	ctx := context.Background()
	sessionID := "session-1"
	message := "Hello"

	// Mock response
	ch := make(chan llm.StreamChunk, 2)
	ch <- llm.StreamChunk{Content: "Hi there!"}
	ch <- llm.StreamChunk{Done: true}
	close(ch)

	p.On("ChatStream", ctx, mock.Anything).Return((<-chan llm.StreamChunk)(ch), nil)

	events, err := a.Run(ctx, sessionID, message)
	require.NoError(t, err)

	var fullResponse string
	for evt := range events {
		if evt.Type == "text" {
			fullResponse += evt.Content
		}
	}

	assert.Equal(t, "Hi there!", fullResponse)
}

type toolMock struct {
	mock.Mock
}

func (m *toolMock) Name() string            { return "test_tool" }
func (m *toolMock) Description() string     { return "A test tool" }
func (m *toolMock) Parameters() interface{} { return nil }
func (m *toolMock) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	callArgs := m.Called(ctx, args)
	return callArgs.Get(0), callArgs.Error(1)
}

func TestAgent_ToolExecution(t *testing.T) {
	p := new(MockProvider)
	a := New("test-agent", "LiteClaw", "test-model", p)
	a.Stream = true

	tm := new(toolMock)
	a.RegisterTools(tm)

	ctx := context.Background()
	sessionID := "session-tool"
	message := "Call the tool"

	// 1. LLM requests tool call
	ch1 := make(chan llm.StreamChunk, 2)
	ch1 <- llm.StreamChunk{
		ToolCalls: []llm.ToolCall{
			{ID: "call-1", Name: "test_tool", RawArguments: `{"arg": "val"}`},
		},
	}
	ch1 <- llm.StreamChunk{Done: true}
	close(ch1)
	p.On("ChatStream", ctx, mock.Anything).Return((<-chan llm.StreamChunk)(ch1), nil).Once()

	// 2. Tool execution mocked
	tm.On("Execute", mock.Anything, map[string]interface{}{"arg": "val"}).Return("Tool Result", nil)

	// 3. LLM receives tool result and responds
	// We use .Run to avoid signature confusion and .Return for values
	ch2 := make(chan llm.StreamChunk, 2)
	p.On("ChatStream", ctx, mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(1).(*llm.ChatRequest)
		foundResult := false
		for _, msg := range req.Messages {
			if msg.Role == "tool" && msg.Content == "Tool Result" {
				foundResult = true
				break
			}
		}

		if foundResult {
			ch2 <- llm.StreamChunk{Content: "Finished!"}
		} else {
			ch2 <- llm.StreamChunk{Content: "Tool result missing!"}
		}
		ch2 <- llm.StreamChunk{Done: true}
		close(ch2)
	}).Return((<-chan llm.StreamChunk)(ch2), nil).Once()

	events, err := a.Run(ctx, sessionID, message)
	require.NoError(t, err)

	var fullResponse string
	for evt := range events {
		if evt.Type == "text" {
			fullResponse += evt.Content
		}
	}

	assert.Equal(t, "Finished!", fullResponse)
}
