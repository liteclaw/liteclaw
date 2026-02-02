# LiteClaw Channels Framework

This package provides the **core framework** for channel adapters. The actual channel implementations (Telegram, Discord, etc.) are in the `extensions/` folder.

## Package Contents

| File | Purpose |
|------|---------|
| `types.go` | Common types: ChannelType, ChatType, MessageType, Capabilities, etc. |
| `adapter.go` | `Adapter` interface and `BaseAdapter` base implementation |
| `registry.go` | `Registry` for managing and routing to adapters |
| `factory.go` | `Factory` for configuration-driven adapter creation |
| `channel.go` | Legacy interfaces (deprecated, for backwards compatibility) |

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│                      LiteClaw Gateway                        │
└──────────────────────────┬─────────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────────┐
│                internal/channels (Framework)               │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Registry - manages adapters, routes messages       │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Adapter Interface - unified contract               │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  BaseAdapter - common functionality                 │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Types - IncomingMessage, SendRequest, etc.         │   │
│  └─────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────┘
                           │
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ extensions/  │  │ extensions/  │  │ extensions/  │
│  telegram    │  │  discord     │  │  slack       │
└──────────────┘  └──────────────┘  └──────────────┘
```

## Core Interfaces

### Adapter Interface

```go
type Adapter interface {
    // Metadata
    ID() string
    Name() string
    Type() ChannelType
    Capabilities() *Capabilities

    // Lifecycle
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    IsRunning() bool

    // Connection
    Connect(ctx context.Context) error
    Disconnect(ctx context.Context) error
    IsConnected() bool
    Probe(ctx context.Context) (*ProbeResult, error)

    // Messaging
    Send(ctx context.Context, req *SendRequest) (*SendResult, error)
    SendReaction(ctx context.Context, req *ReactionRequest) error

    // State
    State() *RuntimeState
    SetHandler(handler MessageHandler)
}
```

### Registry

```go
registry := channels.NewRegistry(logger, handler)

// Register adapters
registry.Register(telegramAdapter)
registry.Register(discordAdapter)

// Start all
registry.StartAll(ctx)

// Send message (routes to appropriate adapter)
registry.Send(ctx, &channels.SendRequest{...})

// Stop all
registry.StopAll(ctx)
```

## Usage

Extensions import this package to access the base types:

```go
import "github.com/liteclaw/liteclaw/internal/channels"

type MyAdapter struct {
    *channels.BaseAdapter
    // ...
}

func New(cfg *Config, logger zerolog.Logger) *MyAdapter {
    base := channels.NewBaseAdapter(
        "myplatform",
        "My Platform",
        channels.ChannelTypeCustom,
        caps,
        baseCfg,
        logger,
    )
    return &MyAdapter{BaseAdapter: base}
}
```

## See Also

- `extensions/` - Actual channel implementations
- `extensions/README.md` - Extension documentation
