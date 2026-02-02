# LiteClaw Extensions

Extensions are modular components that add communication channel support to LiteClaw. Each extension provides integration with a specific messaging platform.

## Architecture

```
extensions/
├── telegram/           # Telegram Bot API integration
│   ├── adapter.go      # Adapter implementation
│   └── client.go       # API client
├── discord/            # Discord integration
│   ├── adapter.go      # Adapter implementation
│   └── client.go       # API client (includes WebSocket)
├── slack/              # Slack integration
│   ├── adapter.go      # Adapter implementation
│   └── client.go       # API client
├── whatsapp/           # WhatsApp integration
├── matrix/             # Matrix protocol
├── signal/             # Signal messenger
├── msteams/            # Microsoft Teams
├── googlechat/         # Google Chat
├── line/               # LINE messenger
└── imessage/           # iMessage (macOS only)
```

## Extension Structure

Each extension follows the same pattern:

```go
// extensions/myplatform/adapter.go
package myplatform

import (
    "github.com/liteclaw/liteclaw/internal/channels"
)

type Adapter struct {
    *channels.BaseAdapter
    // Platform-specific fields
}

func New(cfg *Config, logger zerolog.Logger) *Adapter {
    // Create and return adapter
}

// Implement channels.Adapter interface
func (a *Adapter) Start(ctx context.Context) error { ... }
func (a *Adapter) Stop(ctx context.Context) error { ... }
func (a *Adapter) Send(ctx context.Context, req *channels.SendRequest) (*channels.SendResult, error) { ... }
// ...
```

## Current Status

| Extension | Status | Features |
|-----------|--------|----------|
| **telegram** | ✅ Full | Long polling, Webhooks, Reactions, Threads |
| **discord** | ✅ Full | WebSocket, Reactions, Threads, Voice |
| **slack** | ✅ Full | Socket Mode, Reactions, Threads |
| **matrix** | ✅ Full | Sync API, Federated, Reactions |
| **whatsapp** | 🔨 Skeleton | Needs whatsmeow integration |
| **signal** | 🔨 Skeleton | Needs signal-cli integration |
| **msteams** | 🔨 Skeleton | Bot Framework placeholder |
| **googlechat** | 🔨 Skeleton | Webhook API placeholder |
| **line** | 🔨 Skeleton | Messaging API placeholder |
| **imessage** | 🔨 Skeleton | AppleScript placeholder |

## Core Framework

The core framework lives in `internal/channels/`:

- `types.go` - Common types (ChannelType, ChatType, MessageType, etc.)
- `adapter.go` - Adapter interface and BaseAdapter
- `registry.go` - Registry for managing all adapters
- `factory.go` - Configuration-driven adapter creation

## Creating a New Extension

1. Create directory: `extensions/myplatform/`
2. Create `adapter.go` implementing `channels.Adapter`
3. Create `client.go` for API/SDK integration
4. Register in the main application

## Relationship with Clawdbot

This structure mirrors Clawdbot's `extensions/` folder where each communication channel is a self-contained extension module.
