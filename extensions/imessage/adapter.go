package imessage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Config defines the configuration for the iMessage adapter
type Config struct {
	Enabled bool
	CLIPath string // Path to imsg CLI, defaults to "imsg"
	DBPath  string // Path to chat.db (optional)
}

// Adapter implements the channels.Adapter interface for iMessage using RPC
type Adapter struct {
	*channels.BaseAdapter

	// Configuration
	cliPath string
	dbPath  string

	// RPC client state
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	pending   map[int64]*pendingRequest
	pendingMu sync.Mutex
	nextID    int64
	closed    chan struct{}

	// Subscription state
	subscriptionID atomic.Int64
	cancelFunc     context.CancelFunc
	mu             sync.Mutex
}

type pendingRequest struct {
	resolve chan json.RawMessage
	reject  chan error
}

type rpcRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int64                  `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      *int64          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Data    string `json:"data,omitempty"`
}

type messageNotification struct {
	Message *IMessagePayload `json:"message,omitempty"`
}

// IMessagePayload matches the TS version's message structure
type IMessagePayload struct {
	ID             int64        `json:"id,omitempty"`
	GUID           string       `json:"guid,omitempty"`
	Text           string       `json:"text,omitempty"`
	Sender         string       `json:"sender,omitempty"`
	IsFromMe       bool         `json:"is_from_me,omitempty"`
	IsGroup        bool         `json:"is_group,omitempty"`
	ChatID         *int64       `json:"chat_id,omitempty"`
	ChatGUID       string       `json:"chat_guid,omitempty"`
	ChatIdentifier string       `json:"chat_identifier,omitempty"`
	ChatName       string       `json:"chat_name,omitempty"`
	CreatedAt      string       `json:"created_at,omitempty"`
	Attachments    []Attachment `json:"attachments,omitempty"`
	Participants   []string     `json:"participants,omitempty"`
	ReplyToID      string       `json:"reply_to_id,omitempty"`
	ReplyToText    string       `json:"reply_to_text,omitempty"`
	ReplyToSender  string       `json:"reply_to_sender,omitempty"`
}

type Attachment struct {
	OriginalPath string `json:"original_path,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	Missing      bool   `json:"missing,omitempty"`
}

// New creates a new iMessage adapter
func New(cfg *Config, logger zerolog.Logger) *Adapter {
	if cfg.CLIPath == "" {
		cfg.CLIPath = "imsg"
	}

	caps := &channels.Capabilities{
		ChatTypes:      []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup},
		Reactions:      false,
		Threads:        false,
		Media:          true,
		Stickers:       false,
		Voice:          false,
		NativeCommands: true,
		BlockStreaming: false,
		Webhooks:       false,
		Polling:        false, // Now using RPC subscription, not polling
	}

	baseCfg := &channels.Config{
		Enabled: cfg.Enabled,
	}

	base := channels.NewBaseAdapter(
		"imessage",
		"iMessage",
		channels.ChannelTypeIMessage,
		caps,
		baseCfg,
		logger,
	)

	return &Adapter{
		BaseAdapter: base,
		cliPath:     cfg.CLIPath,
		dbPath:      cfg.DBPath,
		pending:     make(map[int64]*pendingRequest),
		closed:      make(chan struct{}),
	}
}

// Start starts the adapter (RPC subscription mode)
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.IsRunning() {
		return nil
	}

	a.Logger().Info().Str("cli_path", a.cliPath).Msg("Starting iMessage adapter (RPC mode)")

	// Build command args
	args := []string{"rpc"}
	if a.dbPath != "" {
		args = append(args, "--db", a.dbPath)
	}

	// Start the imsg rpc process
	cmd := exec.Command(a.cliPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start imsg rpc: %w", err)
	}

	a.cmd = cmd
	a.stdin = stdin
	a.stdout = stdout
	a.stderr = stderr
	a.closed = make(chan struct{})
	a.pending = make(map[int64]*pendingRequest)

	// Start reading stdout (responses and notifications)
	go a.readLoop()

	// Start reading stderr (errors)
	go a.readStderr()

	// Wait for process exit
	go a.waitForClose()

	// Subscribe to messages
	runCtx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel

	go func() {
		if err := a.subscribe(runCtx); err != nil {
			a.Logger().Error().Err(err).Msg("Failed to subscribe to iMessage")
		}
	}()

	a.SetRunning(true)
	now := time.Now()
	a.State().LastStartAt = &now
	a.State().Mode = "rpc"

	return nil
}

// subscribe subscribes to message notifications
func (a *Adapter) subscribe(ctx context.Context) error {
	// Wait a bit for the process to be ready
	time.Sleep(100 * time.Millisecond)

	// Subscribe to watch.subscribe
	result, err := a.request("watch.subscribe", map[string]interface{}{
		"attachments": true,
	}, 10*time.Second)
	if err != nil {
		return fmt.Errorf("watch.subscribe failed: %w", err)
	}

	// Parse subscription ID
	var subResult struct {
		Subscription int64 `json:"subscription,omitempty"`
	}
	if err := json.Unmarshal(result, &subResult); err == nil {
		a.subscriptionID.Store(subResult.Subscription)
		a.Logger().Info().Int64("subscription_id", subResult.Subscription).Msg("Subscribed to iMessage")
	}

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

// Stop stops the adapter
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.IsRunning() {
		return nil
	}

	// Cancel subscription context
	if a.cancelFunc != nil {
		a.cancelFunc()
		a.cancelFunc = nil
	}

	// Unsubscribe if we have a subscription
	if subID := a.subscriptionID.Load(); subID != 0 {
		_, _ = a.request("watch.unsubscribe", map[string]interface{}{
			"subscription": subID,
		}, 2*time.Second)
	}

	// Close stdin to signal process to exit
	if a.stdin != nil {
		_ = a.stdin.Close()
		a.stdin = nil
	}

	// Wait for process to exit or timeout
	if a.cmd != nil {
		done := make(chan struct{})
		go func() {
			_ = a.cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			_ = a.cmd.Process.Kill()
		}
		a.cmd = nil
	}

	// Close the closed channel to signal readLoop to exit
	select {
	case <-a.closed:
	default:
		close(a.closed)
	}

	a.SetRunning(false)
	now := time.Now()
	a.State().LastStopAt = &now

	a.Logger().Info().Msg("iMessage adapter stopped")
	return nil
}

// Connect implements Adapter interface
func (a *Adapter) Connect(ctx context.Context) error {
	return a.Start(ctx)
}

// Disconnect implements Adapter interface
func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.Stop(ctx)
}

// IsConnected implements Adapter interface
func (a *Adapter) IsConnected() bool {
	return a.IsRunning()
}

// Probe implements Adapter interface
func (a *Adapter) Probe(ctx context.Context) (*channels.ProbeResult, error) {
	start := time.Now()

	// Try to run imsg --version or similar
	cmd := exec.CommandContext(ctx, a.cliPath, "--version")
	_, err := cmd.CombinedOutput()
	if err != nil {
		return &channels.ProbeResult{
			OK:    false,
			Error: fmt.Sprintf("imsg not available: %v", err),
		}, nil
	}

	return &channels.ProbeResult{
		OK:        true,
		BotName:   "iMessage via imsg",
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

// Send sends a message via imsg CLI
func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) {
	if a.cmd == nil {
		return &channels.SendResult{Success: false, Error: "imsg rpc not running"}, fmt.Errorf("imsg rpc not running")
	}

	target := req.To.ChatID
	message := req.Text

	// Use RPC to send message
	result, err := a.request("send", map[string]interface{}{
		"to":   target,
		"text": message,
	}, 30*time.Second)

	if err != nil {
		a.Logger().Error().Err(err).Str("to", target).Msg("Failed to send iMessage")
		return &channels.SendResult{Success: false, Error: err.Error()}, err
	}

	now := time.Now()
	a.State().LastOutboundAt = &now
	a.State().MessageCount++

	a.Logger().Info().Str("to", target).RawJSON("result", result).Msg("Sent iMessage")
	return &channels.SendResult{Success: true, MessageID: "sent-via-rpc"}, nil
}

// SendReaction implements Adapter interface (Placeholder)
func (a *Adapter) SendReaction(ctx context.Context, req *channels.ReactionRequest) error {
	return fmt.Errorf("reactions not supported yet for iMessage")
}

// request sends a JSON-RPC request and waits for response
func (a *Adapter) request(method string, params map[string]interface{}, timeout time.Duration) (json.RawMessage, error) {
	if a.stdin == nil {
		return nil, fmt.Errorf("imsg rpc not running")
	}

	id := atomic.AddInt64(&a.nextID, 1)

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	data = append(data, '\n')

	// Register pending request
	pending := &pendingRequest{
		resolve: make(chan json.RawMessage, 1),
		reject:  make(chan error, 1),
	}
	a.pendingMu.Lock()
	a.pending[id] = pending
	a.pendingMu.Unlock()

	// Clean up on exit
	defer func() {
		a.pendingMu.Lock()
		delete(a.pending, id)
		a.pendingMu.Unlock()
	}()

	// Send request
	if _, err := a.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Wait for response with timeout
	select {
	case result := <-pending.resolve:
		return result, nil
	case err := <-pending.reject:
		return nil, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for response to %s", method)
	case <-a.closed:
		return nil, fmt.Errorf("connection closed")
	}
}

// readLoop reads responses and notifications from stdout
func (a *Adapter) readLoop() {
	scanner := bufio.NewScanner(a.stdout)
	// Increase buffer size for large messages
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		a.handleLine(line)
	}

	if err := scanner.Err(); err != nil {
		a.Logger().Error().Err(err).Msg("Error reading from imsg rpc stdout")
	}

	// Signal closed
	select {
	case <-a.closed:
	default:
		close(a.closed)
	}
}

// handleLine processes a single JSON-RPC line
func (a *Adapter) handleLine(line string) {
	var resp rpcResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		a.Logger().Error().Err(err).Str("line", line).Msg("Failed to parse imsg rpc response")
		return
	}

	// Check if this is a response (has ID)
	if resp.ID != nil {
		a.pendingMu.Lock()
		pending, ok := a.pending[*resp.ID]
		a.pendingMu.Unlock()

		if ok {
			if resp.Error != nil {
				pending.reject <- fmt.Errorf("%s", resp.Error.Message)
			} else {
				pending.resolve <- resp.Result
			}
		}
		return
	}

	// This is a notification
	switch resp.Method {
	case "message":
		a.handleMessageNotification(resp.Params)
	case "error":
		a.Logger().Error().RawJSON("params", resp.Params).Msg("imsg rpc error notification")
	}
}

// handleMessageNotification processes incoming message notifications
func (a *Adapter) handleMessageNotification(params json.RawMessage) {
	var notification messageNotification
	if err := json.Unmarshal(params, &notification); err != nil {
		a.Logger().Error().Err(err).Msg("Failed to parse message notification")
		return
	}

	msg := notification.Message
	if msg == nil {
		return
	}

	// Skip messages from ourselves
	if msg.IsFromMe {
		return
	}

	sender := msg.Sender
	if sender == "" {
		return
	}

	text := msg.Text
	if text == "" && len(msg.Attachments) == 0 {
		return
	}

	a.Logger().Info().
		Str("sender", sender).
		Str("text", text).
		Bool("is_group", msg.IsGroup).
		Msg("New iMessage received")

	// Determine chat type
	chatType := "direct"
	if msg.IsGroup {
		chatType = "group"
	}

	// Build chat ID
	chatID := sender
	if msg.ChatID != nil {
		chatID = fmt.Sprintf("chat:%d", *msg.ChatID)
	} else if msg.ChatGUID != "" {
		chatID = msg.ChatGUID
	}

	// Delegate to handler
	handler := a.Handler()
	if handler != nil {
		inMsg := &channels.IncomingMessage{
			ID:          msg.GUID,
			Text:        text,
			SenderID:    sender,
			SenderName:  sender,
			ChatID:      chatID,
			ChannelType: "imessage",
			ChatType:    chatType,
		}

		// Update state
		now := time.Now()
		a.State().LastInboundAt = &now

		go func(m *channels.IncomingMessage) {
			if err := handler.HandleIncoming(context.Background(), m); err != nil {
				a.Logger().Error().Err(err).Msg("Handler failed processing iMessage")
			}
		}(inMsg)
	}
}

// readStderr reads error output from the imsg process
func (a *Adapter) readStderr() {
	scanner := bufio.NewScanner(a.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			a.Logger().Warn().Str("stderr", line).Msg("imsg rpc stderr")
		}
	}
}

// waitForClose waits for the process to exit
func (a *Adapter) waitForClose() {
	if a.cmd != nil {
		_ = a.cmd.Wait()
	}
	// Signal closed
	select {
	case <-a.closed:
	default:
		close(a.closed)
	}
}
