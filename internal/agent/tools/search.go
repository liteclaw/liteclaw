// Package tools provides agent tool implementations.
package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// GrepTool searches for patterns in files.
type GrepTool struct{}

// NewGrepTool creates a new grep tool.
func NewGrepTool() *GrepTool {
	return &GrepTool{}
}

// Name returns the tool name.
func (t *GrepTool) Name() string {
	return "grep"
}

// Description returns the tool description.
func (t *GrepTool) Description() string {
	return `Search for text patterns in files or directories.
Uses ripgrep (rg) if available, otherwise falls back to built-in search.`
}

// Parameters returns the JSON Schema for parameters.
func (t *GrepTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Search pattern (regex supported)",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File or directory to search in",
			},
			"ignoreCase": map[string]interface{}{
				"type":        "boolean",
				"description": "Case-insensitive search (default: false)",
			},
			"maxResults": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results (default: 100)",
			},
		},
		"required": []string{"pattern", "path"},
	}
}

// GrepMatch represents a search match.
type GrepMatch struct {
	File       string `json:"file"`
	LineNumber int    `json:"lineNumber"`
	Line       string `json:"line"`
}

// Execute searches for the pattern.
func (t *GrepTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	pattern, _ := params["pattern"].(string)
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	path, _ := params["path"].(string)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	ignoreCase, _ := params["ignoreCase"].(bool)
	maxResults := 100
	if m, ok := params["maxResults"].(float64); ok && m > 0 {
		maxResults = int(m)
	}

	// Expand path
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	// Try ripgrep first
	if rgPath, err := exec.LookPath("rg"); err == nil {
		return t.searchWithRipgrep(ctx, rgPath, pattern, path, ignoreCase, maxResults)
	}

	// Fallback to built-in search
	return t.searchBuiltin(ctx, pattern, path, ignoreCase, maxResults)
}

func (t *GrepTool) searchWithRipgrep(ctx context.Context, rgPath, pattern, path string, ignoreCase bool, maxResults int) (interface{}, error) {
	args := []string{"--json", "-m", fmt.Sprintf("%d", maxResults)}
	if ignoreCase {
		args = append(args, "-i")
	}
	args = append(args, pattern, path)

	cmd := exec.CommandContext(ctx, rgPath, args...)
	output, err := cmd.Output()
	if err != nil {
		// rg returns exit code 1 for no matches
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return map[string]interface{}{"matches": []GrepMatch{}, "count": 0}, nil
		}
		return nil, err
	}

	// Parse ripgrep JSON output
	var matches []GrepMatch
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, `"type":"match"`) {
			// Simplified parsing - in production, use proper JSON parsing
			// This is a placeholder for full implementation
			matches = append(matches, GrepMatch{Line: line})
		}
	}

	return map[string]interface{}{"matches": matches, "count": len(matches)}, nil
}

func (t *GrepTool) searchBuiltin(ctx context.Context, pattern, path string, ignoreCase bool, maxResults int) (interface{}, error) {
	flags := ""
	if ignoreCase {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	var matches []GrepMatch

	err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}
		if len(matches) >= maxResults {
			return filepath.SkipAll
		}

		// Search in file
		f, err := os.Open(filePath)
		if err != nil {
			return nil
		}
		defer func() { _ = f.Close() }()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				matches = append(matches, GrepMatch{
					File:       filePath,
					LineNumber: lineNum,
					Line:       line,
				})
				if len(matches) >= maxResults {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return nil, err
	}

	return map[string]interface{}{"matches": matches, "count": len(matches)}, nil
}

// FindTool finds files by name pattern.
type FindTool struct{}

// NewFindTool creates a new find tool.
func NewFindTool() *FindTool {
	return &FindTool{}
}

// Name returns the tool name.
func (t *FindTool) Name() string {
	return "find"
}

// Description returns the tool description.
func (t *FindTool) Description() string {
	return `Find files by name pattern. Uses fd if available, otherwise os.Walk.`
}

// Parameters returns the JSON Schema for parameters.
func (t *FindTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Glob pattern to match file names",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Directory to search in",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Filter by type: file, directory, any (default: any)",
			},
			"maxDepth": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum directory depth to search",
			},
		},
		"required": []string{"pattern", "path"},
	}
}

// Execute finds files.
func (t *FindTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	pattern, _ := params["pattern"].(string)
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	searchPath, _ := params["path"].(string)
	if searchPath == "" {
		return nil, fmt.Errorf("path is required")
	}

	fileType, _ := params["type"].(string)
	maxDepth := -1
	if d, ok := params["maxDepth"].(float64); ok && d > 0 {
		maxDepth = int(d)
	}

	// Expand path
	if searchPath[0] == '~' {
		home, _ := os.UserHomeDir()
		searchPath = filepath.Join(home, searchPath[1:])
	}

	var files []string
	baseDepth := strings.Count(searchPath, string(os.PathSeparator))

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Check depth
		if maxDepth >= 0 {
			depth := strings.Count(path, string(os.PathSeparator)) - baseDepth
			if depth > maxDepth {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Check type
		switch fileType {
		case "file":
			if info.IsDir() {
				return nil
			}
		case "directory":
			if !info.IsDir() {
				return nil
			}
		}

		// Check pattern
		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{"files": files, "count": len(files)}, nil
}
