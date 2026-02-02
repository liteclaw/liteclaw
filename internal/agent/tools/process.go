// Package tools provides agent tool implementations.
package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// ProcessManager manages background processes.
type ProcessManager struct {
	mu        sync.RWMutex
	processes map[string]*ManagedProcess
}

// ManagedProcess represents a managed background process.
type ManagedProcess struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	PID       int       `json:"pid"`
	Status    string    `json:"status"` // running, done, error
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt,omitempty"`
	ExitCode  int       `json:"exitCode,omitempty"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`

	cmd *exec.Cmd
}

// Global process manager instance
var processManager = &ProcessManager{
	processes: make(map[string]*ManagedProcess),
}

// ProcessTool manages background processes.
type ProcessTool struct {
	// CleanupDuration is how long to keep completed processes.
	CleanupDuration time.Duration
}

// NewProcessTool creates a new process tool.
func NewProcessTool() *ProcessTool {
	return &ProcessTool{
		CleanupDuration: 30 * time.Minute,
	}
}

// Name returns the tool name.
func (t *ProcessTool) Name() string {
	return "process"
}

// Description returns the tool description.
func (t *ProcessTool) Description() string {
	return `Manage background processes started by the exec tool.
Actions:
- status: Check the status of a background process by ID or PID
- list: List all tracked background processes
- kill: Terminate a background process by ID or PID
- output: Get the output of a background process`
}

// Parameters returns the JSON Schema for parameters.
func (t *ProcessTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action to perform: status, list, kill, output",
				"enum":        []string{"status", "list", "kill", "output"},
			},
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Process ID (from exec background result)",
			},
			"pid": map[string]interface{}{
				"type":        "integer",
				"description": "System PID of the process",
			},
		},
		"required": []string{"action"},
	}
}

// ProcessListResult represents the list of processes.
type ProcessListResult struct {
	Processes []ProcessInfo `json:"processes"`
	Count     int           `json:"count"`
}

// ProcessInfo represents info about a process.
type ProcessInfo struct {
	ID        string `json:"id"`
	Command   string `json:"command"`
	PID       int    `json:"pid"`
	Status    string `json:"status"`
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt,omitempty"`
	ExitCode  int    `json:"exitCode,omitempty"`
}

// Execute performs the process action.
func (t *ProcessTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}

	switch action {
	case "list":
		return t.listProcesses()
	case "status":
		return t.getStatus(params)
	case "kill":
		return t.killProcess(params)
	case "output":
		return t.getOutput(params)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func (t *ProcessTool) listProcesses() (*ProcessListResult, error) {
	processManager.mu.RLock()
	defer processManager.mu.RUnlock()

	processes := make([]ProcessInfo, 0, len(processManager.processes))
	for _, p := range processManager.processes {
		info := ProcessInfo{
			ID:        p.ID,
			Command:   p.Command,
			PID:       p.PID,
			Status:    p.Status,
			StartedAt: p.StartedAt.Format(time.RFC3339),
			ExitCode:  p.ExitCode,
		}
		if !p.EndedAt.IsZero() {
			info.EndedAt = p.EndedAt.Format(time.RFC3339)
		}
		processes = append(processes, info)
	}

	return &ProcessListResult{
		Processes: processes,
		Count:     len(processes),
	}, nil
}

func (t *ProcessTool) findProcess(params map[string]interface{}) (*ManagedProcess, error) {
	id, _ := params["id"].(string)
	pid, _ := params["pid"].(float64)

	processManager.mu.RLock()
	defer processManager.mu.RUnlock()

	if id != "" {
		if p, ok := processManager.processes[id]; ok {
			return p, nil
		}
		return nil, fmt.Errorf("process not found: %s", id)
	}

	if pid > 0 {
		pidInt := int(pid)
		for _, p := range processManager.processes {
			if p.PID == pidInt {
				return p, nil
			}
		}
		return nil, fmt.Errorf("process not found with PID: %d", pidInt)
	}

	return nil, fmt.Errorf("id or pid is required")
}

func (t *ProcessTool) getStatus(params map[string]interface{}) (interface{}, error) {
	p, err := t.findProcess(params)
	if err != nil {
		return nil, err
	}

	// Update status if still running
	if p.Status == "running" && p.cmd != nil && p.cmd.Process != nil {
		proc, err := os.FindProcess(p.PID)
		if err == nil {
			err = proc.Signal(syscall.Signal(0))
			if err != nil {
				p.Status = "done"
				p.EndedAt = time.Now()
			}
		}
	}

	result := ProcessInfo{
		ID:        p.ID,
		Command:   p.Command,
		PID:       p.PID,
		Status:    p.Status,
		StartedAt: p.StartedAt.Format(time.RFC3339),
		ExitCode:  p.ExitCode,
	}
	if !p.EndedAt.IsZero() {
		result.EndedAt = p.EndedAt.Format(time.RFC3339)
	}

	return result, nil
}

func (t *ProcessTool) killProcess(params map[string]interface{}) (interface{}, error) {
	p, err := t.findProcess(params)
	if err != nil {
		return nil, err
	}

	if p.Status != "running" {
		return map[string]interface{}{
			"id":      p.ID,
			"status":  p.Status,
			"message": "process is not running",
		}, nil
	}

	proc, err := os.FindProcess(p.PID)
	if err != nil {
		return nil, fmt.Errorf("failed to find process: %w", err)
	}

	if err := proc.Kill(); err != nil {
		return nil, fmt.Errorf("failed to kill process: %w", err)
	}

	processManager.mu.Lock()
	p.Status = "killed"
	p.EndedAt = time.Now()
	processManager.mu.Unlock()

	return map[string]interface{}{
		"id":      p.ID,
		"pid":     p.PID,
		"status":  "killed",
		"message": "process killed",
	}, nil
}

func (t *ProcessTool) getOutput(params map[string]interface{}) (interface{}, error) {
	p, err := t.findProcess(params)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":     p.ID,
		"pid":    p.PID,
		"status": p.Status,
		"output": p.Output,
		"error":  p.Error,
		"length": len(p.Output),
	}, nil
}

// RegisterProcess registers a background process for tracking.
func RegisterProcess(id string, command string, cmd *exec.Cmd) {
	processManager.mu.Lock()
	defer processManager.mu.Unlock()

	processManager.processes[id] = &ManagedProcess{
		ID:        id,
		Command:   command,
		PID:       cmd.Process.Pid,
		Status:    "running",
		StartedAt: time.Now(),
		cmd:       cmd,
	}

	// Start goroutine to wait for completion
	go func() {
		err := cmd.Wait()
		processManager.mu.Lock()
		defer processManager.mu.Unlock()

		if p, ok := processManager.processes[id]; ok {
			p.EndedAt = time.Now()
			if err != nil {
				p.Status = "error"
				p.Error = err.Error()
				if exitErr, ok := err.(*exec.ExitError); ok {
					p.ExitCode = exitErr.ExitCode()
				}
			} else {
				p.Status = "done"
				p.ExitCode = 0
			}
		}
	}()
}

// GenerateProcessID generates a unique process ID.
func GenerateProcessID() string {
	return "proc_" + strconv.FormatInt(time.Now().UnixNano(), 36)
}
