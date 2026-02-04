# 🦞 LiteClaw — Lightweight Personal AI Assistant

![Status](https://img.shields.io/badge/Status-Work_In_Progress-orange)
![Go Version](https://img.shields.io/github/go-mod/go-version/liteclaw/liteclaw)
![License](https://img.shields.io/badge/License-MIT-blue.svg)

**LiteClaw** is a high-performance personal AI assistant written in Golang. It is designed to be resource-efficient, fast, and easily deployable as a single binary.

---

## ✨ Features

### � High Performance & Efficient
- **Single Binary**: No complex runtime dependencies (like Node.js, Python envs). Just download and run.
- **Low Footprint**: Extremely low memory usage (~10MB idle) and instant startup time (< 1s).
- **Cross-Platform**: Runs on macOS, Linux, and Windows.

### 🤖 Powerful AI Agents
- **Multi-Provider Support**: Native support for top-tier LLMs including:
  - **OpenAI** (GPT-4o, o1)
  - **Anthropic** (Claude 3.5 Sonnet)
  - **Google Gemini**
  - **DeepSeek** (V3, R1)
  - **Alibaba Qwen**
  - **Minimax**
  - **Moonshot (Kimi)** / **Zhipu (GLM)**
  - **Ollama** (Local models) & **OpenRouter**
- **Agent Runtime**: Supports tool calling, streaming responses, and long-running sessions.

### 📱 Omni-Channel Messaging
Seamlessly connect your AI agent to your favorite chat platforms:
- **Global**: Telegram, Discord
- **China-Specific**: QQ (腾讯QQ), Feishu (飞书), DingTalk (钉钉), WeCom (企业微信)
- **Apple**: iMessage (macOS only)
- *Experimental/Planned*: Slack, WhatsApp, Signal, Matrix

### 🛠️ Tools & Capabilities
Empower your agent with built-in capabilities:
- **File System**: Read, write, and edit files.
- **Shell Execution**: Run terminal commands safely.
- **Browser Automation**: Control web browsers for automation tasks (via Chrome/Playwright).
- **Web Search**: Access real-time information via Brave Search (MCP).
- **Process Management**: Manage system processes.
- **Memory**: Persistent note-taking and context retention.

### 🔌 Extensibility
- **Model Context Protocol (MCP)**: Full support for the MCP standard, allowing connection to any MCP-compatible server for unlimited tool extensions.
- **Skill System**: Define new agent capabilities using simple Markdown files (`SKILL.md`).

### ⏰ Automation & Scheduling
- **Cron Jobs**: Built-in scheduler for recurring tasks (e.g., "Summarize news every morning at 8 AM").
- **Natural Language Scheduling**: Support for "every 24h" or cron expressions.

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

Example configs are available in the `configs/` directory.

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

LiteClaw is inspired by [OpenClaw](https://github.com/openclaw/openclaw). Special thanks to the OpenClaw community for the original design concepts.
