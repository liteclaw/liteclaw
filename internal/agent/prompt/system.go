package prompt

import (
	"fmt"
	"strings"
	"time"

	"github.com/liteclaw/liteclaw/internal/agent/workspace"
)

// Params defines input parameters for building the system prompt.
// This structure mirrors the TypeScript version's params object.
type Params struct {
	// Core
	WorkspaceDir string
	PromptMode   string // "full", "minimal", "none"

	// Tools
	ToolNames []string

	// Context Files (SOUL.md, USER.md, etc.)
	ContextFiles []workspace.BootstrapFile

	// Skills
	SkillsPrompt string

	// Model Configuration
	ModelAliases   []string // e.g. ["Minimax: minimax/MiniMax-M2.1"]
	ReasoningLevel string   // "off", "on", "stream"

	// Reasoning Format
	ReasoningTagHint bool

	// Tokens
	SilentReplyToken string // Default: "NO_REPLY"
	HeartbeatPrompt  string

	// Documentation
	DocsPath string

	// Workspace Notes
	WorkspaceNotes []string

	// Time
	UserTimezone string

	// Runtime Info
	RuntimeInfo RuntimeInfo

	// Extra
	ExtraSystemPrompt string
}

// RuntimeInfo holds runtime environment details.
type RuntimeInfo struct {
	OS           string
	Arch         string
	Model        string
	DefaultModel string
	Channel      string
	Capabilities []string
	AgentID      string
	Host         string
	Node         string
	GoVersion    string
	RepoRoot     string
	Thinking     string
}

// BuildAgentSystemPrompt generates the system prompt string.
// This function orchestrates all section builders to create the final prompt.
func BuildAgentSystemPrompt(params Params) string {
	// Handle "none" mode - minimal identity only
	if params.PromptMode == "none" {
		return "You are LiteClaw, a lightweight assistant running inside LiteClaw."
	}

	// Default to "full" mode
	if params.PromptMode == "" {
		params.PromptMode = "full"
	}

	// Build all sections in order (mirroring TypeScript structure)
	var allLines []string

	// Section builders in order - matches TypeScript lines array structure
	sectionBuilders := []SectionBuilder{
		buildIdentitySection,
		buildToolingSection,
		buildToolCallStyleSection,
		buildCLIReferenceSection,
		buildSkillsSectionLines,
		buildMemorySectionLines,
		buildSelfUpdateSection,
		buildModelAliasesSection,
		buildWorkspaceSection,
		buildDocumentationSection,
		buildTimeSectionLines,
		buildWorkspaceFilesHintSection,
		buildReplyTagsSection,
		buildMessagingSection,
		buildReasoningFormatSection,
		buildProjectContextSection,
		buildSilentRepliesSection,
		buildHeartbeatsSection,
		buildRuntimeSection,
	}

	for _, builder := range sectionBuilders {
		section := builder(&params)
		if len(section) > 0 {
			allLines = append(allLines, section...)
		}
	}

	// Filter empty lines at the end but keep structure
	return strings.Join(allLines, "\n")
}

// buildToolLines generates the tool list with descriptions.
func buildToolLines(tools []string) []string {
	var lines []string
	summaries := map[string]string{
		// File tools
		"read":        "Read file contents",
		"write":       "Create or overwrite files",
		"edit":        "Make precise edits to files",
		"apply_patch": "Apply multi-file patches",
		"grep":        "Search file contents for patterns",
		"find":        "Find files by glob pattern",
		"ls":          "List directory contents",
		"list":        "List directory contents",

		// Execution tools
		"exec":    "Run shell commands (pty available for TTY-required CLIs)",
		"process": "Manage background exec sessions",

		// Web tools
		"web_search": "Search the web (Brave API)",
		"web_fetch":  "Fetch and extract readable content from a URL",

		// Browser and UI tools
		"browser": "Control web browser",
		"canvas":  "Present/eval/snapshot the Canvas",
		"nodes":   "List/describe/notify/camera/screen on paired nodes",

		// Scheduling
		"cron": "Manage cron jobs and wake events (use for reminders; when scheduling a reminder, write the systemEvent text as something that will read like a reminder when it fires, and mention that it is a reminder depending on the time gap between setting and firing; include recent context in reminder text if appropriate)",

		// Messaging
		"message": "Send messages and channel actions",

		// System
		"gateway": "Restart, apply config, or run updates on the running Clawdbot process",

		// Agent/Session tools
		"agents_list":      "List agent ids allowed for sessions_spawn",
		"sessions_list":    "List other sessions (incl. sub-agents) with filters/last",
		"sessions_history": "Fetch history for another session/sub-agent",
		"sessions_send":    "Send a message to another session/sub-agent",
		"sessions_spawn":   "Spawn a sub-agent session",
		"session_status":   "Show a /status-equivalent status card (usage + time + Reasoning/Verbose/Elevated); use for model-use questions (📊 session_status); optional per-session model override",

		// Media
		"image": "Analyze an image with the configured image model",
		"tts":   "Convert text to speech and return a MEDIA: path. Use when the user requests audio or TTS is enabled. Copy the MEDIA line exactly.",

		// Memory
		"memory_search": "Mandatory recall step: semantically search MEMORY.md + memory/*.md (and optional session transcripts) before answering questions about prior work, decisions, dates, people, preferences, or todos; returns top snippets with path + lines.",
		"memory_get":    "Safe snippet read from MEMORY.md or memory/*.md with optional from/lines; use after memory_search to pull only the needed lines and keep context small.",
	}

	for _, t := range tools {
		if summary, ok := summaries[t]; ok {
			lines = append(lines, fmt.Sprintf("- %s: %s", t, summary))
		} else {
			lines = append(lines, fmt.Sprintf("- %s", t))
		}
	}
	return lines
}

// buildRuntimeLine creates the runtime info line.
func buildRuntimeLine(info RuntimeInfo) string {
	parts := []string{}
	if info.AgentID != "" {
		parts = append(parts, fmt.Sprintf("agent=%s", info.AgentID))
	}
	if info.Host != "" {
		parts = append(parts, fmt.Sprintf("host=%s", info.Host))
	}
	if info.RepoRoot != "" {
		parts = append(parts, fmt.Sprintf("repo=%s", info.RepoRoot))
	}
	if info.OS != "" {
		osStr := fmt.Sprintf("os=%s", info.OS)
		if info.Arch != "" {
			osStr += fmt.Sprintf(" (%s)", info.Arch)
		}
		parts = append(parts, osStr)
	}
	if info.Node != "" {
		parts = append(parts, fmt.Sprintf("node=%s", info.Node))
	}
	if info.GoVersion != "" {
		parts = append(parts, fmt.Sprintf("go=%s", info.GoVersion))
	}
	if info.Model != "" {
		parts = append(parts, fmt.Sprintf("model=%s", info.Model))
	}
	if info.DefaultModel != "" {
		parts = append(parts, fmt.Sprintf("default_model=%s", info.DefaultModel))
	}
	if info.Channel != "" {
		parts = append(parts, fmt.Sprintf("channel=%s", info.Channel))
		if len(info.Capabilities) > 0 {
			parts = append(parts, fmt.Sprintf("capabilities=%s", strings.Join(info.Capabilities, ",")))
		} else {
			parts = append(parts, "capabilities=none")
		}
	}
	if info.Thinking != "" {
		parts = append(parts, fmt.Sprintf("thinking=%s", info.Thinking))
	}

	return "Runtime: " + strings.Join(parts, " | ")
}

// Builder helps construct the system prompt by gathering necessary context.
type Builder struct {
	params Params
}

// NewBuilder creates a new prompt builder with defaults.
func NewBuilder(workspaceDir string) *Builder {
	return &Builder{
		params: Params{
			WorkspaceDir:     workspaceDir,
			PromptMode:       "full",
			SilentReplyToken: "NO_REPLY",
		},
	}
}

// WithRuntimeInfo sets runtime information.
func (b *Builder) WithRuntimeInfo(info RuntimeInfo) *Builder {
	b.params.RuntimeInfo = info
	// Default timezone to local if not set
	if b.params.UserTimezone == "" {
		b.params.UserTimezone = time.Now().Location().String()
	}
	return b
}

// WithTools sets the available tool names.
func (b *Builder) WithTools(names []string) *Builder {
	b.params.ToolNames = names
	return b
}

// WithConfig sets configuration-driven parameters.
func (b *Builder) WithConfig(reasoningLevel string) *Builder {
	b.params.ReasoningLevel = reasoningLevel
	return b
}

// WithDocsPath sets the documentation path.
func (b *Builder) WithDocsPath(path string) *Builder {
	b.params.DocsPath = path
	return b
}

// WithWorkspaceNotes sets workspace notes.
func (b *Builder) WithWorkspaceNotes(notes []string) *Builder {
	b.params.WorkspaceNotes = notes
	return b
}

// WithModelAliases sets the model aliases.
func (b *Builder) WithModelAliases(aliases []string) *Builder {
	b.params.ModelAliases = aliases
	return b
}

// WithSkillsPrompt sets the skills prompt.
func (b *Builder) WithSkillsPrompt(prompt string) *Builder {
	b.params.SkillsPrompt = prompt
	return b
}

// WithHeartbeatPrompt sets the heartbeat prompt.
func (b *Builder) WithHeartbeatPrompt(prompt string) *Builder {
	b.params.HeartbeatPrompt = prompt
	return b
}

// WithPromptMode sets the prompt mode (full, minimal, none).
func (b *Builder) WithPromptMode(mode string) *Builder {
	b.params.PromptMode = mode
	return b
}

// WithReasoningTagHint enables the reasoning tag format hint.
func (b *Builder) WithReasoningTagHint(enabled bool) *Builder {
	b.params.ReasoningTagHint = enabled
	return b
}

// LoadWorkspaceContext loads all bootstrap files from the workspace directory.
func (b *Builder) LoadWorkspaceContext() error {
	dir := b.params.WorkspaceDir
	if dir == "" {
		dir = workspace.ResolveDefaultDir()
		b.params.WorkspaceDir = dir
	}

	files, err := workspace.LoadBootstrapFiles(dir)
	if err != nil {
		return fmt.Errorf("failed to load workspace files: %w", err)
	}

	b.params.ContextFiles = files
	return nil
}

// Build constructs the final system prompt string.
func (b *Builder) Build() (string, error) {
	// Lazy load workspace context if not already done
	if len(b.params.ContextFiles) == 0 && b.params.WorkspaceDir != "" {
		_ = b.LoadWorkspaceContext()
	}

	return BuildAgentSystemPrompt(b.params), nil
}
