// Package prompt provides system prompt building for agents.
// This file contains modular section builders that mirror the TypeScript implementation.
package prompt

import (
	"fmt"
	"strings"
)

// SectionBuilder defines a function that builds a prompt section.
// Returns nil/empty slice if the section should be skipped.
type SectionBuilder func(params *Params) []string

// ----------------------------------------------------------------
// Section: Identity
// ----------------------------------------------------------------

func buildIdentitySection(_ *Params) []string {
	return []string{
		"You are LiteClaw, an advanced AI assistant derived from Clawdbot.",
		"You possess the core capabilities of Clawdbot but are engineered to be lightweight, fast, and resource-efficient.",
		"CRITICAL: You must ALWAYS generate a response to the user's request. Do not stay silent. If you are unsure which tool to use, ask the user for clarification.",
		"",
	}
}

// ----------------------------------------------------------------
// Section: Tooling
// ----------------------------------------------------------------

func buildToolingSection(params *Params) []string {
	lines := []string{
		"## Tooling",
		"Tool availability (filtered by policy):",
		"Tool names are case-sensitive. Call tools exactly as listed.",
	}

	toolLines := buildToolLines(params.ToolNames)
	if len(toolLines) > 0 {
		lines = append(lines, toolLines...)
	} else {
		lines = append(lines, "No tools available.")
	}

	lines = append(lines,
		"TOOLS.md does not control tool availability; it is user guidance for how to use external tools.",
		"If a task is more complex or takes longer, spawn a sub-agent. It will do the work for you and ping you when it's done. You can always check up on it.",
		"",
	)
	return lines
}

// ----------------------------------------------------------------
// Section: Tool Call Style
// ----------------------------------------------------------------

func buildToolCallStyleSection(_ *Params) []string {
	return []string{
		"## Tool Call Style",
		"Default: do not narrate routine, low-risk tool calls (just call the tool).",
		"Narrate only when it helps: multi-step work, complex/challenging problems, sensitive actions (e.g., deletions), or when the user explicitly asks.",
		"Keep narration brief and value-dense; avoid repeating obvious steps.",
		"Use plain human language for narration unless in a technical context.",
		"",
	}
}

// ----------------------------------------------------------------
// Section: CLI Quick Reference
// ----------------------------------------------------------------

func buildCLIReferenceSection(_ *Params) []string {
	return nil
}

// ----------------------------------------------------------------
// Section: Skills (mandatory)
// ----------------------------------------------------------------

func buildSkillsSectionLines(params *Params) []string {
	if params.PromptMode == "minimal" {
		return nil
	}
	if params.SkillsPrompt == "" {
		return nil
	}

	return []string{
		"## Skills (mandatory)",
		"Before replying: scan <available_skills> <description> entries.",
		"- If exactly one skill clearly applies: read its SKILL.md at <location> with `read`, then follow it.",
		"- If multiple could apply: choose the most specific one, then read/follow it.",
		"- If none clearly apply: do not read any SKILL.md.",
		"Constraints: never read more than one skill up front; only read after selecting.",
		params.SkillsPrompt,
		"",
	}
}

// ----------------------------------------------------------------
// Section: Memory Recall
// ----------------------------------------------------------------

func buildMemorySectionLines(params *Params) []string {
	return nil
}

// ----------------------------------------------------------------
// Section: Self-Update
// ----------------------------------------------------------------

func buildSelfUpdateSection(params *Params) []string {
	if params.PromptMode == "minimal" {
		return nil
	}

	// Check if gateway tool is available
	hasGateway := false
	for _, t := range params.ToolNames {
		if t == "gateway" {
			hasGateway = true
			break
		}
	}
	if !hasGateway {
		return nil
	}

	return []string{
		"## Clawdbot Self-Update",
		"Get Updates (self-update) is ONLY allowed when the user explicitly asks for it.",
		"Do not run config.apply or update.run unless the user explicitly requests an update or config change; if it's not explicit, ask first.",
		"Actions: config.get, config.schema, config.apply (validate + write full config, then restart), update.run (update deps or git, then restart).",
		"After restart, Clawdbot pings the last active session automatically.",
		"",
	}
}

// ----------------------------------------------------------------
// Section: Model Aliases
// ----------------------------------------------------------------

func buildModelAliasesSection(params *Params) []string {
	return nil
}

// ----------------------------------------------------------------
// Section: Workspace
// ----------------------------------------------------------------

func buildWorkspaceSection(params *Params) []string {
	lines := []string{
		"## Workspace",
		fmt.Sprintf("Your working directory is: %s", params.WorkspaceDir),
		"Treat this directory as the single global workspace for file operations unless explicitly instructed otherwise.",
	}
	lines = append(lines, params.WorkspaceNotes...)
	lines = append(lines, "")
	return lines
}

// ----------------------------------------------------------------
// Section: Documentation
// ----------------------------------------------------------------

func buildDocumentationSection(params *Params) []string {
	if params.PromptMode == "minimal" {
		return nil
	}
	if params.DocsPath == "" {
		return nil
	}

	return []string{
		"## Documentation",
		fmt.Sprintf("Clawdbot docs: %s", params.DocsPath),
		"Mirror: https://docs.clawd.bot",
		"Source: https://github.com/clawdbot/clawdbot",
		"Community: https://discord.com/invite/clawd",
		"Find new skills: https://clawhub.ai",
		"For Clawdbot behavior, commands, config, or architecture: consult local docs first.",
		"When diagnosing issues, run `clawdbot status` yourself when possible; only ask the user if you lack access (e.g., sandboxed).",
		"",
	}
}

// ----------------------------------------------------------------
// Section: Current Date & Time
// ----------------------------------------------------------------

func buildTimeSectionLines(params *Params) []string {
	lines := []string{
		"## Current Date & Time",
	}
	if params.UserTimezone != "" {
		lines = append(lines, fmt.Sprintf("Time zone: %s", params.UserTimezone))
	}
	lines = append(lines, "")
	return lines
}

// ----------------------------------------------------------------
// Section: Workspace Files (injected) Hint
// ----------------------------------------------------------------

func buildWorkspaceFilesHintSection(_ *Params) []string {
	return []string{
		"## Workspace Files (injected)",
		"These user-editable files are loaded by Clawdbot and included below in Project Context.",
		"",
	}
}

// ----------------------------------------------------------------
// Section: Reply Tags
// ----------------------------------------------------------------

func buildReplyTagsSection(params *Params) []string {
	if params.PromptMode == "minimal" {
		return nil
	}

	return []string{
		"## Reply Tags",
		"To request a native reply/quote on supported surfaces, include one tag in your reply:",
		"- [[reply_to_current]] replies to the triggering message.",
		"- [[reply_to:<id>]] replies to a specific message id when you have it.",
		"Whitespace inside the tag is allowed (e.g. [[ reply_to_current ]] / [[ reply_to: 123 ]]).",
		"Tags are stripped before sending; support depends on the current channel config.",
		"",
	}
}

// ----------------------------------------------------------------
// Section: Messaging
// ----------------------------------------------------------------

func buildMessagingSection(params *Params) []string {
	if params.PromptMode == "minimal" {
		return nil
	}

	// Check if message tool is available
	hasMessage := false
	for _, t := range params.ToolNames {
		if t == "message" {
			hasMessage = true
			break
		}
	}

	lines := []string{
		"## Messaging",
		"- Reply in current session → automatically routes to the source channel (Signal, Telegram, etc.)",
		"- Cross-session messaging → use sessions_send(sessionKey, message)",
		"- Never use exec/curl for provider messaging; Clawdbot handles all routing internally.",
	}

	if hasMessage {
		lines = append(lines, "",
			"### message tool",
			"- Use `message` for proactive sends + channel actions (polls, reactions, etc.).",
			"- For `action=send`, include `to` and `message`.",
			"- If multiple channels are configured, pass `channel` (telegram|whatsapp|discord|googlechat|slack|signal|imessage).",
			"// - If you use `message` (`action=send`) to deliver your user-visible reply, respond with ONLY: NO_REPLY (avoid duplicate replies).",
		)
	}

	lines = append(lines, "")
	return lines
}

// ----------------------------------------------------------------
// Section: Reasoning Format
// ----------------------------------------------------------------

func buildReasoningFormatSection(params *Params) []string {
	if !params.ReasoningTagHint {
		return nil
	}

	return []string{
		"## Reasoning Format",
		"ALL internal reasoning MUST be inside <think>...</think>.",
		"Do not output any analysis outside <think>.",
		"Format every reply as <think>...</think> then <final>...</final>, with no other text.",
		"Only the final user-visible reply may appear inside <final>.",
		"Only text inside <final> is shown to the user; everything else is discarded and never seen by the user.",
		"Example:",
		"<think>Short internal reasoning.</think>",
		"<final>Hey there! What would you like to do next?</final>",
		"",
	}
}

// ----------------------------------------------------------------
// Section: Project Context (SOUL.md, USER.md, etc.)
// ----------------------------------------------------------------

func buildProjectContextSection(params *Params) []string {
	if len(params.ContextFiles) == 0 {
		return nil
	}

	hasSoul := false
	for _, f := range params.ContextFiles {
		if strings.EqualFold(f.Name, "SOUL.md") && !f.Missing {
			hasSoul = true
			break
		}
	}

	lines := []string{
		"# Project Context",
		"",
		"The following project context files have been loaded:",
	}
	if hasSoul {
		lines = append(lines, "If SOUL.md is present, embody its persona and tone. Follow its guidance unless higher-priority instructions override it.")
		lines = append(lines, "IMPORTANT: You must ALWAYS generate a response or call a tool when the User makes a request. Do not stay silent.")
	}
	lines = append(lines, "")

	for _, f := range params.ContextFiles {
		if f.Missing || f.Content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("## %s", f.Name), "", f.Content, "")
	}

	return lines
}

// ----------------------------------------------------------------
// Section: Silent Replies
// ----------------------------------------------------------------

// ----------------------------------------------------------------
// Section: Silent Replies
// ----------------------------------------------------------------

func buildSilentRepliesSection(params *Params) []string {
	// Disable Silent Replies instruction for Minimax stability
	// return nil
	return nil
}

// ----------------------------------------------------------------
// Section: Heartbeats
// ----------------------------------------------------------------

func buildHeartbeatsSection(params *Params) []string {
	if params.PromptMode == "minimal" {
		return nil
	}

	// Default heartbeat prompt - matches TypeScript HEARTBEAT_PROMPT constant
	heartbeatPrompt := params.HeartbeatPrompt
	if heartbeatPrompt == "" {
		heartbeatPrompt = "Read HEARTBEAT.md if it exists (workspace context). Follow it strictly. Do not infer or repeat old tasks from prior chats. If nothing needs attention, reply HEARTBEAT_OK."
	}

	return []string{
		"## Heartbeats",
		fmt.Sprintf("Heartbeat prompt: %s", heartbeatPrompt),
		"If you receive a heartbeat poll (a user message matching the heartbeat prompt above), and there is nothing that needs attention, reply exactly:",
		"HEARTBEAT_OK",
		"Clawdbot treats a leading/trailing \"HEARTBEAT_OK\" as a heartbeat ack (and may discard it).",
		"If something needs attention, do NOT include \"HEARTBEAT_OK\"; reply with the alert text instead.",
		"",
	}
}

// ----------------------------------------------------------------
// Section: Runtime
// ----------------------------------------------------------------

func buildRuntimeSection(params *Params) []string {
	lines := []string{
		"## Runtime",
		buildRuntimeLine(params.RuntimeInfo),
	}

	if params.ReasoningLevel != "" {
		lines = append(lines, fmt.Sprintf("Reasoning: %s (hidden unless on/stream). Toggle /reasoning; /status shows Reasoning when enabled.", params.ReasoningLevel))
	}

	return lines
}
