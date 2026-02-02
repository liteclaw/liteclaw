# LiteClaw Skills

Skills are markdown files with YAML frontmatter that extend agent capabilities by providing specialized knowledge and workflows. These skills are **directly copied from Clawdbot** and are fully compatible with LiteClaw's skills system.

## Skills Count: 52

| Category | Skills |
|----------|--------|
| **Productivity** | 1password, apple-notes, apple-reminders, bear-notes, notion, obsidian, things-mac, trello |
| **Communication** | bluebubbles, discord, imsg, slack, wacli, voice-call |
| **Development** | coding-agent, github, skill-creator |
| **Media** | camsnap, gifgrep, nano-banana-pro, nano-pdf, openai-image-gen, openai-whisper, openai-whisper-api, peekaboo, sherpa-onnx-tts, songsee, video-frames |
| **Smart Home** | blucli, eightctl, openhue, sonoscli |
| **Utilities** | blogwatcher, canvas, clawdhub, gemini, gog, goplaces, himalaya, local-places, mcporter, model-usage, oracle, ordercli, sag, session-logs, summarize, tmux, weather |
| **Social** | bird (Twitter/X), spotify-player |
| **Food** | food-order |

## Skill Format

Each skill has a `SKILL.md` file with:

1. **YAML Frontmatter** - Metadata including name, description, and requirements
2. **Markdown Body** - Instructions, workflows, and examples

### Example SKILL.md

```markdown
---
name: weather
description: Get current weather and forecasts (no API key required).
homepage: https://wttr.in/:help
metadata: {"clawdbot":{"emoji":"🌤️","requires":{"bins":["curl"]}}}
---

# Weather

Two free services, no API keys needed.

## wttr.in (primary)

Quick one-liner:
\```bash
curl -s "wttr.in/London?format=3"
\```
```

## Skill Metadata

The `metadata` field contains a JSON object with LiteClaw-specific configuration:

```json
{
  "clawdbot": {
    "emoji": "🔧",
    "requires": {
      "bins": ["required-binary"],
      "env": ["REQUIRED_ENV_VAR"],
      "config": ["channels.telegram"]
    },
    "install": [
      {
        "id": "brew",
        "kind": "brew",
        "formula": "package-name",
        "bins": ["binary-name"],
        "label": "Install via Homebrew"
      }
    ],
    "os": ["darwin", "linux"]
  }
}
```

## Loading Skills

Skills are loaded from:
- `~/.liteclaw/skills/` (user skills)
- `./skills/` (bundled with LiteClaw)
- Custom directories via config

```yaml
skills:
  enabled: true
  dirs:
    - ~/.liteclaw/skills
    - /custom/skills/path
```

## Skill Eligibility

Skills are only shown to the agent if all requirements are met:
1. ✅ All required binaries are available in PATH
2. ✅ All required environment variables are set
3. ✅ OS requirements are satisfied
4. ✅ Required config sections exist

This ensures the agent only sees relevant skills.

## Creating Custom Skills

1. Create a new directory under `skills/` (e.g., `skills/my-skill/`)
2. Create a `SKILL.md` file with YAML frontmatter
3. Add instructions and examples in markdown
4. Optionally specify requirements in metadata

## Full Skills List

| Skill | Description |
|-------|-------------|
| 1password | 1Password CLI (op) for secrets management |
| apple-notes | Manage Apple Notes via memo CLI |
| apple-reminders | Manage Apple Reminders via remindctl CLI |
| bear-notes | Create and manage Bear notes via grizzly CLI |
| bird | X/Twitter CLI for reading and posting |
| blogwatcher | Monitor blogs and RSS feeds |
| blucli | BluOS CLI for speaker control |
| bluebubbles | BlueBubbles external channel plugin |
| camsnap | Capture from RTSP/ONVIF cameras |
| canvas | Display HTML content on nodes |
| clawdhub | Search and install from ClawdHub |
| coding-agent | Run coding agents (Codex, Claude Code) |
| discord | Discord integration tool |
| eightctl | Control Eight Sleep pods |
| food-order | Foodora order reordering |
| gemini | Gemini CLI for Q&A and generation |
| gifgrep | Search GIF providers |
| github | GitHub CLI (gh) for issues, PRs, runs |
| gog | Google Workspace CLI |
| goplaces | Google Places API queries |
| himalaya | Email CLI via IMAP/SMTP |
| imsg | iMessage/SMS CLI |
| local-places | Local places search |
| mcporter | MCP server management |
| model-usage | LLM usage cost tracking |
| nano-banana-pro | Gemini 3 image generation |
| nano-pdf | PDF editing with natural language |
| notion | Notion API for pages and databases |
| obsidian | Obsidian vault management |
| openai-image-gen | OpenAI image generation |
| openai-whisper | Local whisper speech-to-text |
| openai-whisper-api | OpenAI Whisper API |
| openhue | Philips Hue control |
| oracle | Oracle CLI for prompts |
| ordercli | Foodora order CLI |
| peekaboo | macOS UI automation |
| sag | ElevenLabs text-to-speech |
| session-logs | Search session logs |
| sherpa-onnx-tts | Local TTS via sherpa-onnx |
| skill-creator | Create and update skills |
| slack | Slack channel control |
| songsee | Audio spectrogram generation |
| sonoscli | Sonos speaker control |
| spotify-player | Spotify playback control |
| summarize | URL and podcast summarization |
| things-mac | Things 3 task management |
| tmux | tmux session management |
| trello | Trello board management |
| video-frames | Extract video frames |
| voice-call | Voice call integration |
| wacli | WhatsApp messaging |
| weather | Weather forecasts (no API key) |
