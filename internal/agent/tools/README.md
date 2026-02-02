# LiteClaw Agent Tools

This document describes all available tools in the LiteClaw agent system. These tools have been ported from the TypeScript Clawdbot implementation.

## Tool Categories

### 📁 File System Tools

| Tool | File | Description |
|------|------|-------------|
| `read` | `file.go` | Read file contents with optional line range |
| `write` | `file.go` | Write content to files (create/overwrite/append) |
| `list` | `file.go` | List directory contents recursively |
| `edit` | `edit.go` | Perform targeted text replacement in files |

### 🔍 Search Tools

| Tool | File | Description |
|------|------|-------------|
| `grep` | `search.go` | Search file contents using ripgrep or built-in regex |
| `find` | `search.go` | Find files by name pattern |

### ⚡ Execution Tools

| Tool | File | Description |
|------|------|-------------|
| `exec` | `exec.go` | Execute shell commands with timeout support |
| `process` | `process.go` | Manage background processes (status/list/kill/output) |

### 🌐 Web Tools

| Tool | File | Description |
|------|------|-------------|
| `web_search` | `web.go` | Search the web using Brave Search API |
| `web_fetch` | `web.go` | Fetch and extract content from URLs |

### 🖥️ Browser & UI Tools

| Tool | File | Description |
|------|------|-------------|
| `browser` | `browser.go` | Full browser automation (tabs, navigation, clicks, screenshots) |
| `canvas` | `nodes.go` | Control node canvas displays (present/hide/navigate/eval/snapshot) |
| `nodes` | `nodes.go` | Manage gateway nodes (devices, displays, services) |
| `tts` | `nodes.go` | Text-to-speech audio generation |

### 🎨 Media Tools

| Tool | File | Description |
|------|------|-------------|
| `image` | `media.go` | Analyze images using vision models |
| `message` | `media.go` | Send messages via channel plugins (Telegram, Discord, etc.) |

### 🧠 Memory Tools

| Tool | File | Description |
|------|------|-------------|
| `memory_search` | `memory.go` | Search MEMORY.md and memory/*.md files semantically |
| `memory_get` | `memory.go` | Read specific lines from memory files |

### 📱 Session Tools

| Tool | File | Description |
|------|------|-------------|
| `sessions_list` | `sessions.go` | List available agent sessions |
| `sessions_send` | `sessions.go` | Send messages to other sessions |
| `sessions_spawn` | `sessions.go` | Spawn new agent sessions (subagents) |
| `sessions_history` | `sessions.go` | Retrieve session message history |

### ⚙️ System Tools

| Tool | File | Description |
|------|------|-------------|
| `cron` | `system.go` | Manage scheduled tasks (add/update/remove/run) |
| `agents_list` | `system.go` | List available agents |
| `gateway` | `system.go` | Call Gateway API methods directly |
| `session_status` | `system.go` | Check session status |

## Usage

### Creating a Default Registry

```go
import "github.com/liteclaw/liteclaw/internal/agent/tools"

// Create a registry with all standard tools
registry := tools.NewDefaultRegistry(&tools.RegistryOptions{
    AgentDir: "/path/to/agent",
})

// List all available tools
for _, name := range registry.Names() {
    fmt.Println(name)
}
```

### Using Individual Tools

```go
// Get a specific tool
searchTool, ok := registry.Get("web_search")
if ok {
    result, err := searchTool.Execute(ctx, map[string]interface{}{
        "query": "Go programming",
        "count": 5,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Results: %+v\n", result)
}
```

### Creating a Custom Registry

```go
// Create an empty registry
registry := tools.NewRegistry()

// Register only the tools you need
registry.Register(tools.NewReadTool())
registry.Register(tools.NewWriteTool())
registry.Register(tools.NewExecTool())
```

## Environment Variables

Some tools require environment variables to be configured:

| Variable | Tool | Description |
|----------|------|-------------|
| `BRAVE_API_KEY` | `web_search` | Brave Search API key |
| `BROWSER_CONTROL_URL` | `browser` | Browser control server URL |
| `ALLOW_HOST_BROWSER_CONTROL` | `browser` | Allow controlling host browser (true/false) |
| `GATEWAY_URL` | `canvas`, `nodes` | Gateway server URL |
| `GATEWAY_TOKEN` | `canvas`, `nodes` | Gateway authentication token |

## Tool Interface

All tools implement the `Tool` interface:

```go
type Tool interface {
    // Name returns the tool name (used by the LLM)
    Name() string
    
    // Description returns a description for the LLM
    Description() string
    
    // Parameters returns the JSON Schema for tool parameters
    Parameters() interface{}
    
    // Execute runs the tool with the given parameters
    Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)
}
```

## Tool Comparison: TypeScript vs Go

| TypeScript Tool | Go Implementation | Status |
|-----------------|-------------------|--------|
| `read` | ✅ `NewReadTool()` | Complete |
| `write` | ✅ `NewWriteTool()` | Complete |
| `list` | ✅ `NewListTool()` | Complete |
| `edit` | ✅ `NewEditTool()` | Complete |
| `exec` | ✅ `NewExecTool()` | Complete |
| `process` | ✅ `NewProcessTool()` | Complete |
| `grep` | ✅ `NewGrepTool()` | Complete |
| `find` | ✅ `NewFindTool()` | Complete |
| `web_search` | ✅ `NewWebSearchTool()` | Complete (Brave API) |
| `web_fetch` | ✅ `NewWebFetchTool()` | Complete |
| `browser` | ✅ `NewBrowserTool()` | Complete |
| `canvas` | ✅ `NewCanvasTool()` | Complete |
| `nodes` | ✅ `NewNodesTool()` | Complete |
| `tts` | ✅ `NewTtsTool()` | Structure ready |
| `image` | ✅ `NewImageTool()` | Structure ready |
| `message` | ✅ `NewMessageTool()` | Structure ready |
| `memory_search` | ✅ `NewMemorySearchTool()` | Complete |
| `memory_get` | ✅ `NewMemoryGetTool()` | Complete |
| `sessions_list` | ✅ `NewSessionsListTool()` | Structure ready |
| `sessions_send` | ✅ `NewSessionsSendTool()` | Structure ready |
| `sessions_spawn` | ✅ `NewSessionsSpawnTool()` | Structure ready |
| `sessions_history` | ✅ `NewSessionsHistoryTool()` | Structure ready |
| `cron` | ✅ `NewCronTool()` | Structure ready |
| `agents_list` | ✅ `NewAgentsListTool()` | Structure ready |
| `gateway` | ✅ `NewGatewayTool()` | Structure ready |
| `session_status` | ✅ `NewSessionStatusTool()` | Structure ready |

## Internal Integration

Tools that require session or agent management (like `sessions_*`, `cron`, etc.) define a `GatewayClient` interface. This interface should be implemented by LiteClaw's internal gateway/session manager:

```go
// GatewayClient defines the interface for gateway communication.
type GatewayClient interface {
    ListSessions(ctx context.Context, params map[string]interface{}) ([]map[string]interface{}, error)
    GetSessionHistory(ctx context.Context, sessionKey string, limit int) ([]map[string]interface{}, error)
    SendMessage(ctx context.Context, sessionKey, message string, opts map[string]interface{}) (map[string]interface{}, error)
    WaitForRun(ctx context.Context, runID string, timeoutMs int) (map[string]interface{}, error)
}
```

Inject this implementation when creating tools that need it:

```go
tool := &tools.SessionsListTool{
    AgentSessionKey: "current-session-key",
    Gateway:         myGatewayImplementation, // implements GatewayClient
}
```

## Notes

- Tools marked as "Structure ready" have complete interfaces but require integration with LiteClaw's internal services
- The `browser` tool requires a browser control server (Chrome DevTools Protocol compatible)
- The `web_search` tool currently supports Brave Search API (Perplexity support can be added)
- The `memory_search` tool uses simple keyword matching; semantic search requires embedding integration


