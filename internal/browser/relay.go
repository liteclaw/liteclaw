package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Command represents a command sent to the extension.
type Command struct {
	ID     int64                  `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// Response represents a response from the extension.
type Response struct {
	ID      string      `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
	Error   string      `json:"error,omitempty"`
	Success bool        `json:"success,omitempty"`
}

// RelayManager manages extension connections and command relaying.
type RelayManager struct {
	mu           sync.RWMutex
	connections  map[string]*websocket.Conn                   // profile -> connection
	pendingCalls map[string]chan *Response                    // commandID -> response chan
	tabs         map[string]map[string]map[string]interface{} // profile -> sessionId -> targetInfo
}

// NewRelayManager creates a new relay manager.
func NewRelayManager() *RelayManager {
	return &RelayManager{
		connections:  make(map[string]*websocket.Conn),
		pendingCalls: make(map[string]chan *Response),
		tabs:         make(map[string]map[string]map[string]interface{}),
	}
}

// RegisterConnection registers a new extension connection for a profile.
func (m *RelayManager) RegisterConnection(profile string, conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close old connection if any
	if old, ok := m.connections[profile]; ok {
		_ = old.Close()
	}
	m.connections[profile] = conn
}

// UnregisterConnection removes a connection.
func (m *RelayManager) UnregisterConnection(profile string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.connections, profile)
	delete(m.tabs, profile)
}

// Call sends a command to the extension and waits for a response.
func (m *RelayManager) Call(ctx context.Context, profile, method string, params map[string]interface{}, targetID string) (map[string]interface{}, error) {
	m.mu.RLock()
	conn, ok := m.connections[profile]
	// If specific profile not found or empty, use the first available connection
	if !ok {
		for p, c := range m.connections {
			fmt.Printf("[Relay] Profile '%s' not found, falling back to discovered profile '%s'\n", profile, p)
			profile = p
			conn = c
			ok = true
			break
		}
	}
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no extension connected for profile: %s", profile)
	}

	// Handle 'tabs' locally if using the extension relay
	if method == "tabs" {
		m.mu.RLock()
		profileTabs := m.tabs[profile]
		m.mu.RUnlock()

		tabList := []interface{}{}
		for _, info := range profileTabs {
			tabList = append(tabList, info)
		}
		// Even if empty, return successfully to avoid tool error
		return map[string]interface{}{"tabs": tabList}, nil
	}

	// Map abstract methods to CDP methods if needed
	cdpMethod := method
	switch method {
	case "tabs.open":
		cdpMethod = "Target.createTarget"
	case "tabs.close":
		cdpMethod = "Target.closeTarget"
	case "tabs.focus":
		cdpMethod = "Target.activateTarget"
	case "navigate":
		cdpMethod = "Page.navigate"
	case "snapshot":
		cdpMethod = "Accessibility.getFullAXTree"
	}

	// If targetID is provided, try to find the actual sessionId
	sessionId := targetID
	m.mu.RLock()
	if profileTabs, ok := m.tabs[profile]; ok {
		// First check if targetID is actually a sessionId
		if _, ok := profileTabs[targetID]; !ok {
			// Not a sessionId, look for a matching targetId in the info
			for sid, info := range profileTabs {
				if tid, _ := info["targetId"].(string); tid == targetID {
					sessionId = sid
					break
				}
			}
		}
	}
	m.mu.RUnlock()

	// The extension background.js handles 'forwardCDPCommand'
	// Use a numeric ID because background.js:175 checks: typeof msg.id === 'number'
	numericID := time.Now().UnixNano() / 1000
	commandID := fmt.Sprintf("%d", numericID)

	cmd := map[string]interface{}{
		"id":     numericID,
		"method": "forwardCDPCommand",
		"params": map[string]interface{}{
			"method":    cdpMethod,
			"params":    params,
			"sessionId": sessionId,
		},
	}
	// Use the generated ID for tracking
	trackingID := commandID

	respChan := make(chan *Response, 1)
	m.mu.Lock()
	m.pendingCalls[trackingID] = respChan
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.pendingCalls, trackingID)
		m.mu.Unlock()
	}()

	// Send command
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		m.UnregisterConnection(profile)
		return nil, fmt.Errorf("failed to send command to extension: %w", err)
	}
	fmt.Printf("[Relay] COMMAND SENT to '%s' (sid: %s): %s\n", profile, sessionId, string(data))

	// Wait for response or timeout
	select {
	case resp := <-respChan:
		if resp.Error != "" {
			return nil, fmt.Errorf("extension error: %s", resp.Error)
		}
		// If the result is a map, return it directly
		if m, ok := resp.Result.(map[string]interface{}); ok {
			return m, nil
		}
		if m, ok := resp.Payload.(map[string]interface{}); ok {
			return m, nil
		}
		// If the result is a slice (like a list of tabs), wrap it in a map
		// because the BrowserTool expects map[string]interface{} with a "tabs" key usually.
		if s, ok := resp.Result.([]interface{}); ok {
			return map[string]interface{}{"tabs": s}, nil
		}
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("timeout waiting for extension response")
	}
}

// HandleResponse processes an incoming response from the extension.
func (m *RelayManager) HandleResponse(profile string, data []byte) error {
	fmt.Printf("[Relay] Raw message: %s\n", string(data))

	var raw map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return err
	}

	// Handle Events (Forwarded CDP Events)
	method, _ := raw["method"].(string)
	if method == "forwardCDPEvent" {
		params, _ := raw["params"].(map[string]interface{})
		m.handleEvent(profile, params)
		return nil
	}

	// Handle Ping
	if method == "ping" {
		m.mu.RLock()
		conn, ok := m.connections[profile]
		m.mu.RUnlock()
		if ok {
			pong, _ := json.Marshal(map[string]string{"method": "pong"})
			_ = conn.WriteMessage(websocket.TextMessage, pong)
		}
		return nil
	}

	// Try to find an ID (extension might use 'id', 'callId', or 'uid')
	var id string
	if v, ok := raw["id"]; ok {
		id = fmt.Sprint(v)
	} else if v, ok := raw["callId"]; ok {
		id = fmt.Sprint(v)
	} else if v, ok := raw["uid"]; ok {
		id = fmt.Sprint(v)
	}

	if id == "" || id == "<nil>" {
		// Possibly an event or heartbeat we missed
		return nil
	}

	fmt.Printf("[Relay] MATCHED ID: %s\n", id)

	resp := &Response{
		ID:      id,
		Result:  raw["result"],
		Payload: raw["payload"],
		Success: raw["success"] == true,
	}
	if e, ok := raw["error"].(string); ok {
		resp.Error = e
	} else if raw["error"] != nil {
		resp.Error = fmt.Sprint(raw["error"])
	}

	m.mu.RLock()
	ch, ok := m.pendingCalls[resp.ID]
	m.mu.RUnlock()

	if ok {
		select {
		case ch <- resp:
		default:
		}
	}
	return nil
}

func (m *RelayManager) handleEvent(profile string, evt map[string]interface{}) {
	method, _ := evt["method"].(string)
	params, _ := evt["params"].(map[string]interface{})

	m.mu.Lock()
	defer m.mu.Unlock()

	switch method {
	case "Target.attachedToTarget":
		sessionId, _ := params["sessionId"].(string)
		targetInfo, _ := params["targetInfo"].(map[string]interface{})
		if sessionId != "" && targetInfo != nil {
			targetInfo["sessionId"] = sessionId // Inject sessionId for easier lookup
			if m.tabs[profile] == nil {
				m.tabs[profile] = make(map[string]map[string]interface{})
			}
			m.tabs[profile][sessionId] = targetInfo
		}
	case "Target.targetInfoChanged":
		targetInfo, _ := params["targetInfo"].(map[string]interface{})
		targetId, _ := targetInfo["targetId"].(string)
		if targetId != "" && targetInfo != nil {
			if m.tabs[profile] != nil {
				// Update the matching target in our local store
				for sid, info := range m.tabs[profile] {
					if info["targetId"] == targetId {
						m.tabs[profile][sid] = targetInfo
					}
				}
			}
		}
	case "Target.detachedFromTarget":
		sessionId, _ := params["sessionId"].(string)
		if sessionId != "" {
			if m.tabs[profile] != nil {
				delete(m.tabs[profile], sessionId)
			}
		}
	}
}

// ListProfiles returns the profiles of connected extensions.
func (m *RelayManager) ListProfiles() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profiles := make([]string, 0, len(m.connections))
	for p := range m.connections {
		profiles = append(profiles, p)
	}
	return profiles
}
