package mcp

import (
	"context"
	"fmt"
	"strings"
)

// MCPToolAdapter implements the tools.Tool interface for an MCP tool
type MCPToolAdapter struct {
	manager    *Manager
	serverName string
	tool       Tool
}

func NewToolAdapter(manager *Manager, serverName string, tool Tool) *MCPToolAdapter {
	return &MCPToolAdapter{
		manager:    manager,
		serverName: serverName,
		tool:       tool,
	}
}

func (a *MCPToolAdapter) Name() string {
	// We prefix the tool name with the server name to ensure uniqueness
	return fmt.Sprintf("mcp_%s_%s", a.serverName, a.tool.Name)
}

func (a *MCPToolAdapter) Description() string {
	return a.tool.Description
}

func (a *MCPToolAdapter) Parameters() interface{} {
	return a.tool.InputSchema
}

func (a *MCPToolAdapter) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	result, err := a.manager.CallTool(ctx, a.serverName, a.tool.Name, params)
	if err != nil {
		return nil, err
	}

	if result.IsError {
		var sb strings.Builder
		for _, content := range result.Content {
			sb.WriteString(content.Text)
		}
		return nil, fmt.Errorf("MCP execution error: %s", sb.String())
	}

	return result, nil
}
