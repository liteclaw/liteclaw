// Package tools provides agent tool implementations.
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditTool performs incremental file edits.
type EditTool struct{}

// NewEditTool creates a new edit tool.
func NewEditTool() *EditTool {
	return &EditTool{}
}

// Name returns the tool name.
func (t *EditTool) Name() string {
	return "edit"
}

// Description returns the tool description.
func (t *EditTool) Description() string {
	return `Edit a file by replacing specific text with new content.
Use for making targeted changes to existing files without rewriting the entire file.
The oldText must exactly match the content to be replaced (including whitespace).
For creating new files, use the write tool instead.`
}

// Parameters returns the JSON Schema for parameters.
func (t *EditTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Absolute path to the file to edit",
			},
			"oldText": map[string]interface{}{
				"type":        "string",
				"description": "The exact text to find and replace (must match exactly)",
			},
			"newText": map[string]interface{}{
				"type":        "string",
				"description": "The new text to replace oldText with",
			},
			"dryRun": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, show what would change without actually modifying the file",
			},
		},
		"required": []string{"path", "oldText", "newText"},
	}
}

// EditResult represents the result of an edit operation.
type EditResult struct {
	Path        string `json:"path"`
	Replaced    bool   `json:"replaced"`
	Occurrences int    `json:"occurrences"`
	DryRun      bool   `json:"dryRun,omitempty"`
	Diff        string `json:"diff,omitempty"`
}

// Execute performs the edit.
func (t *EditTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	oldText, _ := params["oldText"].(string)
	if oldText == "" {
		return nil, fmt.Errorf("oldText is required")
	}

	newText, _ := params["newText"].(string)
	dryRun, _ := params["dryRun"].(bool)

	// Expand path
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	// Read existing content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)

	// Count occurrences
	occurrences := strings.Count(contentStr, oldText)
	if occurrences == 0 {
		return &EditResult{
			Path:        path,
			Replaced:    false,
			Occurrences: 0,
		}, nil
	}

	// Perform replacement
	newContent := strings.ReplaceAll(contentStr, oldText, newText)

	result := &EditResult{
		Path:        path,
		Replaced:    true,
		Occurrences: occurrences,
		DryRun:      dryRun,
	}

	if dryRun {
		// Generate a simple diff preview
		result.Diff = fmt.Sprintf("Would replace %d occurrence(s):\n- %s\n+ %s",
			occurrences,
			truncateForDiff(oldText, 200),
			truncateForDiff(newText, 200))
		return result, nil
	}

	// Write the modified content
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return result, nil
}

// truncateForDiff truncates a string for diff display.
func truncateForDiff(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
