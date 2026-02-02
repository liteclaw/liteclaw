// Package tools provides agent tool implementations.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// BrowserTool controls browser instances via a control server.
type BrowserTool struct {
	// DefaultControlURL is the default browser control server URL.
	DefaultControlURL string
	// AllowHostControl allows controlling the host browser.
	AllowHostControl bool
	// AllowedControlHosts is a list of allowed control hosts.
	AllowedControlHosts []string
	// Timeout is the request timeout.
	Timeout time.Duration
	// ScreenshotDir is where screenshots are saved.
	ScreenshotDir string
}

// NewBrowserTool creates a new browser tool.
func NewBrowserTool() *BrowserTool {
	defaultURL := os.Getenv("BROWSER_CONTROL_URL")
	if defaultURL == "" {
		// Default to LiteClaw gateway browser API (dedicated port)
		defaultURL = "http://localhost:18791"
	}

	return &BrowserTool{
		DefaultControlURL: defaultURL,
		AllowHostControl:  os.Getenv("ALLOW_HOST_BROWSER_CONTROL") == "true",
		Timeout:           30 * time.Second,
		ScreenshotDir:     os.TempDir(),
	}
}

// Name returns the tool name.
func (t *BrowserTool) Name() string {
	return "browser"
}

// Description returns the tool description.
func (t *BrowserTool) Description() string {
	return `Control browser instances for web automation and testing.

ACTIONS:
- status: Get browser and profile status
- start: Start the browser for a profile
- stop: Stop the browser for a profile
- profiles: List all available browser profiles
- tabs: List open tabs in the current profile
- open: Open a new tab with URL
- focus: Focus a specific tab
- close: Close a tab or the current page
- navigate: Navigate a tab to a URL
- snapshot: Get page DOM snapshot (accessibility tree/AI snapshot)
- screenshot: Capture page or element screenshot
- click: Click an element (via /act)
- type: Type text into an element (via /act)
- evaluate: Execute JavaScript (via /act)
- wait: Wait for selector, text, or timeout (via /act)
- resize: Resize the browser window (via /act)
- console: Get recent console messages
- pdf: Save the current page as PDF
- upload: Handle file upload dialogs (file chooser hook)
- dialog: Handle JS alert/confirm/prompt dialogs (dialog hook)

Use snapshot to understand page structure before interacting.
Element identifiers (ref) can be accessibility labels or IDs from snapshot.`
}

// Parameters returns the JSON Schema for parameters.
func (t *BrowserTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Browser action: tabs, open, close, navigate, snapshot, screenshot, click, type",
			},
			"tabId": map[string]interface{}{
				"type":        "string",
				"description": "Tab ID for tab-specific actions",
			},
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL for open/navigate actions",
			},
			"selector": map[string]interface{}{
				"type":        "string",
				"description": "CSS selector or accessibility label (ref) for actions",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text for type action",
			},
			"profile": map[string]interface{}{
				"type":        "string",
				"description": "Browser profile (e.g., 'chrome' for extension)",
			},
		},
		"required": []string{"action"},
	}
}

// BrowserTab represents a browser tab.
type BrowserTab struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Active bool   `json:"active"`
}

// BrowserResult represents a browser action result.
type BrowserResult struct {
	Action     string       `json:"action"`
	Success    bool         `json:"success"`
	TabID      string       `json:"tabId,omitempty"`
	Tabs       []BrowserTab `json:"tabs,omitempty"`
	URL        string       `json:"url,omitempty"`
	Title      string       `json:"title,omitempty"`
	Snapshot   string       `json:"snapshot,omitempty"`
	Screenshot string       `json:"screenshot,omitempty"`
	FilePath   string       `json:"filePath,omitempty"`
	Result     interface{}  `json:"result,omitempty"`
	Error      string       `json:"error,omitempty"`
}

// Execute performs the browser action.
func (t *BrowserTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}

	controlURL := t.DefaultControlURL
	if url, ok := params["controlUrl"].(string); ok && url != "" {
		controlURL = url
	}

	// Validate control URL if required
	if controlURL == "" && !t.AllowHostControl {
		return nil, fmt.Errorf("browser control URL not configured. Set BROWSER_CONTROL_URL or provide controlUrl parameter")
	}

	profile, _ := params["profile"].(string)

	switch action {
	case "status":
		return t.getStatus(ctx, controlURL, profile)
	case "start":
		return t.startBrowser(ctx, controlURL, profile)
	case "stop":
		return t.stopBrowser(ctx, controlURL, profile)
	case "profiles":
		return t.listProfiles(ctx, controlURL)
	case "tabs":
		return t.listTabs(ctx, controlURL, profile)
	case "open":
		url, _ := params["url"].(string)
		if url == "" {
			url, _ = params["targetUrl"].(string)
		}
		return t.openTab(ctx, controlURL, profile, url)
	case "focus":
		tabID, _ := params["tabId"].(string)
		return t.focusTab(ctx, controlURL, profile, tabID)
	case "close":
		tabID, _ := params["tabId"].(string)
		return t.closeTab(ctx, controlURL, profile, tabID)
	case "navigate":
		url, _ := params["url"].(string)
		tabID, _ := params["tabId"].(string)
		return t.navigate(ctx, controlURL, profile, tabID, url)
	case "snapshot":
		tabID, _ := params["tabId"].(string)
		return t.getSnapshot(ctx, controlURL, profile, tabID)
	case "screenshot":
		tabID, _ := params["tabId"].(string)
		format, _ := params["format"].(string)
		fullPage, _ := params["fullPage"].(bool)
		return t.takeScreenshot(ctx, controlURL, profile, tabID, format, fullPage)
	case "click":
		selector, _ := params["selector"].(string)
		if selector == "" {
			selector, _ = params["ref"].(string)
		}
		tabID, _ := params["tabId"].(string)
		return t.act(ctx, controlURL, profile, "click", map[string]interface{}{
			"targetId": tabID,
			"ref":      selector,
		})
	case "type":
		selector, _ := params["selector"].(string)
		if selector == "" {
			selector, _ = params["ref"].(string)
		}
		text, _ := params["text"].(string)
		tabID, _ := params["tabId"].(string)
		return t.act(ctx, controlURL, profile, "type", map[string]interface{}{
			"targetId": tabID,
			"ref":      selector,
			"text":     text,
		})
	case "evaluate":
		script, _ := params["script"].(string)
		if script == "" {
			script, _ = params["fn"].(string)
		}
		tabID, _ := params["tabId"].(string)
		return t.act(ctx, controlURL, profile, "evaluate", map[string]interface{}{
			"targetId": tabID,
			"fn":       script,
		})
	case "wait":
		selector, _ := params["selector"].(string)
		text, _ := params["text"].(string)
		timeout, _ := params["timeout"].(float64)
		tabID, _ := params["tabId"].(string)
		waitParams := map[string]interface{}{
			"targetId": tabID,
		}
		if selector != "" {
			waitParams["selector"] = selector
		}
		if text != "" {
			waitParams["text"] = text
		}
		if timeout > 0 {
			waitParams["timeoutMs"] = int(timeout)
		}
		return t.act(ctx, controlURL, profile, "wait", waitParams)
	case "resize":
		width, _ := params["width"].(float64)
		height, _ := params["height"].(float64)
		return t.act(ctx, controlURL, profile, "resize", map[string]interface{}{
			"width":  int(width),
			"height": int(height),
		})
	case "console":
		tabID, _ := params["tabId"].(string)
		return t.getConsole(ctx, controlURL, profile, tabID)
	case "pdf":
		tabID, _ := params["tabId"].(string)
		return t.savePDF(ctx, controlURL, profile, tabID)
	case "upload":
		tabID, _ := params["tabId"].(string)
		paths, _ := params["paths"].([]interface{})
		selector, _ := params["selector"].(string)
		if selector == "" {
			selector, _ = params["ref"].(string)
		}
		timeout, _ := params["timeout"].(float64)
		return t.uploadFiles(ctx, controlURL, profile, tabID, paths, selector, int(timeout))
	case "dialog":
		tabID, _ := params["tabId"].(string)
		accept, _ := params["accept"].(bool)
		promptText, _ := params["promptText"].(string)
		timeout, _ := params["timeout"].(float64)
		return t.handleDialog(ctx, controlURL, profile, tabID, accept, promptText, int(timeout))
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

// Browser control server communication

func (t *BrowserTool) sendCommand(ctx context.Context, controlURL, method, path string, payload interface{}, profile string) (map[string]interface{}, error) {
	if controlURL == "" {
		return map[string]interface{}{
			"success": true,
			"mock":    true,
		}, nil
	}

	fullURL := strings.TrimRight(controlURL, "/") + path
	if profile != "" {
		if strings.Contains(fullURL, "?") {
			fullURL += "&profile=" + profile
		} else {
			fullURL += "?profile=" + profile
		}
	}

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: t.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

func (t *BrowserTool) act(ctx context.Context, controlURL, profile, kind string, params map[string]interface{}) (*BrowserResult, error) {
	params["kind"] = kind
	resp, err := t.sendCommand(ctx, controlURL, "POST", "/act", params, profile)
	if err != nil {
		return nil, err
	}

	tabID, _ := resp["targetId"].(string)
	url, _ := resp["url"].(string)
	result := resp["result"]

	return &BrowserResult{
		Action:  kind,
		Success: true,
		TabID:   tabID,
		URL:     url,
		Result:  result,
	}, nil
}

func (t *BrowserTool) listTabs(ctx context.Context, controlURL, profile string) (*BrowserResult, error) {
	resp, err := t.sendCommand(ctx, controlURL, "GET", "/tabs", nil, profile)
	if err != nil {
		return nil, err
	}

	tabs := []BrowserTab{}
	if tabsData, ok := resp["tabs"].([]interface{}); ok {
		for _, td := range tabsData {
			if tm, ok := td.(map[string]interface{}); ok {
				tab := BrowserTab{
					ID:     fmt.Sprint(tm["targetId"]), // TS uses targetId
					URL:    fmt.Sprint(tm["url"]),
					Title:  fmt.Sprint(tm["title"]),
					Active: tm["active"] == true,
				}
				tabs = append(tabs, tab)
			}
		}
	}

	return &BrowserResult{
		Action:  "tabs",
		Success: true,
		Tabs:    tabs,
	}, nil
}

func (t *BrowserTool) openTab(ctx context.Context, controlURL, profile, url string) (*BrowserResult, error) {
	if url == "" {
		url = "about:blank"
	}

	resp, err := t.sendCommand(ctx, controlURL, "POST", "/tabs/open", map[string]interface{}{
		"url": url,
	}, profile)
	if err != nil {
		return nil, err
	}

	tabID := ""
	if id, ok := resp["targetId"].(string); ok {
		tabID = id
	}

	return &BrowserResult{
		Action:  "open",
		Success: true,
		TabID:   tabID,
		URL:     url,
	}, nil
}

func (t *BrowserTool) closeTab(ctx context.Context, controlURL, profile, tabID string) (*BrowserResult, error) {
	if tabID != "" {
		_, err := t.sendCommand(ctx, controlURL, "DELETE", "/tabs/"+tabID, nil, profile)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := t.act(ctx, controlURL, profile, "close", map[string]interface{}{})
		if err != nil {
			return nil, err
		}
	}

	return &BrowserResult{
		Action:  "close",
		Success: true,
		TabID:   tabID,
	}, nil
}

func (t *BrowserTool) navigate(ctx context.Context, controlURL, profile, tabID, url string) (*BrowserResult, error) {
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}

	_, err := t.sendCommand(ctx, controlURL, "POST", "/navigate", map[string]interface{}{
		"targetId": tabID,
		"url":      url,
	}, profile)
	if err != nil {
		return nil, err
	}

	return &BrowserResult{
		Action:  "navigate",
		Success: true,
		TabID:   tabID,
		URL:     url,
	}, nil
}

func (t *BrowserTool) getSnapshot(ctx context.Context, controlURL, profile, tabID string) (*BrowserResult, error) {
	path := "/snapshot"
	if tabID != "" {
		path += "?targetId=" + tabID
	}
	resp, err := t.sendCommand(ctx, controlURL, "GET", path, nil, profile)
	if err != nil {
		return nil, err
	}

	snapshot := ""
	if s, ok := resp["snapshot"].(string); ok {
		snapshot = s
	}

	title := ""
	if t, ok := resp["title"].(string); ok {
		title = t
	}

	url := ""
	if u, ok := resp["url"].(string); ok {
		url = u
	}

	actualTabID := tabID
	if id, ok := resp["targetId"].(string); ok {
		actualTabID = id
	}

	return &BrowserResult{
		Action:   "snapshot",
		Success:  true,
		TabID:    actualTabID,
		Snapshot: snapshot,
		Title:    title,
		URL:      url,
	}, nil
}

func (t *BrowserTool) takeScreenshot(ctx context.Context, controlURL, profile, tabID, format string, fullPage bool) (*BrowserResult, error) {
	if format == "" {
		format = "png"
	}

	resp, err := t.sendCommand(ctx, controlURL, "POST", "/screenshot", map[string]interface{}{
		"targetId": tabID,
		"type":     format,
		"fullPage": fullPage,
	}, profile)
	if err != nil {
		return nil, err
	}

	// TS server returns { "ok": true, "path": "..." }
	filePath, _ := resp["path"].(string)

	return &BrowserResult{
		Action:   "screenshot",
		Success:  true,
		TabID:    tabID,
		FilePath: filePath,
	}, nil
}

func (t *BrowserTool) getStatus(ctx context.Context, controlURL, profile string) (*BrowserResult, error) {
	resp, err := t.sendCommand(ctx, controlURL, "GET", "/", nil, profile)
	if err != nil {
		return nil, err
	}

	return &BrowserResult{
		Action:  "status",
		Success: true,
		Result:  resp,
	}, nil
}

func (t *BrowserTool) startBrowser(ctx context.Context, controlURL, profile string) (*BrowserResult, error) {
	_, err := t.sendCommand(ctx, controlURL, "POST", "/start", nil, profile)
	if err != nil {
		return nil, err
	}

	return t.getStatus(ctx, controlURL, profile)
}

func (t *BrowserTool) stopBrowser(ctx context.Context, controlURL, profile string) (*BrowserResult, error) {
	_, err := t.sendCommand(ctx, controlURL, "POST", "/stop", nil, profile)
	if err != nil {
		return nil, err
	}

	return t.getStatus(ctx, controlURL, profile)
}

func (t *BrowserTool) listProfiles(ctx context.Context, controlURL string) (*BrowserResult, error) {
	resp, err := t.sendCommand(ctx, controlURL, "GET", "/profiles", nil, "")
	if err != nil {
		return nil, err
	}

	return &BrowserResult{
		Action:  "profiles",
		Success: true,
		Result:  resp["profiles"],
	}, nil
}

func (t *BrowserTool) focusTab(ctx context.Context, controlURL, profile, tabID string) (*BrowserResult, error) {
	if tabID == "" {
		return nil, fmt.Errorf("tabId is required")
	}

	_, err := t.sendCommand(ctx, controlURL, "POST", "/tabs/focus", map[string]interface{}{
		"targetId": tabID,
	}, profile)
	if err != nil {
		return nil, err
	}

	return &BrowserResult{
		Action:  "focus",
		Success: true,
		TabID:   tabID,
	}, nil
}

func (t *BrowserTool) getConsole(ctx context.Context, controlURL, profile, tabID string) (*BrowserResult, error) {
	path := "/console"
	if tabID != "" {
		path += "?targetId=" + tabID
	}
	resp, err := t.sendCommand(ctx, controlURL, "GET", path, nil, profile)
	if err != nil {
		return nil, err
	}

	return &BrowserResult{
		Action:  "console",
		Success: true,
		TabID:   tabID,
		Result:  resp["messages"],
	}, nil
}

func (t *BrowserTool) savePDF(ctx context.Context, controlURL, profile, tabID string) (*BrowserResult, error) {
	resp, err := t.sendCommand(ctx, controlURL, "POST", "/pdf", map[string]interface{}{
		"targetId": tabID,
	}, profile)
	if err != nil {
		return nil, err
	}

	filePath, _ := resp["path"].(string)

	return &BrowserResult{
		Action:   "pdf",
		Success:  true,
		TabID:    tabID,
		FilePath: filePath,
	}, nil
}

func (t *BrowserTool) uploadFiles(ctx context.Context, controlURL, profile, tabID string, paths []interface{}, selector string, timeout int) (*BrowserResult, error) {
	body := map[string]interface{}{
		"targetId": tabID,
		"paths":    paths,
	}
	if selector != "" {
		body["ref"] = selector
	}
	if timeout > 0 {
		body["timeoutMs"] = timeout
	}

	_, err := t.sendCommand(ctx, controlURL, "POST", "/hooks/file-chooser", body, profile)
	if err != nil {
		return nil, err
	}

	return &BrowserResult{
		Action:  "upload",
		Success: true,
		TabID:   tabID,
	}, nil
}

func (t *BrowserTool) handleDialog(ctx context.Context, controlURL, profile, tabID string, accept bool, promptText string, timeout int) (*BrowserResult, error) {
	body := map[string]interface{}{
		"targetId": tabID,
		"accept":   accept,
	}
	if promptText != "" {
		body["promptText"] = promptText
	}
	if timeout > 0 {
		body["timeoutMs"] = timeout
	}

	_, err := t.sendCommand(ctx, controlURL, "POST", "/hooks/dialog", body, profile)
	if err != nil {
		return nil, err
	}

	return &BrowserResult{
		Action:  "dialog",
		Success: true,
		TabID:   tabID,
	}, nil
}
