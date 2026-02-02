// Package tools provides the tool framework for AI agents.
package tools

import (
	"context"
)

// Tool is the interface for agent tools.
type Tool interface {
	// Name returns the tool name (used by the LLM).
	Name() string

	// Description returns a description for the LLM.
	Description() string

	// Parameters returns the JSON Schema for tool parameters.
	Parameters() interface{}

	// Execute runs the tool with the given parameters.
	Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)
}

// Result represents a tool execution result.
type Result struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Text    string      `json:"text,omitempty"` // Human-readable output
}

// OK creates a successful result.
func OK(data interface{}) *Result {
	return &Result{Success: true, Data: data}
}

// OKText creates a successful result with text.
func OKText(text string) *Result {
	return &Result{Success: true, Text: text}
}

// Err creates an error result.
func Err(err error) *Result {
	return &Result{Success: false, Error: err.Error()}
}

// ErrText creates an error result with message.
func ErrText(msg string) *Result {
	return &Result{Success: false, Error: msg}
}

// Registry holds all registered tools.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// All returns all registered tools.
func (r *Registry) All() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// Names returns all tool names.
func (r *Registry) Names() []string {
	result := make([]string, 0, len(r.tools))
	for name := range r.tools {
		result = append(result, name)
	}
	return result
}

// NewDefaultRegistry creates a registry with all standard tools registered.
func NewDefaultRegistry(opts *RegistryOptions) *Registry {
	r := NewRegistry()

	// File system tools
	r.Register(NewReadTool())
	r.Register(NewWriteTool())
	r.Register(NewListTool())
	r.Register(NewEditTool())

	// Search tools
	r.Register(NewGrepTool())
	r.Register(NewFindTool())

	// Execution tools
	r.Register(NewExecTool())
	r.Register(NewProcessTool())

	// Web tools
	r.Register(NewWebSearchTool())
	r.Register(NewWebFetchTool())

	// Browser and UI tools
	r.Register(NewBrowserTool())
	r.Register(NewCanvasTool())
	r.Register(NewNodesTool())
	r.Register(NewTtsTool())

	// Media tools
	agentDir := ""
	var sender MessageSender
	if opts != nil {
		if opts.AgentDir != "" {
			agentDir = opts.AgentDir
		}
		sender = opts.Sender
	}
	r.Register(NewImageTool(agentDir))
	r.Register(NewMessageTool(sender))

	// Memory tools
	r.Register(NewMemorySearchTool(agentDir))
	r.Register(NewMemoryGetTool(agentDir))

	// Session tools
	r.Register(NewSessionsListTool())
	r.Register(NewSessionsSendTool())
	r.Register(NewSessionsSpawnTool())
	r.Register(NewSessionsHistoryTool())

	// System tools
	r.Register(NewAgentsListTool())
	r.Register(NewGatewayTool())
	r.Register(NewSessionStatusTool())

	return r
}

// RegistryOptions contains options for creating a default registry.
type RegistryOptions struct {
	// AgentDir is the agent's working directory.
	AgentDir string
	// AgentSessionKey is the current session key.
	AgentSessionKey string
	// Sender handles message delivery
	Sender MessageSender
}
