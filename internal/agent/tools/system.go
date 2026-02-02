// Package tools provides agent tool implementations.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/liteclaw/liteclaw/internal/cron"
)

// CronTool manages scheduled tasks.
type CronTool struct {
	// AgentSessionKey is the current agent's session key.
	AgentSessionKey string
	Scheduler       *cron.Scheduler
}

// NewCronTool creates a new cron tool.
func NewCronTool(s *cron.Scheduler) *CronTool {
	return &CronTool{
		Scheduler: s,
	}
}

// Name returns the tool name.
func (t *CronTool) Name() string {
	return "cron"
}

// Description ...
func (t *CronTool) Description() string {
	return `Manage Gateway cron jobs (status/list/add/update/remove/run/runs) and send wake events.

ACTIONS:
- status: Check cron scheduler status
- list: List jobs (use includeDisabled:true to include disabled)
- add: Create job (requires job object, see schema below)
- update: Modify job (requires jobId + patch object)
- remove: Delete job (requires jobId)
- run: Trigger job immediately (requires jobId)
- runs: Get job run history (requires jobId)
- wake: Send wake event (requires text)

JOB SCHEMA (for add action):
{
  "name": "string (optional)",
  "schedule": {
    "kind": "at" | "every" | "cron",
    "atMs": <number>,     // if kind="at"
    "everyMs": <number>,  // if kind="every"
    "expr": "<string>"    // if kind="cron"
  },
  "payload": {
    "kind": "systemEvent" | "agentTurn",
    "text": "string",     // if kind="systemEvent"
    "message": "string"   // if kind="agentTurn"
  },
  "sessionTarget": "main" | "isolated",
  "enabled": boolean (optional, default true)
}

CRITICAL CONSTRAINTS:
- sessionTarget="main" REQUIRES payload.kind="systemEvent"
- sessionTarget="isolated" REQUIRES payload.kind="agentTurn"`
}

// Parameters ...
func (t *CronTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action: status, list, add, update, remove, run, runs, wake",
				"enum":        []string{"status", "list", "add", "update", "remove", "run", "runs", "wake"},
			},
			"job": map[string]interface{}{
				"type":        "object",
				"description": "Job definition for add action",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"schedule": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"kind":    map[string]interface{}{"type": "string", "enum": []string{"at", "every", "cron"}},
							"atMs":    map[string]interface{}{"type": "number"},
							"everyMs": map[string]interface{}{"type": "number"},
							"expr":    map[string]interface{}{"type": "string"},
						},
						"required": []string{"kind"},
					},
					"payload": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"kind":    map[string]interface{}{"type": "string", "enum": []string{"systemEvent", "agentTurn"}},
							"text":    map[string]interface{}{"type": "string"},
							"message": map[string]interface{}{"type": "string"},
						},
						"required": []string{"kind"},
					},
					"sessionTarget": map[string]interface{}{"type": "string", "enum": []string{"main", "isolated"}},
					"enabled":       map[string]interface{}{"type": "boolean"},
				},
				"required": []string{"schedule", "payload", "sessionTarget"},
			},
			"jobId": map[string]interface{}{
				"type":        "string",
				"description": "Job ID for update/remove/run/runs actions",
			},
			"patch": map[string]interface{}{
				"type":        "object",
				"description": "Patch object for update action",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text for wake action",
			},
			"includeDisabled": map[string]interface{}{
				"type":        "boolean",
				"description": "Include disabled jobs in list",
			},
		},
		"required": []string{"action"},
	}
}

// CronResult ...
type CronResult struct {
	Action string      `json:"action"`
	Status string      `json:"status"`
	Jobs   []*cron.Job `json:"jobs,omitempty"`
	Job    *cron.Job   `json:"job,omitempty"`
	Error  string      `json:"error,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

// Execute performs the cron action.
func (t *CronTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}

	if t.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	// Alias common hallucinations
	if action == "set" || action == "create" {
		action = "add"
	}

	switch action {
	case "status":
		return &CronResult{
			Action: action,
			Status: "ok",
			Data:   map[string]interface{}{"running": t.Scheduler.IsRunning()},
		}, nil

	case "list":
		includeDisabled, _ := params["includeDisabled"].(bool)
		allJobs := t.Scheduler.Jobs()
		jobs := make([]*cron.Job, 0)
		for _, j := range allJobs {
			if j.Enabled || includeDisabled {
				jobs = append(jobs, j)
			}
		}
		return &CronResult{
			Action: action,
			Status: "ok",
			Jobs:   jobs,
		}, nil

	case "add":
		jobArg, ok := params["job"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("job object required for add")
		}

		// Marshal and unmarshal into cron.Job for clean parsing
		jobJSON, err := json.Marshal(jobArg)
		if err != nil {
			return nil, fmt.Errorf("failed to encode job: %v", err)
		}

		var job cron.Job
		if err := json.Unmarshal(jobJSON, &job); err != nil {
			return nil, fmt.Errorf("failed to parse job: %v", err)
		}

		// Validation
		if job.Schedule.Kind == "" {
			return nil, fmt.Errorf("schedule.kind required")
		}
		if job.Payload.Kind == "" {
			return nil, fmt.Errorf("payload.kind required")
		}
		if job.SessionTarget == "" {
			return nil, fmt.Errorf("sessionTarget required")
		}

		// Constraints
		if job.SessionTarget == "main" && job.Payload.Kind != cron.PayloadKindSystemEvent {
			return nil, fmt.Errorf("sessionTarget='main' requires payload.kind='systemEvent'")
		}
		if job.SessionTarget == "isolated" && job.Payload.Kind != cron.PayloadKindAgentTurn {
			return nil, fmt.Errorf("sessionTarget='isolated' requires payload.kind='agentTurn'")
		}

		// Normalize schedule: if 5 fields, prepend "0 " to make it 6 fields (seconds)
		// robfig/cron with Seconds requires 6 fields.
		if job.Schedule.Kind == cron.ScheduleKindCron {
			fields := strings.Fields(job.Schedule.Expr)
			if len(fields) == 5 {
				job.Schedule.Expr = "0 " + job.Schedule.Expr
			}
		}

		// Assign ID if missing
		if job.ID == "" {
			job.ID = uuid.New().String()
		}

		// Defaults
		if _, ok := jobArg["enabled"]; !ok { // Check if 'enabled' was explicitly provided
			job.Enabled = true
		}

		if err := t.Scheduler.AddJob(&job); err != nil {
			return nil, fmt.Errorf("failed to register job: %v", err)
		}

		return &CronResult{
			Action: action,
			Status: "created",
			Job:    &job,
		}, nil

	case "update":
		id, _ := params["jobId"].(string)
		if id == "" {
			id, _ = params["id"].(string)
		}
		if id == "" {
			return nil, fmt.Errorf("jobId required")
		}
		// In-place update (poc)
		return &CronResult{Action: action, Status: "ok", Data: "updated (not fully implemented)"}, nil

	case "remove":
		id, _ := params["jobId"].(string)
		if id == "" {
			id, _ = params["id"].(string)
		}
		if id == "" {
			return nil, fmt.Errorf("jobId required")
		}
		if err := t.Scheduler.RemoveJob(id); err != nil {
			return nil, fmt.Errorf("remove failed: %v", err)
		}
		return &CronResult{Action: action, Status: "removed", Data: id}, nil

	case "run":
		id, _ := params["jobId"].(string)
		if id == "" {
			id, _ = params["id"].(string)
		}
		if id == "" {
			return nil, fmt.Errorf("jobId required")
		}
		if err := t.Scheduler.RunJobNow(id); err != nil {
			return nil, err
		}
		return &CronResult{Action: action, Status: "triggered"}, nil

	case "runs":
		id, _ := params["jobId"].(string)
		if id == "" {
			id, _ = params["id"].(string)
		}
		if id == "" {
			return nil, fmt.Errorf("jobId required")
		}
		return &CronResult{Action: action, Status: "ok", Data: map[string]interface{}{"id": id, "runs": []interface{}{}}}, nil

	case "wake":
		text, _ := params["text"].(string)
		if text == "" {
			return nil, fmt.Errorf("text required for wake")
		}
		return &CronResult{Action: action, Status: "sent", Data: text}, nil

	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

// AgentsListTool lists available agents.
type AgentsListTool struct {
	// AgentSessionKey is the current agent's session key.
	AgentSessionKey string
}

// NewAgentsListTool creates a new agents list tool.
func NewAgentsListTool() *AgentsListTool {
	return &AgentsListTool{}
}

// Name returns the tool name.
func (t *AgentsListTool) Name() string {
	return "agents_list"
}

// Description returns the tool description.
func (t *AgentsListTool) Description() string {
	return `List available agents configured in the system.
Shows agent IDs, names, models, and capabilities.`
}

// Parameters returns the JSON Schema for parameters.
func (t *AgentsListTool) Parameters() interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// AgentInfo represents agent information.
type AgentInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Model       string   `json:"model,omitempty"`
	Description string   `json:"description,omitempty"`
	Roles       []string `json:"roles,omitempty"`
}

// AgentsListResult represents the agents list result.
type AgentsListResult struct {
	Agents []AgentInfo `json:"agents"`
	Count  int         `json:"count"`
}

// Execute lists agents.
func (t *AgentsListTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Note: This would read from configuration
	// For now, return a placeholder
	return &AgentsListResult{
		Agents: []AgentInfo{},
		Count:  0,
	}, nil
}

// GatewayTool calls Gateway API methods.
type GatewayTool struct {
	// AgentSessionKey is the current agent's session key.
	AgentSessionKey string
}

// NewGatewayTool creates a new gateway tool.
func NewGatewayTool() *GatewayTool {
	return &GatewayTool{}
}

// Name returns the tool name.
func (t *GatewayTool) Name() string {
	return "gateway"
}

// Description returns the tool description.
func (t *GatewayTool) Description() string {
	return `Call Gateway API methods directly.
Use for advanced gateway operations not covered by specialized tools.`
}

// Parameters returns the JSON Schema for parameters.
func (t *GatewayTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"method": map[string]interface{}{
				"type":        "string",
				"description": "Gateway method to call",
			},
			"params": map[string]interface{}{
				"type":        "object",
				"description": "Parameters for the method",
			},
			"timeoutMs": map[string]interface{}{
				"type":        "integer",
				"description": "Request timeout in milliseconds",
			},
		},
		"required": []string{"method"},
	}
}

// GatewayResult represents a gateway call result.
type GatewayResult struct {
	Method  string      `json:"method"`
	Status  string      `json:"status"`
	Payload interface{} `json:"payload,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Execute calls the gateway method.
func (t *GatewayTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	method, _ := params["method"].(string)
	if method == "" {
		return nil, fmt.Errorf("method is required")
	}

	// Note: This would make actual gateway calls
	// For now, return a placeholder
	return &GatewayResult{
		Method: method,
		Status: "pending",
	}, nil
}

// SessionStatusTool checks session status.
type SessionStatusTool struct {
	// AgentSessionKey is the current agent's session key.
	AgentSessionKey string
}

// NewSessionStatusTool creates a new session status tool.
func NewSessionStatusTool() *SessionStatusTool {
	return &SessionStatusTool{}
}

// Name returns the tool name.
func (t *SessionStatusTool) Name() string {
	return "session_status"
}

// Description returns the tool description.
func (t *SessionStatusTool) Description() string {
	return `Check the status of the current session or a specific session.
Shows run state, pending messages, and resource usage.`
}

// Parameters returns the JSON Schema for parameters.
func (t *SessionStatusTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"sessionKey": map[string]interface{}{
				"type":        "string",
				"description": "Session key to check (defaults to current session)",
			},
		},
	}
}

// SessionStatusResult represents session status.
type SessionStatusResult struct {
	SessionKey string `json:"sessionKey"`
	Status     string `json:"status"`
	Running    bool   `json:"running"`
	Messages   int    `json:"messages"`
}

// Execute checks session status.
func (t *SessionStatusTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sessionKey, _ := params["sessionKey"].(string)
	if sessionKey == "" {
		sessionKey = t.AgentSessionKey
	}

	return &SessionStatusResult{
		SessionKey: sessionKey,
		Status:     "active",
		Running:    true,
		Messages:   0,
	}, nil
}
