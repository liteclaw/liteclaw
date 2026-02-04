package tui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"

	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/liteclaw/liteclaw/internal/gateway"
)

// DefaultGatewayPort is the default LiteClaw Gateway port.
const DefaultGatewayPort = 3456

// ClawdbotGatewayPort is the Clawdbot Gateway port.
const ClawdbotGatewayPort = 18789

// Config holds TUI configuration.
type Config struct {
	Host  string // Gateway host (default: localhost)
	Port  int    // Gateway port (default: 3456 for LiteClaw, 18789 for Clawdbot)
	Token string // Gateway authentication token
}

// gatewayAddr stores the configured gateway address.
var gatewayAddr = "localhost:3456"
var gatewayToken = ""

type model struct {
	viewport  viewport.Model
	textInput textinput.Model
	messages  []string
	err       error
	ready     bool

	// State
	connecting bool
	connected  bool // socket is open
	masking    bool // waiting for handshake
	conn       *websocket.Conn

	// Gateway target
	gatewayAddr string

	// Streaming state
	lastRunID string
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()
	ti.CharLimit = 1000000
	ti.Width = 20

	return model{
		textInput:   ti,
		messages:    []string{},
		connecting:  true,
		masking:     true, // hide UI interactions until fully ready
		gatewayAddr: gatewayAddr,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		connectGateway,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textInput, tiCmd = m.textInput.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := 1
		footerHeight := 3
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.textInput.Width = msg.Width - 2
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
			m.textInput.Width = msg.Width - 2
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.conn != nil {
				_ = m.conn.Close()
			}
			return m, tea.Quit
		case tea.KeyEnter:
			if m.textInput.Value() != "" && !m.masking {
				val := m.textInput.Value()
				m.messages = append(m.messages, fmt.Sprintf("%s %s", senderStyle.Render("You:"), val))
				m.textInput.SetValue("")
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
				m.viewport.GotoBottom()

				if m.connected && m.conn != nil {
					return m, sendMessage(m.conn, val)
				}
			}
		}

	case connectedMsg:
		m.connected = true
		m.conn = msg.conn
		m.messages = append(m.messages, infoStyle.Render("✓ Socket Connected - Negotiating..."))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))

		// Start listening and send handshake
		return m, tea.Batch(waitForMessage(m.conn), sendHandshake(m.conn))

	case handshakeOkMsg:
		m.connecting = false
		m.masking = false
		m.messages = append(m.messages, infoStyle.Render("✓ Gateway Ready"))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
		return m, waitForMessage(m.conn)

	case incomingMessageMsg:
		sender := senderStyle.Render("Gateway:")
		newMessage := fmt.Sprintf("%s %s", sender, msg.content)

		if msg.runID != "" && msg.runID == m.lastRunID && len(m.messages) > 0 {
			// Update the last message instead of appending (Streaming)
			m.messages[len(m.messages)-1] = newMessage
		} else {
			// Append new message
			m.messages = append(m.messages, newMessage)
			m.lastRunID = msg.runID
		}

		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
		return m, waitForMessage(m.conn)

	case errMsg:
		m.err = msg
		m.messages = append(m.messages, infoStyle.Render(fmt.Sprintf("Error: %v", msg)))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		// If connection failed, maybe retry or quit. For now just show error.
		m.connecting = false
		m.masking = false
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	header := m.headerView()
	footer := m.footerView()

	return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)
}

func (m model) headerView() string {
	title := "LiteClaw TUI"
	status := "Disconnected"
	if m.connected && !m.masking {
		status = "Connected"
	} else if m.connected {
		status = "Handshaking..."
	} else if m.connecting {
		status = "Connecting..."
	}

	line := strings.Repeat("─", maximum(0, m.viewport.Width-len(title)-len(status)-2))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line, status)
}

func (m model) footerView() string {
	if m.masking {
		return infoStyle.Render("Waiting for connection...")
	}
	return infoStyle.Render(m.textInput.View()) // + help
}

func maximum(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Messages
type connectedMsg struct{ conn *websocket.Conn }
type handshakeOkMsg struct{}
type incomingMessageMsg struct {
	content string
	runID   string
	state   string
}
type errMsg error

// Commands
func connectGateway() tea.Msg {
	// Connect to LiteClaw/Clawdbot Gateway
	// Default: localhost:3456 (LiteClaw Gateway)
	// Can be configured via Run() to connect to remote Clawdbot Gateway (port 18789)
	// Both LiteClaw and Clawdbot Gateway use root path for WebSocket
	u := url.URL{Scheme: "ws", Host: gatewayAddr, Path: ""}
	if gatewayToken != "" {
		q := u.Query()
		q.Set("token", gatewayToken)
		u.RawQuery = q.Encode()
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return errMsg(err)
	}
	return connectedMsg{conn: conn}
}

func sendHandshake(conn *websocket.Conn) tea.Cmd {
	return func() tea.Msg {
		// Use Clawdbot Gateway protocol version 4
		req := map[string]interface{}{
			"type":   "req",
			"id":     "handshake-1",
			"method": "connect",
			"params": map[string]interface{}{
				"minProtocol": 3,
				"maxProtocol": 3,
				"client": map[string]interface{}{
					"id":          "cli", // Must be "cli" to pass Clawdbot validation
					"displayName": "LiteClaw TUI",
					"version":     "0.1.0",
					"platform":    "darwin",
					"mode":        "cli", // Must be "cli" to pass Clawdbot validation
				},
				"caps":   []string{},
				"role":   "operator",
				"scopes": []string{"operator.admin"},
			},
		}

		// Add authentication token if provided
		if gatewayToken != "" {
			req["params"].(map[string]interface{})["auth"] = map[string]interface{}{
				"token": gatewayToken,
			}
		}

		// Generate device identity and sign payload
		identity, err := gateway.GenerateDeviceIdentity()
		if err != nil {
			return errMsg(err)
		}

		signedAt := time.Now().UnixMilli()
		payload := gateway.BuildDeviceAuthPayload(
			identity.ID,
			"cli", // client.id
			"cli", // client.mode
			"operator",
			[]string{"operator.admin"},
			signedAt,
			gatewayToken, // token - MUST be included in signature if present
			"",           // nonce
		)
		signature := gateway.SignDevicePayload(identity.PrivateKey, payload)

		// Add device info to params
		params := req["params"].(map[string]interface{})
		params["device"] = map[string]interface{}{
			"id":        identity.ID,
			"publicKey": gateway.PublicKeyToBase64Url(identity.PublicKey),
			"signature": signature,
			"signedAt":  signedAt,
		}

		if err := conn.WriteJSON(req); err != nil {
			return errMsg(err)
		}
		return nil
	}
}

func sendMessage(conn *websocket.Conn, content string) tea.Cmd {
	return func() tea.Msg {
		id := fmt.Sprintf("%d", time.Now().UnixNano())
		req := map[string]interface{}{
			"type":   "req",
			"id":     id,
			"method": "chat.send",
			"params": map[string]interface{}{
				"sessionKey":     "main",
				"message":        content,
				"idempotencyKey": id, // Use the request ID as idempotency key
				"deliver":        true,
			},
		}

		err := conn.WriteJSON(req)
		if err != nil {
			return errMsg(err)
		}
		return nil
	}
}

func waitForMessage(conn *websocket.Conn) tea.Cmd {
	return func() tea.Msg {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return errMsg(err)
		}

		var frame struct {
			Type    string                 `json:"type"`
			ID      string                 `json:"id"`
			OK      bool                   `json:"ok"`
			Event   string                 `json:"event"`
			Payload map[string]interface{} `json:"payload"`
		}
		if err := json.Unmarshal(message, &frame); err == nil {
			// Check for handshake response
			if frame.Type == "res" && frame.ID == "handshake-1" && frame.OK {
				return handshakeOkMsg{}
			}

			if frame.Type == "event" && frame.Event == "chat" {
				runID, _ := frame.Payload["runId"].(string)
				state, _ := frame.Payload["state"].(string)

				if msg, ok := frame.Payload["message"].(map[string]interface{}); ok {
					if content, ok := msg["content"].([]interface{}); ok && len(content) > 0 {
						if firstBlock, ok := content[0].(map[string]interface{}); ok {
							if text, ok := firstBlock["text"].(string); ok {
								return incomingMessageMsg{
									content: text,
									runID:   runID,
									state:   state,
								}
							}
						}
					}
				}
			}
			// Ignore non-chat events or responses for now
			return waitForMessage(conn)()
		}

		return incomingMessageMsg{content: string(message)}
	}
}

// Run starts the TUI with default gateway address (localhost:3456).
func Run() error {
	return RunWithConfig(nil)
}

// RunWithConfig starts the TUI with custom configuration.
// If host is empty, defaults to "localhost".
// If port is 0, reads from config file, then defaults to 18789 (Clawdbot Gateway).
func RunWithConfig(cfg *Config) error {
	// Set gateway address
	host := "localhost"
	port := 0

	if cfg != nil {
		if cfg.Host != "" {
			host = cfg.Host
		}
		if cfg.Port > 0 {
			port = cfg.Port
		}
		if cfg.Token != "" {
			gatewayToken = cfg.Token
		}
	}

	// If port not explicitly set, try to load from config file
	if port == 0 {
		if loadedCfg, err := config.Load(); err == nil && loadedCfg.Gateway.Port > 0 {
			port = loadedCfg.Gateway.Port
		}
	}

	// Default to ClawdbotGatewayPort if still not set
	if port == 0 {
		port = ClawdbotGatewayPort
	}

	// Try to load token from environment or config if not set
	if gatewayToken == "" {
		token, _ := gateway.LoadClawdbotToken()
		if token != "" {
			gatewayToken = token
		}
	}

	gatewayAddr = fmt.Sprintf("%s:%d", host, port)

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
