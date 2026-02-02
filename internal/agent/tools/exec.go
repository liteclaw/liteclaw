// Package tools provides agent tool implementations.
package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
)

// ExecTool executes shell commands.
type ExecTool struct {
	// DefaultTimeout is the default command timeout.
	DefaultTimeout time.Duration
	// AllowedDirs restricts command execution to these directories.
	AllowedDirs []string
	// SafeBins are commands that can be auto-approved.
	SafeBins []string
}

// NewExecTool creates a new exec tool.
func NewExecTool() *ExecTool {
	return &ExecTool{
		DefaultTimeout: 60 * time.Second,
		SafeBins:       []string{"ls", "cat", "head", "tail", "grep", "find", "wc", "pwd", "echo", "date"},
	}
}

// Name returns the tool name.
func (t *ExecTool) Name() string {
	return "exec"
}

// Description returns the tool description.
func (t *ExecTool) Description() string {
	return `Execute a shell command. Use for running commands, scripts, and interacting with the system.
The command runs in a bash shell. Use workdir to specify the working directory.
For long-running commands, set background=true to run asynchronously.`
}

// Parameters returns the JSON Schema for parameters.
func (t *ExecTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"workdir": map[string]interface{}{
				"type":        "string",
				"description": "Working directory for the command (optional)",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds (default: 60)",
			},
			"background": map[string]interface{}{
				"type":        "boolean",
				"description": "Run command in background (default: false)",
			},
		},
		"required": []string{"command"},
	}
}

// ExecResult represents the result of command execution.
type ExecResult struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Duration int64  `json:"durationMs"`
	TimedOut bool   `json:"timedOut,omitempty"`
}

// Execute runs the command.
func (t *ExecTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// SAFETY: Prevent sudo from blocking indefinitely for password.
	trimmedCmd := strings.TrimSpace(command)
	if strings.HasPrefix(trimmedCmd, "sudo") {
		if !strings.Contains(trimmedCmd, "-n") && !strings.Contains(trimmedCmd, "--non-interactive") {
			command = strings.Replace(command, "sudo", "sudo -n", 1)
		}
	}

	workdir, _ := params["workdir"].(string)
	if workdir == "" {
		workdir, _ = os.Getwd()
	}

	timeoutSec, _ := params["timeout"].(float64)
	timeout := t.DefaultTimeout
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}

	background, _ := params["background"].(bool)
	if background {
		// Start in background
		cmd := exec.Command("bash", "-c", command)
		cmd.Dir = workdir
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"pid":     cmd.Process.Pid,
			"status":  "running",
			"command": command,
		}, nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	// Execute command
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = workdir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &ExecResult{
		Command:  command,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start).Milliseconds(),
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = -1
		return result, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else if err != nil {
		return nil, err
	}

	return result, nil
}

// ExecPtyTool executes commands with PTY support for interactive commands.
type ExecPtyTool struct {
	ExecTool
}

// NewExecPtyTool creates a new PTY exec tool.
func NewExecPtyTool() *ExecPtyTool {
	return &ExecPtyTool{
		ExecTool: *NewExecTool(),
	}
}

// Execute runs the command with PTY.
func (t *ExecPtyTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// SAFETY: Prevent sudo from blocking indefinitely for password.
	trimmedCmd := strings.TrimSpace(command)
	if strings.HasPrefix(trimmedCmd, "sudo") {
		if !strings.Contains(trimmedCmd, "-n") && !strings.Contains(trimmedCmd, "--non-interactive") {
			command = strings.Replace(command, "sudo", "sudo -n", 1)
		}
	}

	workdir, _ := params["workdir"].(string)
	if workdir == "" {
		workdir, _ = os.Getwd()
	}

	timeoutSec, _ := params["timeout"].(float64)
	timeout := t.DefaultTimeout
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = workdir

	// Start with PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		// Fallback to regular exec
		return t.ExecTool.Execute(ctx, params)
	}
	defer ptmx.Close()

	// Read output
	var output bytes.Buffer
	done := make(chan error)
	go func() {
		_, err := output.ReadFrom(ptmx)
		done <- err
	}()

	// Wait for completion
	cmdErr := cmd.Wait()
	<-done

	result := &ExecResult{
		Command:  command,
		Stdout:   output.String(),
		Duration: time.Since(start).Milliseconds(),
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = -1
		return result, nil
	}

	if exitErr, ok := cmdErr.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	}

	return result, nil
}

// IsSafeBin checks if a command starts with a safe binary.
func (t *ExecTool) IsSafeBin(command string) bool {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}
	bin := parts[0]
	for _, safe := range t.SafeBins {
		if bin == safe {
			return true
		}
	}
	return false
}
