// Package tools provides agent tool implementations.
package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MemorySearchTool searches memory files for relevant content.
type MemorySearchTool struct {
	// AgentDir is the agent's working directory.
	AgentDir string
	// AgentSessionKey is the current session key.
	AgentSessionKey string
}

// NewMemorySearchTool creates a new memory search tool.
func NewMemorySearchTool(agentDir string) *MemorySearchTool {
	return &MemorySearchTool{
		AgentDir: agentDir,
	}
}

// Name returns the tool name.
func (t *MemorySearchTool) Name() string {
	return "memory_search"
}

// Description returns the tool description.
func (t *MemorySearchTool) Description() string {
	return `Semantically search MEMORY.md and memory/*.md files.
Mandatory recall step before answering questions about prior work, decisions, dates, people, preferences, or todos.
Returns top snippets with path and line numbers.`
}

// Parameters returns the JSON Schema for parameters.
func (t *MemorySearchTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
			"maxResults": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results (default: 5)",
			},
		},
		"required": []string{"query"},
	}
}

// MemorySearchResult represents a search result.
type MemorySearchResult struct {
	Path    string  `json:"path"`
	Line    int     `json:"line"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// MemorySearchResults represents the search results.
type MemorySearchResults struct {
	Results []MemorySearchResult `json:"results"`
	Query   string               `json:"query"`
	Count   int                  `json:"count"`
}

// Execute searches memory files.
func (t *MemorySearchTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	maxResults := 5
	if mr, ok := params["maxResults"].(float64); ok && mr > 0 {
		maxResults = int(mr)
	}

	agentDir := t.AgentDir
	if agentDir == "" {
		cwd, _ := os.Getwd()
		agentDir = cwd
	}

	// Find memory files
	memoryFiles := []string{}

	// Check MEMORY.md
	mainMemory := filepath.Join(agentDir, "MEMORY.md")
	if _, err := os.Stat(mainMemory); err == nil {
		memoryFiles = append(memoryFiles, mainMemory)
	}

	// Check memory/*.md
	memoryDir := filepath.Join(agentDir, "memory")
	if entries, err := os.ReadDir(memoryDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				memoryFiles = append(memoryFiles, filepath.Join(memoryDir, entry.Name()))
			}
		}
	}

	// Simple keyword search (semantic search would require embeddings)
	results := []MemorySearchResult{}
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	for _, filePath := range memoryFiles {
		matches := t.searchFile(filePath, queryWords, agentDir)
		results = append(results, matches...)
	}

	// Sort by score and limit
	// Simple sort by number of matching words
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return &MemorySearchResults{
		Results: results,
		Query:   query,
		Count:   len(results),
	}, nil
}

func (t *MemorySearchTool) searchFile(filePath string, queryWords []string, agentDir string) []MemorySearchResult {
	results := []MemorySearchResult{}

	file, err := os.Open(filePath)
	if err != nil {
		return results
	}
	defer func() { _ = file.Close() }()

	relPath, _ := filepath.Rel(agentDir, filePath)
	if relPath == "" {
		relPath = filePath
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lineLower := strings.ToLower(line)

		// Count matching words
		matchCount := 0
		for _, word := range queryWords {
			if strings.Contains(lineLower, word) {
				matchCount++
			}
		}

		if matchCount > 0 {
			score := float64(matchCount) / float64(len(queryWords))
			results = append(results, MemorySearchResult{
				Path:    relPath,
				Line:    lineNum,
				Content: line,
				Score:   score,
			})
		}
	}

	return results
}

// MemoryGetTool reads specific lines from memory files.
type MemoryGetTool struct {
	// AgentDir is the agent's working directory.
	AgentDir string
}

// NewMemoryGetTool creates a new memory get tool.
func NewMemoryGetTool(agentDir string) *MemoryGetTool {
	return &MemoryGetTool{
		AgentDir: agentDir,
	}
}

// Name returns the tool name.
func (t *MemoryGetTool) Name() string {
	return "memory_get"
}

// Description returns the tool description.
func (t *MemoryGetTool) Description() string {
	return `Read specific lines from MEMORY.md or memory/*.md files.
Use after memory_search to pull only the needed lines and keep context small.`
}

// Parameters returns the JSON Schema for parameters.
func (t *MemoryGetTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Relative path to memory file",
			},
			"from": map[string]interface{}{
				"type":        "integer",
				"description": "Starting line number (1-indexed)",
			},
			"lines": map[string]interface{}{
				"type":        "integer",
				"description": "Number of lines to read (default: 10)",
			},
		},
		"required": []string{"path"},
	}
}

// MemoryGetResult represents the get result.
type MemoryGetResult struct {
	Path       string `json:"path"`
	Text       string `json:"text"`
	FromLine   int    `json:"fromLine"`
	ToLine     int    `json:"toLine"`
	TotalLines int    `json:"totalLines"`
}

// Execute reads lines from a memory file.
func (t *MemoryGetTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	relPath, _ := params["path"].(string)
	if relPath == "" {
		return nil, fmt.Errorf("path is required")
	}

	from := 1
	if f, ok := params["from"].(float64); ok && f > 0 {
		from = int(f)
	}

	numLines := 10
	if l, ok := params["lines"].(float64); ok && l > 0 {
		numLines = int(l)
	}

	agentDir := t.AgentDir
	if agentDir == "" {
		cwd, _ := os.Getwd()
		agentDir = cwd
	}

	// Resolve path
	fullPath := relPath
	if !filepath.IsAbs(relPath) {
		fullPath = filepath.Join(agentDir, relPath)
	}

	// Validate it's a memory file
	if !strings.Contains(fullPath, "MEMORY.md") && !strings.Contains(fullPath, "memory/") {
		return nil, fmt.Errorf("path must be MEMORY.md or within memory/ directory")
	}

	// Read file
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Read lines
	scanner := bufio.NewScanner(file)
	lineNum := 0
	var lines []string
	totalLines := 0

	for scanner.Scan() {
		lineNum++
		totalLines = lineNum
		if lineNum >= from && lineNum < from+numLines {
			lines = append(lines, scanner.Text())
		}
	}

	return &MemoryGetResult{
		Path:       relPath,
		Text:       strings.Join(lines, "\n"),
		FromLine:   from,
		ToLine:     from + len(lines) - 1,
		TotalLines: totalLines,
	}, nil
}
