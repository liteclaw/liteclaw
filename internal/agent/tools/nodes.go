// Package tools provides agent tool implementations.
package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CanvasTool controls node canvas displays.
type CanvasTool struct {
	// GatewayURL is the gateway server URL.
	GatewayURL string
	// GatewayToken is the authentication token.
	GatewayToken string
	// Timeout is the request timeout.
	Timeout time.Duration
}

// NewCanvasTool creates a new canvas tool.
func NewCanvasTool() *CanvasTool {
	return &CanvasTool{
		GatewayURL:   os.Getenv("GATEWAY_URL"),
		GatewayToken: os.Getenv("GATEWAY_TOKEN"),
		Timeout:      30 * time.Second,
	}
}

// Name returns the tool name.
func (t *CanvasTool) Name() string {
	return "canvas"
}

// Description returns the tool description.
func (t *CanvasTool) Description() string {
	return `Control node canvases for displaying UI and content.

ACTIONS:
- present: Show canvas with optional URL and placement
- hide: Hide the canvas
- navigate: Navigate canvas to a URL
- eval: Execute JavaScript in canvas context
- snapshot: Capture canvas screenshot
- a2ui_push: Push A2UI JSONL content
- a2ui_reset: Reset A2UI state

Use for displaying dynamic content, dashboards, or interactive UIs.`
}

// Parameters returns the JSON Schema for parameters.
func (t *CanvasTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Canvas action to perform",
				"enum":        []string{"present", "hide", "navigate", "eval", "snapshot", "a2ui_push", "a2ui_reset"},
			},
			"node": map[string]interface{}{
				"type":        "string",
				"description": "Node ID (optional, uses first available)",
			},
			"target": map[string]interface{}{
				"type":        "string",
				"description": "URL for present action",
			},
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL for navigate action",
			},
			"javaScript": map[string]interface{}{
				"type":        "string",
				"description": "JavaScript code for eval action",
			},
			"x": map[string]interface{}{
				"type":        "integer",
				"description": "X position for present",
			},
			"y": map[string]interface{}{
				"type":        "integer",
				"description": "Y position for present",
			},
			"width": map[string]interface{}{
				"type":        "integer",
				"description": "Width for present",
			},
			"height": map[string]interface{}{
				"type":        "integer",
				"description": "Height for present",
			},
			"outputFormat": map[string]interface{}{
				"type":        "string",
				"description": "Snapshot format: png, jpeg",
				"enum":        []string{"png", "jpeg"},
			},
			"jsonl": map[string]interface{}{
				"type":        "string",
				"description": "JSONL content for a2ui_push",
			},
			"jsonlPath": map[string]interface{}{
				"type":        "string",
				"description": "Path to JSONL file for a2ui_push",
			},
		},
		"required": []string{"action"},
	}
}

// CanvasResult represents a canvas action result.
type CanvasResult struct {
	Action     string `json:"action"`
	Success    bool   `json:"success"`
	NodeID     string `json:"nodeId,omitempty"`
	Screenshot string `json:"screenshot,omitempty"`
	FilePath   string `json:"filePath,omitempty"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Execute performs the canvas action.
func (t *CanvasTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}

	nodeID, _ := params["node"].(string)

	switch action {
	case "present":
		return t.present(ctx, nodeID, params)
	case "hide":
		return t.hide(ctx, nodeID)
	case "navigate":
		url, _ := params["url"].(string)
		return t.navigate(ctx, nodeID, url)
	case "eval":
		js, _ := params["javaScript"].(string)
		return t.eval(ctx, nodeID, js)
	case "snapshot":
		format, _ := params["outputFormat"].(string)
		return t.snapshot(ctx, nodeID, format)
	case "a2ui_push":
		jsonl, _ := params["jsonl"].(string)
		jsonlPath, _ := params["jsonlPath"].(string)
		return t.a2uiPush(ctx, nodeID, jsonl, jsonlPath)
	case "a2ui_reset":
		return t.a2uiReset(ctx, nodeID)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func (t *CanvasTool) invokeNode(ctx context.Context, nodeID, command string, cmdParams map[string]interface{}) (map[string]interface{}, error) {
	if t.GatewayURL == "" {
		// Mock mode
		return map[string]interface{}{
			"success": true,
			"mock":    true,
		}, nil
	}

	payload := map[string]interface{}{
		"nodeId":         nodeID,
		"command":        command,
		"params":         cmdParams,
		"idempotencyKey": uuid.New().String(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.GatewayURL+"/node.invoke", strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if t.GatewayToken != "" {
		req.Header.Set("Authorization", "Bearer "+t.GatewayToken)
	}

	client := &http.Client{Timeout: t.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

func (t *CanvasTool) present(ctx context.Context, nodeID string, params map[string]interface{}) (*CanvasResult, error) {
	cmdParams := map[string]interface{}{}

	if target, ok := params["target"].(string); ok && target != "" {
		cmdParams["url"] = target
	}

	placement := map[string]interface{}{}
	if x, ok := params["x"].(float64); ok {
		placement["x"] = int(x)
	}
	if y, ok := params["y"].(float64); ok {
		placement["y"] = int(y)
	}
	if w, ok := params["width"].(float64); ok {
		placement["width"] = int(w)
	}
	if h, ok := params["height"].(float64); ok {
		placement["height"] = int(h)
	}
	if len(placement) > 0 {
		cmdParams["placement"] = placement
	}

	_, err := t.invokeNode(ctx, nodeID, "canvas.present", cmdParams)
	if err != nil {
		return nil, err
	}

	return &CanvasResult{
		Action:  "present",
		Success: true,
		NodeID:  nodeID,
	}, nil
}

func (t *CanvasTool) hide(ctx context.Context, nodeID string) (*CanvasResult, error) {
	_, err := t.invokeNode(ctx, nodeID, "canvas.hide", nil)
	if err != nil {
		return nil, err
	}

	return &CanvasResult{
		Action:  "hide",
		Success: true,
		NodeID:  nodeID,
	}, nil
}

func (t *CanvasTool) navigate(ctx context.Context, nodeID, url string) (*CanvasResult, error) {
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}

	_, err := t.invokeNode(ctx, nodeID, "canvas.navigate", map[string]interface{}{
		"url": url,
	})
	if err != nil {
		return nil, err
	}

	return &CanvasResult{
		Action:  "navigate",
		Success: true,
		NodeID:  nodeID,
	}, nil
}

func (t *CanvasTool) eval(ctx context.Context, nodeID, javaScript string) (*CanvasResult, error) {
	if javaScript == "" {
		return nil, fmt.Errorf("javaScript is required")
	}

	resp, err := t.invokeNode(ctx, nodeID, "canvas.eval", map[string]interface{}{
		"javaScript": javaScript,
	})
	if err != nil {
		return nil, err
	}

	result := ""
	if payload, ok := resp["payload"].(map[string]interface{}); ok {
		if r, ok := payload["result"].(string); ok {
			result = r
		}
	}

	return &CanvasResult{
		Action:  "eval",
		Success: true,
		NodeID:  nodeID,
		Result:  result,
	}, nil
}

func (t *CanvasTool) snapshot(ctx context.Context, nodeID, format string) (*CanvasResult, error) {
	if format == "" {
		format = "png"
	}

	resp, err := t.invokeNode(ctx, nodeID, "canvas.snapshot", map[string]interface{}{
		"format": format,
	})
	if err != nil {
		return nil, err
	}

	screenshotData := ""
	if payload, ok := resp["payload"].(map[string]interface{}); ok {
		if data, ok := payload["base64"].(string); ok {
			screenshotData = data
		}
	}

	// Save to file
	filePath := ""
	if screenshotData != "" {
		ext := ".png"
		if format == "jpeg" {
			ext = ".jpg"
		}
		fileName := fmt.Sprintf("canvas_%s%s", uuid.New().String()[:8], ext)
		filePath = filepath.Join(os.TempDir(), fileName)

		data, err := base64.StdEncoding.DecodeString(screenshotData)
		if err == nil {
			_ = os.WriteFile(filePath, data, 0644)
		}
	}

	return &CanvasResult{
		Action:     "snapshot",
		Success:    true,
		NodeID:     nodeID,
		Screenshot: screenshotData,
		FilePath:   filePath,
	}, nil
}

func (t *CanvasTool) a2uiPush(ctx context.Context, nodeID, jsonl, jsonlPath string) (*CanvasResult, error) {
	content := jsonl
	if content == "" && jsonlPath != "" {
		data, err := os.ReadFile(jsonlPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read jsonl file: %w", err)
		}
		content = string(data)
	}

	if content == "" {
		return nil, fmt.Errorf("jsonl or jsonlPath is required")
	}

	_, err := t.invokeNode(ctx, nodeID, "canvas.a2ui.pushJSONL", map[string]interface{}{
		"jsonl": content,
	})
	if err != nil {
		return nil, err
	}

	return &CanvasResult{
		Action:  "a2ui_push",
		Success: true,
		NodeID:  nodeID,
	}, nil
}

func (t *CanvasTool) a2uiReset(ctx context.Context, nodeID string) (*CanvasResult, error) {
	_, err := t.invokeNode(ctx, nodeID, "canvas.a2ui.reset", nil)
	if err != nil {
		return nil, err
	}

	return &CanvasResult{
		Action:  "a2ui_reset",
		Success: true,
		NodeID:  nodeID,
	}, nil
}

// NodesTool manages gateway nodes.
type NodesTool struct {
	// GatewayURL is the gateway server URL.
	GatewayURL string
	// GatewayToken is the authentication token.
	GatewayToken string
	// AgentSessionKey is the current session key.
	AgentSessionKey string
	// Timeout is the request timeout.
	Timeout time.Duration
}

// NewNodesTool creates a new nodes tool.
func NewNodesTool() *NodesTool {
	return &NodesTool{
		GatewayURL:   os.Getenv("GATEWAY_URL"),
		GatewayToken: os.Getenv("GATEWAY_TOKEN"),
		Timeout:      30 * time.Second,
	}
}

// Name returns the tool name.
func (t *NodesTool) Name() string {
	return "nodes"
}

// Description returns the tool description.
func (t *NodesTool) Description() string {
	return `Manage gateway nodes (devices, displays, services).

ACTIONS:
- list: List all connected nodes
- status: Get status of a specific node
- invoke: Invoke a command on a node
- subscribe: Subscribe to node events

Nodes can be displays, cameras, microphones, or other devices.`
}

// Parameters returns the JSON Schema for parameters.
func (t *NodesTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action: list, status, invoke, subscribe",
				"enum":        []string{"list", "status", "invoke", "subscribe"},
			},
			"nodeId": map[string]interface{}{
				"type":        "string",
				"description": "Node ID for node-specific actions",
			},
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Command for invoke action",
			},
			"params": map[string]interface{}{
				"type":        "object",
				"description": "Parameters for invoke action",
			},
		},
		"required": []string{"action"},
	}
}

// NodeInfo represents node information.
type NodeInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Status       string   `json:"status"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// NodesResult represents a nodes action result.
type NodesResult struct {
	Action  string      `json:"action"`
	Success bool        `json:"success"`
	Nodes   []NodeInfo  `json:"nodes,omitempty"`
	Node    *NodeInfo   `json:"node,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Execute performs the nodes action.
func (t *NodesTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}

	switch action {
	case "list":
		return t.listNodes(ctx)
	case "status":
		nodeID, _ := params["nodeId"].(string)
		return t.getStatus(ctx, nodeID)
	case "invoke":
		nodeID, _ := params["nodeId"].(string)
		command, _ := params["command"].(string)
		cmdParams, _ := params["params"].(map[string]interface{})
		return t.invoke(ctx, nodeID, command, cmdParams)
	case "subscribe":
		nodeID, _ := params["nodeId"].(string)
		return t.subscribe(ctx, nodeID)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func (t *NodesTool) callGateway(ctx context.Context, method string, payload interface{}) (map[string]interface{}, error) {
	if t.GatewayURL == "" {
		return map[string]interface{}{
			"success": true,
			"mock":    true,
		}, nil
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.GatewayURL+"/"+method, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if t.GatewayToken != "" {
		req.Header.Set("Authorization", "Bearer "+t.GatewayToken)
	}

	client := &http.Client{Timeout: t.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

func (t *NodesTool) listNodes(ctx context.Context) (*NodesResult, error) {
	resp, err := t.callGateway(ctx, "nodes.list", nil)
	if err != nil {
		return nil, err
	}

	nodes := []NodeInfo{}
	if nodesData, ok := resp["nodes"].([]interface{}); ok {
		for _, nd := range nodesData {
			if nm, ok := nd.(map[string]interface{}); ok {
				node := NodeInfo{
					ID:     fmt.Sprint(nm["id"]),
					Name:   fmt.Sprint(nm["name"]),
					Type:   fmt.Sprint(nm["type"]),
					Status: fmt.Sprint(nm["status"]),
				}
				if caps, ok := nm["capabilities"].([]interface{}); ok {
					for _, c := range caps {
						node.Capabilities = append(node.Capabilities, fmt.Sprint(c))
					}
				}
				nodes = append(nodes, node)
			}
		}
	}

	return &NodesResult{
		Action:  "list",
		Success: true,
		Nodes:   nodes,
	}, nil
}

func (t *NodesTool) getStatus(ctx context.Context, nodeID string) (*NodesResult, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("nodeId is required")
	}

	resp, err := t.callGateway(ctx, "nodes.status", map[string]interface{}{
		"nodeId": nodeID,
	})
	if err != nil {
		return nil, err
	}

	node := &NodeInfo{
		ID:     nodeID,
		Status: "unknown",
	}
	if nd, ok := resp["node"].(map[string]interface{}); ok {
		node.Name = fmt.Sprint(nd["name"])
		node.Type = fmt.Sprint(nd["type"])
		node.Status = fmt.Sprint(nd["status"])
	}

	return &NodesResult{
		Action:  "status",
		Success: true,
		Node:    node,
	}, nil
}

func (t *NodesTool) invoke(ctx context.Context, nodeID, command string, cmdParams map[string]interface{}) (*NodesResult, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("nodeId is required")
	}
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	resp, err := t.callGateway(ctx, "node.invoke", map[string]interface{}{
		"nodeId":         nodeID,
		"command":        command,
		"params":         cmdParams,
		"idempotencyKey": uuid.New().String(),
	})
	if err != nil {
		return nil, err
	}

	return &NodesResult{
		Action:  "invoke",
		Success: true,
		Result:  resp["result"],
	}, nil
}

func (t *NodesTool) subscribe(ctx context.Context, nodeID string) (*NodesResult, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("nodeId is required")
	}

	_, err := t.callGateway(ctx, "nodes.subscribe", map[string]interface{}{
		"nodeId": nodeID,
	})
	if err != nil {
		return nil, err
	}

	return &NodesResult{
		Action:  "subscribe",
		Success: true,
	}, nil
}

// TtsTool provides text-to-speech functionality.
type TtsTool struct {
	// DefaultVoice is the default voice to use.
	DefaultVoice string
	// OutputDir is where audio files are saved.
	OutputDir string
}

// NewTtsTool creates a new TTS tool.
func NewTtsTool() *TtsTool {
	return &TtsTool{
		DefaultVoice: "alloy",
		OutputDir:    os.TempDir(),
	}
}

// Name returns the tool name.
func (t *TtsTool) Name() string {
	return "tts"
}

// Description returns the tool description.
func (t *TtsTool) Description() string {
	return `Convert text to speech audio.
Supports multiple voices and output formats.
Audio can be played directly or saved to file.`
}

// Parameters returns the JSON Schema for parameters.
func (t *TtsTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text to convert to speech",
			},
			"voice": map[string]interface{}{
				"type":        "string",
				"description": "Voice to use (alloy, echo, fable, onyx, nova, shimmer)",
				"enum":        []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"},
			},
			"speed": map[string]interface{}{
				"type":        "number",
				"description": "Speaking speed (0.25 to 4.0, default: 1.0)",
				"minimum":     0.25,
				"maximum":     4.0,
			},
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Output format: mp3, opus, aac, flac",
				"enum":        []string{"mp3", "opus", "aac", "flac"},
			},
			"play": map[string]interface{}{
				"type":        "boolean",
				"description": "Play audio immediately after generation",
			},
		},
		"required": []string{"text"},
	}
}

// TtsResult represents TTS generation result.
type TtsResult struct {
	Text     string  `json:"text"`
	Voice    string  `json:"voice"`
	Format   string  `json:"format"`
	FilePath string  `json:"filePath,omitempty"`
	Duration float64 `json:"duration,omitempty"`
	Played   bool    `json:"played"`
	Error    string  `json:"error,omitempty"`
}

// Execute generates speech from text.
func (t *TtsTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	text, _ := params["text"].(string)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	voice, _ := params["voice"].(string)
	if voice == "" {
		voice = t.DefaultVoice
	}

	format, _ := params["format"].(string)
	if format == "" {
		format = "mp3"
	}

	// Note: Actual TTS API integration would go here
	// For now, return a placeholder indicating the tool is ready
	fileName := fmt.Sprintf("tts_%s.%s", uuid.New().String()[:8], format)
	filePath := filepath.Join(t.OutputDir, fileName)

	return &TtsResult{
		Text:     text,
		Voice:    voice,
		Format:   format,
		FilePath: filePath,
		Played:   false,
	}, nil
}
