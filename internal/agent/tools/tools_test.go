package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadToolExecute(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, LiteClaw!"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewReadTool()

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected map result")
	}

	if resultMap["content"] != content {
		t.Errorf("content = %v, want %v", resultMap["content"], content)
	}
}

func TestWriteToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "output.txt")
	content := "Test content"

	tool := NewWriteTool()

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    testFile,
		"content": content,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify file was written
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != content {
		t.Errorf("file content = %s, want %s", string(data), content)
	}
}

func TestListToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	// Create some files
	_ = os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("b"), 0644)
	_ = os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	tool := NewListTool()

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": tmpDir,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected map result")
	}

	count := resultMap["count"].(int)
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestExecToolName(t *testing.T) {
	tool := NewExecTool()
	if name := tool.Name(); name != "exec" {
		t.Errorf("Name() = %s, want exec", name)
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	registry := NewRegistry()
	tool := NewReadTool()

	registry.Register(tool)

	got, ok := registry.Get("read")
	if !ok {
		t.Fatal("Get() returned false")
	}

	if got.Name() != tool.Name() {
		t.Errorf("Get().Name() = %s, want %s", got.Name(), tool.Name())
	}
}

func TestDefaultRegistryHasAllTools(t *testing.T) {
	registry := NewDefaultRegistry(nil)

	expectedTools := []string{
		// File system tools
		"read", "write", "list", "edit",
		// Search tools
		"grep", "find",
		// Execution tools
		"exec", "process",
		// Web tools
		"web_search", "web_fetch",
		// Browser and UI tools
		"browser", "canvas", "nodes", "tts",
		// Media tools
		"image", "message",
		// Memory tools
		"memory_search", "memory_get",
		// Session tools
		"sessions_list", "sessions_send", "sessions_spawn", "sessions_history",
		// System tools
		"agents_list", "gateway", "session_status",
	}

	for _, name := range expectedTools {
		if _, ok := registry.Get(name); !ok {
			t.Errorf("Missing tool: %s", name)
		}
	}

	// Check total count
	all := registry.All()
	if len(all) != len(expectedTools) {
		t.Errorf("Registry has %d tools, expected %d", len(all), len(expectedTools))
	}
}

func TestEditToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "edit_test.txt")
	original := "Hello World"
	if err := os.WriteFile(testFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewEditTool()

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    testFile,
		"oldText": "World",
		"newText": "LiteClaw",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	editResult, ok := result.(*EditResult)
	if !ok {
		t.Fatal("Expected EditResult")
	}

	if !editResult.Replaced {
		t.Error("Expected replacement to succeed")
	}

	// Verify file was edited
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "Hello LiteClaw"
	if string(data) != expected {
		t.Errorf("file content = %s, want %s", string(data), expected)
	}
}

func TestWebSearchToolName(t *testing.T) {
	tool := NewWebSearchTool()
	if name := tool.Name(); name != "web_search" {
		t.Errorf("Name() = %s, want web_search", name)
	}
}

func TestWebFetchToolName(t *testing.T) {
	tool := NewWebFetchTool()
	if name := tool.Name(); name != "web_fetch" {
		t.Errorf("Name() = %s, want web_fetch", name)
	}
}

func TestBrowserToolName(t *testing.T) {
	tool := NewBrowserTool()
	if name := tool.Name(); name != "browser" {
		t.Errorf("Name() = %s, want browser", name)
	}
}

func TestCanvasToolName(t *testing.T) {
	tool := NewCanvasTool()
	if name := tool.Name(); name != "canvas" {
		t.Errorf("Name() = %s, want canvas", name)
	}
}

func TestNodesToolName(t *testing.T) {
	tool := NewNodesTool()
	if name := tool.Name(); name != "nodes" {
		t.Errorf("Name() = %s, want nodes", name)
	}
}

func TestTtsToolName(t *testing.T) {
	tool := NewTtsTool()
	if name := tool.Name(); name != "tts" {
		t.Errorf("Name() = %s, want tts", name)
	}
}

func TestCronToolActions(t *testing.T) {
	// Skip this test in simple suite as it requires heavy dependencies
	// or mock the dependencies later.
	t.Skip("Skipping CronTool test due to dependency injection requirements")
}

func TestSessionsListToolName(t *testing.T) {
	tool := NewSessionsListTool()
	if name := tool.Name(); name != "sessions_list" {
		t.Errorf("Name() = %s, want sessions_list", name)
	}
}

func TestProcessToolActions(t *testing.T) {
	tool := NewProcessTool()

	// Test list action
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "list",
	})
	if err != nil {
		t.Fatalf("list action error: %v", err)
	}

	listResult, ok := result.(*ProcessListResult)
	if !ok {
		t.Fatal("Expected ProcessListResult")
	}

	if listResult.Count < 0 {
		t.Error("Count should be non-negative")
	}
}
