package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// CachedTool represents a tool across all MCP servers
type CachedTool struct {
	ServerName string
	Tool       Tool
	MatchScore float64 // Used for selection
}

// Manager handles multiple MCP servers and tool selection
type Manager struct {
	configPath string
	servers    map[string]ServerConfig

	// Cache for tool discovery
	// In a real million-scale system, this would be a search engine or vector DB
	toolsCache []CachedTool

	activeClients map[string]*Client
	mu            sync.RWMutex

	Verbose bool
}

func NewManager(configPath string) *Manager {
	return &Manager{
		configPath:    configPath,
		servers:       make(map[string]ServerConfig),
		activeClients: make(map[string]*Client),
	}
}

// LoadConfig reads the mcpServers config from liteclaw.extras.json (or override).
func (m *Manager) LoadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var config MCPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	m.mu.Lock()
	m.servers = config.MCPServers
	m.mu.Unlock()

	return nil
}

// LoadToolsMetadata loads tool definitions from a metadata cache file
// This is essential for millions of servers where discovery is impossible at runtime
func (m *Manager) LoadToolsMetadata(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var metadata []CachedTool
	if err := json.Unmarshal(data, &metadata); err != nil {
		return err
	}

	m.mu.Lock()
	m.toolsCache = metadata
	m.mu.Unlock()
	return nil
}

// AddToolToCache manually adds a tool to the search index
func (m *Manager) AddToolToCache(serverName string, tool Tool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolsCache = append(m.toolsCache, CachedTool{
		ServerName: serverName,
		Tool:       tool,
	})
}

// DiscoverTools collects tool definitions from all servers
func (m *Manager) DiscoverTools(ctx context.Context) error {
	m.mu.RLock()
	serverCount := len(m.servers)
	m.mu.RUnlock()

	if m.Verbose {
		fmt.Printf("Discovering tools from %d MCP servers...\n", serverCount)
	}

	// Since we might have many servers, we process them in parallel
	// But we limit concurrency to avoid OS resource exhaustion
	concurrency := 10
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var allTools []CachedTool

	m.mu.RLock()
	for name, cfg := range m.servers {
		if !cfg.Enabled {
			continue
		}
		wg.Add(1)
		go func(name string, cfg ServerConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
			defer cancel()

			client := NewClient(cfg)
			client.Verbose = m.Verbose
			if err := client.Start(ctx); err != nil {
				fmt.Printf("Failed to start server %s: %v\n", name, err)
				return
			}
			defer func() { _ = client.Stop() }()

			var result ListToolsResult
			// Use empty struct to send "{}" instead of "null"
			if err := client.Call(ctx, "tools/list", struct{}{}, &result); err != nil {
				fmt.Printf("Failed to list tools for server %s: %v\n", name, err)
				return
			}

			mu.Lock()
			for _, t := range result.Tools {
				allTools = append(allTools, CachedTool{
					ServerName: name,
					Tool:       t,
				})
			}
			mu.Unlock()
		}(name, cfg)
	}
	m.mu.RUnlock()

	wg.Wait()
	if m.Verbose {
		fmt.Printf("Found %d tools across all servers.\n", len(allTools))
	}

	m.mu.Lock()
	m.toolsCache = allTools
	m.mu.Unlock()

	return nil
}

// SelectTools picks the top N tools based on a query (matching degree)
func (m *Manager) SelectTools(query string, limit int) []CachedTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.toolsCache) == 0 {
		return nil
	}

	// Score tools based on query
	scored := make([]CachedTool, len(m.toolsCache))
	copy(scored, m.toolsCache)

	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	for i := range scored {
		score := 0.0
		toolContent := strings.ToLower(scored[i].Tool.Name + " " + scored[i].Tool.Description)

		for _, word := range queryWords {
			if strings.Contains(toolContent, word) {
				score += 1.0
			}
		}

		// Boost exact name matches
		if strings.Contains(strings.ToLower(scored[i].Tool.Name), queryLower) {
			score += 5.0
		}

		scored[i].MatchScore = score
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].MatchScore > scored[j].MatchScore
	})

	// Return top N
	if len(scored) > limit {
		return scored[:limit]
	}
	return scored
}

// CallTool executes a tool on a specific MCP server
func (m *Manager) CallTool(ctx context.Context, serverName string, toolName string, args map[string]interface{}) (*CallToolResult, error) {
	m.mu.Lock()
	client, ok := m.activeClients[serverName]
	if !ok {
		cfg, exists := m.servers[serverName]
		if !exists {
			m.mu.Unlock()
			return nil, fmt.Errorf("server %s not found", serverName)
		}
		client = NewClient(cfg)
		if err := client.Start(ctx); err != nil {
			m.mu.Unlock()
			return nil, err
		}
		m.activeClients[serverName] = client
	}
	m.mu.Unlock()

	req := CallToolRequest{
		Name:      toolName,
		Arguments: args,
	}

	var result CallToolResult
	if err := client.Call(ctx, "tools/call", req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
