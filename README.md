# 🦞 LiteClaw — Lightweight Personal AI Assistant

![Status](https://img.shields.io/badge/Status-Work_In_Progress-orange)
![Go Version](https://img.shields.io/github/go-mod/go-version/liteclaw/liteclaw)
![License](https://img.shields.io/badge/License-MIT-blue.svg)

> **⚠️ Note:** This project is currently in **active development**. Features are being ported and optimized. It is not yet recommended for production use.  
> **UI:** This version does **not** include a working UI; UI will be addressed in a later release.

**LiteClaw** is a high-performance, single-binary rewrite of [OpenClaw](https://github.com/openclaw/openclaw) (TypeScript) in Golang. It aims to provide the same powerful personal AI assistant capabilities with a fraction of the resource footprint.

---

## 📊 Performance Comparison

| Metric | OpenClaw (TypeScript) | LiteClaw (Go) |
|--------|-----------------------|---------------|
| Binary Size | ~500MB (w/ node_modules) | **~25MB** |
| Idle Memory | ~300MB | **~10MB** |
| Startup Time | 5-10s | **< 1s** |
| Build Chain | npm/yarn/pnpm complexity | `go build` |
| Runtime | Node.js 22+ required | **Zero runtime deps** |

---

## 🔄 Feature Parity Status: OpenClaw vs LiteClaw

Below is a comprehensive comparison of features between the original TypeScript **OpenClaw** and the Go rewrite **LiteClaw**.

| Legend | Meaning |
|--------|---------|
| ✅ | Fully implemented |
| 🔶 | Partially implemented / Basic support |
| ❌ | Not yet implemented |
| ➖ | Not applicable / Not planned |

### Core Platform

| Feature | OpenClaw (TS) | LiteClaw (Go) | Notes |
|---------|:-------------:|:-------------:|-------|
| Gateway (WS Control Plane) | ✅ | ✅ | HTTP + WebSocket API |
| CLI Interface | ✅ | ✅ | `gateway`, `agent`, `cron`, `status`, etc. |
| Agent Runtime (LLM Loop) | ✅ | ✅ | Tool calling, streaming |
| Session Management | ✅ | ✅ | Per-user/group sessions |
| Media Pipeline | ✅ | 🔶 | Basic image/audio support |
| Onboarding Wizard | ✅ | ✅ | `liteclaw onboard` |
| Config Hot Reload | ✅ | 🔶 | Manual restart required |

### Messaging Channels

| Channel | OpenClaw (TS) | LiteClaw (Go) | Notes |
|---------|:-------------:|:-------------:|-------|
| **QQ（腾讯QQ）** | ❌ | ✅ | China-specific |
| **Feishu （Lark 飞书）** | ❌ | ✅ | China-specific |
| **DingTalk（钉钉）** | ❌ | ✅ | China-specific |
| **WeCom（企业微信）** | ❌ | ✅ | China-specific |
| **Telegram** | ✅ | ✅ | grammY / go-telegram-bot-api |
| **Discord** | ✅ | ✅ | discord.js / discordgo |
| **Slack** | ✅ | 🔶 | Adapter exists, needs testing |
| **WhatsApp** | ✅ (Baileys) | 🔶 | Adapter exists, needs Baileys bridge |
| **Signal** | ✅ (signal-cli) | 🔶 | Adapter exists, needs signal-cli |
| **iMessage** | ✅ (macOS) | 🔶 | Adapter exists, macOS only |
| **Google Chat** | ✅ | 🔶 | Adapter exists |
| **Microsoft Teams** | ✅ (extension) | 🔶 | Adapter exists |
| **Matrix** | ✅ (extension) | 🔶 | Adapter exists |
| **Line** | ❌ | 🔶 | Adapter exists |
| **WebChat** | ✅ | ❌ | Planned for later |
| **BlueBubbles** | ✅ (extension) | ❌ | Not planned |
| **Zalo** | ✅ (extension) | ❌ | Not planned |

### Apps & Companion Nodes

| Feature | OpenClaw (TS) | LiteClaw (Go) | Notes |
|---------|:-------------:|:-------------:|-------|
| macOS Menu Bar App | ✅ | ❌ | Swift app, not in scope |
| iOS Node | ✅ | ❌ | Swift app, not in scope |
| Android Node | ✅ | ❌ | Kotlin app, not in scope |
| Voice Wake (Always-on Speech) | ✅ | ❌ | Requires native app |
| Talk Mode (Conversation Overlay) | ✅ | ❌ | Requires native app |
| Canvas (A2UI Visual Workspace) | ✅ | ❌ | Requires native app |
| TUI (Terminal UI) | ✅ | ✅ | `liteclaw tui` |

### Tools & Automation

| Tool | OpenClaw (TS) | LiteClaw (Go) | Notes |
|------|:-------------:|:-------------:|-------|
| **Shell Execution (bash)** | ✅ | ✅ | `exec` tool |
| **File Read/Write/Edit** | ✅ | ✅ | `read`, `write`, `edit` tools |
| **Process Management** | ✅ | ✅ | `process` tool |
| **Browser Automation (CDP)** | ✅ | ✅ | Playwright/Chrome relay |
| **Web Search (Brave)** | ✅ | ✅ | Via MCP |
| **Content Fetch** | ✅ | ✅ | `fetch` tool |
| **Memory (Persistent Notes)** | ✅ | ✅ | `memory` tool |
| **Sessions (Agent-to-Agent)** | ✅ | 🔶 | `sessions_*` tools |
| **Camera Snap/Clip** | ✅ | ❌ | Requires node |
| **Screen Recording** | ✅ | ❌ | Requires node |
| **Location.get** | ✅ | ❌ | Requires node |
| **System Notifications** | ✅ | ❌ | Requires node |
| **Discord/Slack Actions** | ✅ | 🔶 | Basic support |

### Scheduling & Automation

| Feature | OpenClaw (TS) | LiteClaw (Go) | Notes |
|---------|:-------------:|:-------------:|-------|
| Cron Jobs (Scheduler) | ✅ | ✅ | `cron add/list/rm/run` |
| One-time At Tasks | ✅ | ✅ | `--at` flag |
| Recurring Every Tasks | ✅ | ✅ | `--every` flag |
| Webhooks (HTTP Triggers) | ✅ | 🔶 | Basic support |
| Gmail Pub/Sub | ✅ | ❌ | Not implemented |

### Skills & Extensibility

| Feature | OpenClaw (TS) | LiteClaw (Go) | Notes |
|---------|:-------------:|:-------------:|-------|
| Skill System (`SKILL.md`) | ✅ | ✅ | Load from workspace |
| Bundled Skills | ✅ | ✅ | `skills/` directory |
| ClawdHub (Skill Registry) | ✅ | 🔶 | Basic hub support |
| MCP (Model Context Protocol) | ✅ | ✅ | `liteclaw.extras.json` |
| Plugin SDK | ✅ | ❌ | Not implemented |

### Models & LLM Support

| Provider | OpenClaw (TS) | LiteClaw (Go) | Notes |
|----------|:-------------:|:-------------:|-------|
| **Anthropic (Claude)** | ✅ | ✅ | Messages API |
| **OpenAI (GPT-4/o)** | ✅ | ✅ | Completions API |
| **DeepSeek** | ✅ | ✅ | OpenAI-compatible |
| **Qwen (Alibaba)** | ✅ | ✅ | OpenAI-compatible |
| **Minimax** | ✅ | ✅ | OpenAI-compatible |
| **Google Gemini** | ✅ | ✅ | OpenAI-compatible |
| **Moonshot (Kimi)** | ✅ | ✅ | OpenAI-compatible |
| **Zhipu (GLM)** | ✅ | ✅ | OpenAI-compatible |
| **Ollama (Local)** | ✅ | ✅ | OpenAI-compatible |
| **OpenRouter** | ✅ | ✅ | OpenAI-compatible |
| Model Failover | ✅ | 🔶 | Basic support |
| OAuth Auth (Claude/ChatGPT Pro) | ✅ | ❌ | API key only |

### Runtime & Safety

| Feature | OpenClaw (TS) | LiteClaw (Go) | Notes |
|---------|:-------------:|:-------------:|-------|
| DM Pairing (Security) | ✅ | ✅ | `pairing approve/deny` |
| Group Allowlists | ✅ | ✅ | Config-based |
| Streaming Responses | ✅ | ✅ | Real-time output |
| Typing Indicators | ✅ | 🔶 | Channel-dependent |
| Usage Tracking | ✅ | 🔶 | Basic logging |
| Session Compaction | ✅ | 🔶 | Basic support |
| Docker Sandboxing | ✅ | ❌ | Not implemented |

### Operations & Deployment

| Feature | OpenClaw (TS) | LiteClaw (Go) | Notes |
|---------|:-------------:|:-------------:|-------|
| Control UI (Web Dashboard) | ✅ | ❌ | Planned for later |
| WebChat UI | ✅ | ❌ | Planned for later |
| Tailscale Serve/Funnel | ✅ | ❌ | Not implemented |
| SSH Tunnel Support | ✅ | ❌ | Not implemented |
| Daemon (launchd/systemd) | ✅ | 🔶 | `--detached` mode |
| Doctor (Diagnostics) | ✅ | ❌ | Not implemented |
| Nix Packaging | ✅ | ❌ | Not implemented |
| Docker Support | ✅ | ❌ | Not implemented |

---

## ✨ Key Features (LiteClaw)

- **🚀 Deployment Simplified**: Single binary, zero runtime dependencies (no Node.js/npm required).
- **💾 Efficiency First**: Extremely low memory footprint (~30MB idle vs 300MB+ in Node).
- **🔌 MCP Support**: Native support for Model Context Protocol (MCP), enabling connection to external tools like Brave Search, Playwright, and more.
- **🤖 Universal Agent**: Supports multiple LLM backends (OpenAI, Anthropic, Minimax, Ollama, Gemini, DeepSeek, Qwen, etc.).
- **📱 Omni-Channel**: Seamless integration with **Telegram**, **Discord**, QQ, Feishu, DingTalk, WeCom, and more.
- **⚡ Skill System**: Modular skill architecture (`SKILL.md`) for defining agent capabilities and instructions.
- **🛠️ Built-in Tools**: Browser automation, shell execution, file management, web search.
- **⏰ Scheduler**: Built-in cron job manager for reminders and recurring tasks.
- **🇨🇳 China-Friendly**: Native support for Chinese platforms (QQ, Feishu, DingTalk, WeCom).

---

## 🏗️ Architecture

```
LiteClaw/
├── cmd/liteclaw/          # Main entry point
├── configs/               # Example configs (example.liteclaw*.json)
├── extensions/            # Channel adapters (Telegram, Discord, QQ, etc.)
├── internal/              # Core logic
│   ├── agent/             # AI Agent (Tools, Prompts, Skills)
│   ├── gateway/           # HTTP/WS server
│   ├── browser/           # CDP/Browser automation
│   ├── cron/              # Scheduler
│   ├── pairing/           # DM security
│   └── cli/               # CLI commands
├── mcp/                   # Model Context Protocol client
└── skills/                # Built-in skills library
```

---

## 🚀 Getting Started

### Prerequisites

- Go 1.24+
- `make` (optional, for easy building)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/liteclaw/liteclaw.git
cd liteclaw

# Build the binary
make build
# or: go build -o liteclaw ./cmd/liteclaw

# Verify installation
./liteclaw version
```

### First-time Setup

```bash
# Run the onboarding wizard
./liteclaw onboard
```

The wizard will guide you through:
1. Model selection (provider + model)
2. API key configuration
3. Workspace directory
4. Channel setup (optional)

### Configuration

LiteClaw reads config from:
- `~/.liteclaw/liteclaw.json` — Main configuration
- `~/.liteclaw/liteclaw.extras.json` — MCP servers and extensions

Example configs are available at:
- `configs/example.liteclaw.json`
- `configs/example.liteclaw.extras.json`

A minimal `liteclaw.json` sample:

```json
{
  "agents": {
    "defaults": {
      "model": { "primary": "anthropic/claude-sonnet-4-20250514" },
      "workspace": "~/clawd"
    }
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "botToken": "YOUR_BOT_TOKEN"
    }
  },
  "gateway": {
    "port": 18789
  }
}
```

### Running the Gateway

```bash
# Foreground mode
./liteclaw gateway start

# Background (detached) mode
./liteclaw gateway start --detached

# Check status
./liteclaw status

# Stop gateway
./liteclaw gateway stop
```

### Sending Messages

```bash
# Send a message to the agent
./liteclaw agent --message "Hello, summarize the news today"

# With thinking level
./liteclaw agent --message "Analyze this code" --thinking high
```

### Managing Cron Jobs

```bash
# List jobs
./liteclaw cron list

# Add a recurring job
./liteclaw cron add --name "Morning News" --every 24h --message "Summarize today's news"

# Add a cron-expression job
./liteclaw cron add --name "Daily Report" --cron "0 9 * * *" --message "Generate daily report"

# Run a job manually
./liteclaw cron run <job-id>
```

---

## 🚧 Roadmap (v0.2+)

- [ ] Control UI (React/Vue Web Dashboard)
- [ ] WebChat interface
- [ ] Docker support & Dockerfile
- [ ] Tailscale Serve/Funnel integration
- [ ] Doctor diagnostics command
- [ ] Full WhatsApp/Signal support (Baileys/signal-cli bridges)
- [ ] OAuth authentication for Claude/ChatGPT Pro subscriptions

---

## 🤝 Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Run `go test ./...` before submitting
4. Submit a Pull Request

---

## 📄 License

MIT License. See [LICENSE](LICENSE) for details.

---

## 🙏 Acknowledgments

LiteClaw is a Go rewrite inspired by [OpenClaw](https://github.com/openclaw/openclaw) (TypeScript). Special thanks to the OpenClaw community for the original design and architecture.
