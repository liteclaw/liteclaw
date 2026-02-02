// Package tools provides agent tool implementations.
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ReadTool reads file contents.
type ReadTool struct{}

// NewReadTool creates a new read tool.
func NewReadTool() *ReadTool {
	return &ReadTool{}
}

// Name returns the tool name.
func (t *ReadTool) Name() string {
	return "read"
}

// Description returns the tool description.
func (t *ReadTool) Description() string {
	return `Read the contents of a file. Supports text files.
For large files, use startLine and endLine to read specific sections.`
}

// Parameters returns the JSON Schema for parameters.
func (t *ReadTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Absolute path to the file to read",
			},
			"startLine": map[string]interface{}{
				"type":        "integer",
				"description": "Start line number (1-indexed, optional)",
			},
			"endLine": map[string]interface{}{
				"type":        "integer",
				"description": "End line number (1-indexed, optional)",
			},
		},
		"required": []string{"path"},
	}
}

// Execute reads the file.
func (t *ReadTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Expand path
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// TODO: Handle startLine/endLine for partial reads

	return map[string]interface{}{
		"path":    path,
		"content": string(content),
		"size":    len(content),
	}, nil
}

// WriteTool writes content to a file.
type WriteTool struct{}

// NewWriteTool creates a new write tool.
func NewWriteTool() *WriteTool {
	return &WriteTool{}
}

// Name returns the tool name.
func (t *WriteTool) Name() string {
	return "write"
}

// Description returns the tool description.
func (t *WriteTool) Description() string {
	return `Write content to a file. Creates the file if it doesn't exist.
Parent directories are created automatically if needed.`
}

// Parameters returns the JSON Schema for parameters.
func (t *WriteTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Absolute path to the file to write",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to write to the file",
			},
			"append": map[string]interface{}{
				"type":        "boolean",
				"description": "Append to file instead of overwriting (default: false)",
			},
		},
		"required": []string{"path", "content"},
	}
}

// Execute writes the file.
func (t *WriteTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	content, _ := params["content"].(string)
	appendMode, _ := params["append"].(bool)

	// Expand path
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// Write file
	flag := os.O_WRONLY | os.O_CREATE
	if appendMode {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	f, err := os.OpenFile(path, flag, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	n, err := f.WriteString(content)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"path":         path,
		"bytesWritten": n,
		"appended":     appendMode,
	}, nil
}

// ListTool lists directory contents.
type ListTool struct{}

// NewListTool creates a new list tool.
func NewListTool() *ListTool {
	return &ListTool{}
}

// Name returns the tool name.
func (t *ListTool) Name() string {
	return "list"
}

// Description returns the tool description.
func (t *ListTool) Description() string {
	return `List the contents of a directory.`
}

// Parameters returns the JSON Schema for parameters.
func (t *ListTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Absolute path to the directory to list",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "List recursively (default: false)",
			},
		},
		"required": []string{"path"},
	}
}

// FileEntry represents a file/directory entry.
type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size,omitempty"`
}

// Execute lists the directory.
func (t *ListTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Expand path
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []FileEntry
	for _, entry := range entries {
		info, _ := entry.Info()
		fe := FileEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(path, entry.Name()),
			IsDir: entry.IsDir(),
		}
		if info != nil && !entry.IsDir() {
			fe.Size = info.Size()
		}
		files = append(files, fe)
	}

	return map[string]interface{}{
		"path":    path,
		"entries": files,
		"count":   len(files),
	}, nil
}
