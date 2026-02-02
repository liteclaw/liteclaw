package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
)

// Client is an MCP client implementation
type Client struct {
	config ServerConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	idGen   int64
	pending sync.Map // id -> chan *JSONRPCResponse

	cancel  context.CancelFunc
	Verbose bool
}

// NewClient creates a new MCP client for a server config
func NewClient(cfg ServerConfig) *Client {
	return &Client{
		config: cfg,
	}
}

// Start launches the MCP server process
func (c *Client) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.cmd = exec.CommandContext(ctx, c.config.Command, c.config.Args...)

	// Set environment variables
	if len(c.config.Env) > 0 {
		c.cmd.Env = os.Environ()
		for k, v := range c.config.Env {
			c.cmd.Env = append(c.cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return err
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	// Capture stderr for debugging
	if c.Verbose {
		c.cmd.Stderr = os.Stderr
	} else {
		// Silent stderr to avoid terminal interference
		c.cmd.Stderr = nil
	}

	if c.Verbose {
		fmt.Printf("[MCP %s] Starting: %s %v\n", c.config.Command, c.config.Command, c.config.Args)
	}
	if err := c.cmd.Start(); err != nil {
		return err
	}

	go c.readLoop()

	// Initialize
	initReq := InitializeRequest{
		ProtocolVersion: "2024-11-05",
		Capabilities:    ClientCapabilities{},
		ClientInfo: Implementation{
			Name:    "LiteClaw",
			Version: "1.0.0",
		},
	}

	var result InitializeResult
	if err := c.Call(ctx, "initialize", initReq, &result); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	// Send initialized notification (no ID)
	if err := c.Notify("notifications/initialized", nil); err != nil {
		return err
	}

	return nil
}

func (c *Client) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil {
		return c.cmd.Wait()
	}
	return nil
}

func (c *Client) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
	id := atomic.AddInt64(&c.idGen, 1)

	paramBytes, err := json.Marshal(params)
	if err != nil {
		return err
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramBytes,
		ID:      id,
	}

	respChan := make(chan *JSONRPCResponse, 1)
	c.pending.Store(id, respChan)
	defer c.pending.Delete(id)

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return err
	}

	if c.Verbose {
		fmt.Printf("[MCP SEND] %s\n", string(reqBytes))
	}
	if _, err := c.stdin.Write(append(reqBytes, '\n')); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-respChan:
		if resp.Error != nil {
			return fmt.Errorf("MCP error (%d): %s", resp.Error.Code, resp.Error.Message)
		}
		if result != nil {
			return json.Unmarshal(resp.Result, result)
		}
		return nil
	}
}

func (c *Client) Notify(method string, params interface{}) error {
	var paramBytes json.RawMessage
	if params != nil {
		var err error
		paramBytes, err = json.Marshal(params)
		if err != nil {
			return err
		}
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramBytes,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return err
	}

	if c.Verbose {
		fmt.Printf("[MCP SEND] %s\n", string(reqBytes))
	}
	_, err = c.stdin.Write(append(reqBytes, '\n'))
	return err
}

func (c *Client) readLoop() {
	// Use a pipe to bridge the scanner with a decoder if needed, or just decode directly from stdout
	// Actually, we need to read line by line for MCP stdio.
	scanner := bufio.NewScanner(c.stdout)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if c.Verbose {
			fmt.Printf("[MCP RECV] %s\n", string(line))
		}

		var resp JSONRPCResponse
		decoder := json.NewDecoder(strings.NewReader(string(line)))
		decoder.UseNumber()
		if err := decoder.Decode(&resp); err != nil {
			fmt.Printf("[MCP RECV ERROR] Failed to decode: %v\n", err)
			continue
		}

		if resp.ID == nil {
			if c.Verbose {
				fmt.Printf("[MCP NOTIF/REQ] Method: %s\n", resp.Method)
			}
			continue
		}

		var id int64
		switch v := resp.ID.(type) {
		case float64:
			id = int64(v)
		case json.Number:
			id, _ = v.Int64()
		case int64:
			id = v
		case string:
			_, _ = fmt.Sscanf(v, "%d", &id)
		default:
			fmt.Printf("[MCP RECV] Unknown ID type: %T for %v\n", v, v)
			continue
		}

		if ch, ok := c.pending.Load(id); ok {
			ch.(chan *JSONRPCResponse) <- &resp
		} else {
			fmt.Printf("[MCP RECV] No pending call for ID %d\n", id)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("[MCP %s] Scanner error: %v\n", c.config.Command, err)
	}

	// Connection closed, clean up all pending calls
	c.pending.Range(func(key, value interface{}) bool {
		ch := value.(chan *JSONRPCResponse)
		ch <- &JSONRPCResponse{
			Error: &JSONRPCError{
				Code:    -32000,
				Message: "Connection closed",
			},
		}
		return true
	})
}
