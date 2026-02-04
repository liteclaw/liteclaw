// Package gateway provides a client for communicating with the Clawdbot Gateway.
// This client can connect to both Clawdbot Gateway (TypeScript) and LiteClaw Gateway (Go)
// as they share the same WebSocket protocol.
package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ProtocolVersion is the current gateway protocol version.
const ProtocolVersion = 3

// Frame types.
const (
	FrameTypeRequest  = "req"
	FrameTypeResponse = "res"
	FrameTypeEvent    = "event"
)

// Client modes.
const (
	ClientModeBackend = "backend"
	ClientModeTUI     = "cli" // Changed from "tui" to "cli" to match Clawdbot validation
	ClientModeNode    = "node"
	ClientModeProbe   = "probe"
)

// Client names.
const (
	ClientNameGateway = "gateway-client"
	ClientNameTUI     = "cli" // Changed from "liteclaw-tui" to "cli" to match Clawdbot validation
	ClientNameAgent   = "liteclaw-agent"
)

// ConnectParams represents the connect request parameters.
type ConnectParams struct {
	MinProtocol int             `json:"minProtocol"`
	MaxProtocol int             `json:"maxProtocol"`
	Client      ClientInfo      `json:"client"`
	Caps        []string        `json:"caps,omitempty"`
	Commands    []string        `json:"commands,omitempty"`
	Permissions map[string]bool `json:"permissions,omitempty"`
	PathEnv     string          `json:"pathEnv,omitempty"`
	Role        string          `json:"role,omitempty"`
	Scopes      []string        `json:"scopes,omitempty"`
	Auth        *AuthInfo       `json:"auth,omitempty"`
	Device      *DeviceInfo     `json:"device,omitempty"`
}

// ClientInfo contains client identification.
type ClientInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
	Version     string `json:"version"`
	Platform    string `json:"platform"`
	Mode        string `json:"mode"`
	InstanceID  string `json:"instanceId,omitempty"`
}

// AuthInfo contains authentication credentials.
type AuthInfo struct {
	Token    string `json:"token,omitempty"`
	Password string `json:"password,omitempty"`
}

// DeviceInfo contains device identity for device auth.
type DeviceInfo struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
	SignedAt  int64  `json:"signedAt"`
	Nonce     string `json:"nonce,omitempty"`
}

// HelloOk is the successful connect response.
type HelloOk struct {
	Type     string      `json:"type"` // "hello-ok"
	Protocol int         `json:"protocol"`
	Server   ServerInfo  `json:"server"`
	Features Features    `json:"features"`
	Snapshot interface{} `json:"snapshot,omitempty"`
	Auth     *AuthResult `json:"auth,omitempty"`
	Policy   PolicyInfo  `json:"policy"`
}

// ServerInfo contains server information.
type ServerInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Host    string `json:"host,omitempty"`
	ConnID  string `json:"connId"`
}

// Features lists available methods and events.
type Features struct {
	Methods []string `json:"methods"`
	Events  []string `json:"events"`
}

// PolicyInfo contains connection policy.
type PolicyInfo struct {
	MaxPayload       int `json:"maxPayload"`
	MaxBufferedBytes int `json:"maxBufferedBytes"`
	TickIntervalMs   int `json:"tickIntervalMs"`
}

// AuthResult contains authentication result.
type AuthResult struct {
	DeviceToken string   `json:"deviceToken,omitempty"`
	Role        string   `json:"role,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	IssuedAtMs  int64    `json:"issuedAtMs,omitempty"`
}

// RequestFrame is a client request to the server.
type RequestFrame struct {
	Type   string      `json:"type"` // "req"
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// ResponseFrame is a server response to a request.
type ResponseFrame struct {
	Type    string      `json:"type"` // "res"
	ID      string      `json:"id"`
	OK      bool        `json:"ok"`
	Payload interface{} `json:"payload,omitempty"`
	Error   *ErrorShape `json:"error,omitempty"`
}

// EventFrame is a server-pushed event.
type EventFrame struct {
	Type    string      `json:"type"` // "event"
	Event   string      `json:"event"`
	Payload interface{} `json:"payload,omitempty"`
	Seq     int         `json:"seq,omitempty"`
}

// ErrorShape represents an error from the server.
type ErrorShape struct {
	Code         string      `json:"code"`
	Message      string      `json:"message"`
	Details      interface{} `json:"details,omitempty"`
	Retryable    bool        `json:"retryable,omitempty"`
	RetryAfterMs int         `json:"retryAfterMs,omitempty"`
}

func (e *ErrorShape) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Pending tracks a pending request.
type pending struct {
	ch chan *ResponseFrame
}

// ClientOptions configures the gateway client.
type ClientOptions struct {
	URL               string // ws://127.0.0.1:3456 (LiteClaw) or ws://127.0.0.1:18789 (Clawdbot)
	Token             string
	Password          string
	InstanceID        string
	ClientName        string
	ClientDisplayName string
	ClientVersion     string
	Platform          string
	Mode              string
	Role              string
	Scopes            []string
	Caps              []string
	Commands          []string

	OnEvent        func(evt *EventFrame)
	OnHelloOk      func(hello *HelloOk)
	OnConnectError func(err error)
	OnClose        func(code int, reason string)
}

// Client is a WebSocket client for Gateway communication.
type Client struct {
	opts     ClientOptions
	ws       *websocket.Conn
	mu       sync.RWMutex
	pending  map[string]*pending
	closed   bool
	lastSeq  int
	hello    *HelloOk
	wg       sync.WaitGroup
	closeCh  chan struct{}
	identity *DeviceIdentity
}

// NewClient creates a new Gateway client.
// Default URL is ws://127.0.0.1:3456 (LiteClaw Gateway).
// Use ws://127.0.0.1:18789 for Clawdbot Gateway.
func NewClient(opts ClientOptions) *Client {
	if opts.URL == "" {
		opts.URL = "ws://127.0.0.1:3456" // LiteClaw Gateway default port
	}
	if opts.ClientName == "" {
		opts.ClientName = ClientNameTUI
	}
	if opts.ClientVersion == "" {
		opts.ClientVersion = "dev"
	}
	if opts.Platform == "" {
		opts.Platform = "darwin" // TODO: detect
	}
	if opts.Mode == "" {
		opts.Mode = ClientModeTUI
	}
	if opts.Role == "" {
		opts.Role = "operator"
	}
	if len(opts.Scopes) == 0 {
		opts.Scopes = []string{"operator.admin"}
	}

	return &Client{
		opts:    opts,
		pending: make(map[string]*pending),
		closeCh: make(chan struct{}),
	}
}

// Connect establishes connection to the gateway.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("client is closed")
	}
	c.mu.Unlock()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// If token exists, append it to URL as query param for initial HTTP auth (upgrade)
	targetURL := c.opts.URL
	if c.opts.Token != "" {
		if strings.Contains(targetURL, "?") {
			targetURL += "&token=" + url.QueryEscape(c.opts.Token)
		} else {
			targetURL += "?token=" + url.QueryEscape(c.opts.Token)
		}
	}

	ws, _, err := dialer.DialContext(ctx, targetURL, nil)
	if err != nil {
		if c.opts.OnConnectError != nil {
			c.opts.OnConnectError(err)
		}
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.mu.Lock()
	c.ws = ws
	c.mu.Unlock()

	// Start message reader
	c.wg.Add(1)
	go c.readLoop()

	// Send connect request
	params := ConnectParams{
		MinProtocol: ProtocolVersion,
		MaxProtocol: ProtocolVersion,
		Client: ClientInfo{
			ID:          c.opts.ClientName,
			DisplayName: c.opts.ClientDisplayName,
			Version:     c.opts.ClientVersion,
			Platform:    c.opts.Platform,
			Mode:        c.opts.Mode,
			InstanceID:  c.opts.InstanceID,
		},
		Caps:     c.opts.Caps,
		Commands: c.opts.Commands,
		Role:     c.opts.Role,
		Scopes:   c.opts.Scopes,
	}

	// Device Authentication
	if c.identity == nil {
		// Generate ephemeral identity if none exists
		// TODO: Load from disk for persistence
		var err error
		c.identity, err = GenerateDeviceIdentity()
		if err != nil {
			_ = c.Close()
			return fmt.Errorf("failed to generate device identity: %w", err)
		}
	}

	signedAt := time.Now().UnixMilli()
	payload := BuildDeviceAuthPayload(
		c.identity.ID,
		c.opts.ClientName,
		c.opts.Mode,
		c.opts.Role,
		c.opts.Scopes,
		signedAt,
		c.opts.Token,
		"", // nonce (v1)
	)
	signature := SignDevicePayload(c.identity.PrivateKey, payload)

	params.Device = &DeviceInfo{
		ID:        c.identity.ID,
		PublicKey: PublicKeyToBase64Url(c.identity.PublicKey),
		Signature: signature,
		SignedAt:  signedAt,
	}

	if c.opts.Token != "" || c.opts.Password != "" {
		params.Auth = &AuthInfo{
			Token:    c.opts.Token,
			Password: c.opts.Password,
		}
	}

	result, err := c.Request(ctx, "connect", params)
	if err != nil {
		_ = c.Close()
		if c.opts.OnConnectError != nil {
			c.opts.OnConnectError(err)
		}
		return fmt.Errorf("connect failed: %w", err)
	}

	// Parse hello response
	data, _ := json.Marshal(result)
	var hello HelloOk
	if err := json.Unmarshal(data, &hello); err != nil {
		_ = c.Close()
		return fmt.Errorf("failed to parse hello: %w", err)
	}

	c.mu.Lock()
	c.hello = &hello
	c.mu.Unlock()

	if c.opts.OnHelloOk != nil {
		c.opts.OnHelloOk(&hello)
	}

	return nil
}

// Close closes the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	close(c.closeCh)

	// Flush pending requests
	for id, p := range c.pending {
		close(p.ch)
		delete(c.pending, id)
	}

	ws := c.ws
	c.ws = nil
	c.mu.Unlock()

	if ws != nil {
		_ = ws.Close()
	}

	c.wg.Wait()
	return nil
}

// Request sends a request and waits for response.
func (c *Client) Request(ctx context.Context, method string, params interface{}) (interface{}, error) {
	c.mu.RLock()
	ws := c.ws
	c.mu.RUnlock()

	if ws == nil {
		return nil, fmt.Errorf("not connected")
	}

	id := generateID()
	frame := RequestFrame{
		Type:   FrameTypeRequest,
		ID:     id,
		Method: method,
		Params: params,
	}

	ch := make(chan *ResponseFrame, 1)
	c.mu.Lock()
	c.pending[id] = &pending{ch: ch}
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	data, err := json.Marshal(frame)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closeCh:
		return nil, fmt.Errorf("client closed")
	case resp := <-ch:
		if resp == nil {
			return nil, fmt.Errorf("no response")
		}
		if !resp.OK {
			if resp.Error != nil {
				return nil, resp.Error
			}
			return nil, fmt.Errorf("request failed")
		}
		return resp.Payload, nil
	}
}

func (c *Client) readLoop() {
	defer c.wg.Done()

	for {
		c.mu.RLock()
		ws := c.ws
		closed := c.closed
		c.mu.RUnlock()

		if closed || ws == nil {
			return
		}

		_, data, err := ws.ReadMessage()
		if err != nil {
			c.mu.RLock()
			closed := c.closed
			c.mu.RUnlock()
			if !closed && c.opts.OnClose != nil {
				c.opts.OnClose(1006, err.Error())
			}
			return
		}

		c.handleMessage(data)
	}
}

func (c *Client) handleMessage(data []byte) {
	// Determine message type
	var base struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return
	}

	switch base.Type {
	case FrameTypeResponse:
		var resp ResponseFrame
		if err := json.Unmarshal(data, &resp); err != nil {
			return
		}
		c.mu.RLock()
		p, ok := c.pending[resp.ID]
		c.mu.RUnlock()
		if ok {
			select {
			case p.ch <- &resp:
			default:
			}
		}

	case FrameTypeEvent:
		var evt EventFrame
		if err := json.Unmarshal(data, &evt); err != nil {
			return
		}

		// Handle challenge for connect
		if evt.Event == "connect.challenge" {
			// Challenges are handled during connect, ignore here
			return
		}

		// Track sequence
		if evt.Seq > 0 {
			c.mu.Lock()
			c.lastSeq = evt.Seq
			c.mu.Unlock()
		}

		if c.opts.OnEvent != nil {
			c.opts.OnEvent(&evt)
		}
	}
}

// IsConnected returns true if connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ws != nil && !c.closed
}

// Hello returns the hello response.
func (c *Client) Hello() *HelloOk {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hello
}

// generateID creates a unique request ID.
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // error ignored: crypto/rand.Read always succeeds
	return hex.EncodeToString(b)
}

// === Convenience methods for common operations ===

// SessionsList lists sessions.
func (c *Client) SessionsList(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return c.Request(ctx, "sessions.list", params)
}

// SessionsPreview gets session preview.
func (c *Client) SessionsPreview(ctx context.Context, sessionKey string) (interface{}, error) {
	return c.Request(ctx, "sessions.preview", map[string]interface{}{
		"sessionKey": sessionKey,
	})
}

// ChatSend sends a chat message.
func (c *Client) ChatSend(ctx context.Context, sessionKey, message string) (interface{}, error) {
	return c.Request(ctx, "chat.send", map[string]interface{}{
		"sessionKey": sessionKey,
		"message":    message,
	})
}

// ChatHistory gets chat history.
func (c *Client) ChatHistory(ctx context.Context, sessionKey string, limit int) (interface{}, error) {
	return c.Request(ctx, "chat.history", map[string]interface{}{
		"sessionKey": sessionKey,
		"limit":      limit,
	})
}

// Agent sends an agent request.
func (c *Client) Agent(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return c.Request(ctx, "agent", params)
}

// AgentWait waits for an agent run to complete.
func (c *Client) AgentWait(ctx context.Context, runID string, timeoutMs int) (interface{}, error) {
	return c.Request(ctx, "agent.wait", map[string]interface{}{
		"runId":     runID,
		"timeoutMs": timeoutMs,
	})
}

// NodesList lists connected nodes.
func (c *Client) NodesList(ctx context.Context) (interface{}, error) {
	return c.Request(ctx, "nodes.list", nil)
}

// NodeInvoke invokes a command on a node.
func (c *Client) NodeInvoke(ctx context.Context, nodeID, command string, params map[string]interface{}) (interface{}, error) {
	return c.Request(ctx, "node.invoke", map[string]interface{}{
		"nodeId":  nodeID,
		"command": command,
		"params":  params,
	})
}

// CronList lists cron jobs.
func (c *Client) CronList(ctx context.Context, includeDisabled bool) (interface{}, error) {
	return c.Request(ctx, "cron.list", map[string]interface{}{
		"includeDisabled": includeDisabled,
	})
}

// ConfigGet gets configuration.
func (c *Client) ConfigGet(ctx context.Context, path string) (interface{}, error) {
	return c.Request(ctx, "config.get", map[string]interface{}{
		"path": path,
	})
}

// ConfigPatch patches configuration.
func (c *Client) ConfigPatch(ctx context.Context, patches []map[string]interface{}) (interface{}, error) {
	return c.Request(ctx, "config.patch", map[string]interface{}{
		"patches": patches,
	})
}

// ChannelsStatus gets channel status.
func (c *Client) ChannelsStatus(ctx context.Context) (interface{}, error) {
	return c.Request(ctx, "channels.status", nil)
}

// ModelsList lists available models.
func (c *Client) ModelsList(ctx context.Context) (interface{}, error) {
	return c.Request(ctx, "models.list", nil)
}

// AgentsList lists available agents.
func (c *Client) AgentsList(ctx context.Context) (interface{}, error) {
	return c.Request(ctx, "agents.list", nil)
}

// SkillsStatus gets skills status.
func (c *Client) SkillsStatus(ctx context.Context) (interface{}, error) {
	return c.Request(ctx, "skills.status", nil)
}
