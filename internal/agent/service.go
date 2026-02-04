package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/liteclaw/liteclaw/internal/agent/llm"
	"github.com/liteclaw/liteclaw/internal/agent/prompt"
	"github.com/liteclaw/liteclaw/internal/agent/skills"
	"github.com/liteclaw/liteclaw/internal/agent/tools"
	"github.com/liteclaw/liteclaw/internal/agent/workspace"
	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/liteclaw/liteclaw/internal/cron"
	mcp "github.com/liteclaw/liteclaw/mcp"
	"github.com/rs/zerolog"
)

type Service struct {
	Config    *config.Config
	Agent     *Agent
	Scheduler *cron.Scheduler
	Verbose   bool
}

func NewService(cfg *config.Config, sender tools.MessageSender) *Service {
	// ... (Existing env setup) ...
	// 0. Hydrate Environment from Config
	// checks cfg.Env and sets os.Setenv so all tools/libs can access them.
	for k, v := range cfg.Env {
		if v != "" {
			_ = os.Setenv(k, v)
			// Also set uppercase version for convention (e.g. minimax_api_key -> MINIMAX_API_KEY)
			upperK := strings.ToUpper(k)
			if upperK != k {
				_ = os.Setenv(upperK, v)
			}
		}
	}

	// 2. Determine Provider
	var provider llm.Provider
	var model string

	// Parse Primary Model from Config (e.g. "minimax/MiniMax-M2.1")
	primaryStr := cfg.Agents.Defaults.Model.Primary
	if primaryStr == "" {
		if !cfg.Wizard.Mock { // If not in a mock/test state
			fmt.Printf("Config Error: agents.defaults.model.primary is missing in liteclaw.json\n")
			os.Exit(1)
		}
	}

	parts := strings.SplitN(primaryStr, "/", 2)
	if len(parts) != 2 {
		fmt.Printf("Config Error: Invalid model format '%s'. Expected 'provider/model' (e.g. 'minimax/MiniMax-M2.1')\n", primaryStr)
		os.Exit(1)
	}

	providerName := parts[0]
	model = parts[1]

	// 1. Check Provider Config
	p, ok := cfg.Models.Providers[providerName]
	if !ok {
		fmt.Printf("Config Error: Provider '%s' is not defined in 'models.providers' section of liteclaw.json\n", providerName)
		os.Exit(1)
	}

	baseURL := p.BaseURL
	if baseURL == "" {
		fmt.Printf("Config Error: baseUrl for provider '%s' is empty. Please set it in 'models.providers.%s.baseUrl'\n", providerName, providerName)
		os.Exit(1)
	}

	// 2. Check API Key
	// Convention: PROVIDERNAME_API_KEY (e.g. MINIMAX_API_KEY)
	targetKey := strings.ToUpper(providerName) + "_API_KEY"
	apiKey := os.Getenv(targetKey)

	// Fallback to cfg.Env (case-insensitive)
	if apiKey == "" {
		for k, v := range cfg.Env {
			if strings.EqualFold(k, targetKey) {
				apiKey = v
				break
			}
		}
	}

	if apiKey == "" {
		// Special case: Ollama typically doesn't require API key
		if strings.EqualFold(providerName, "ollama") {
			apiKey = "ollama" // Use placeholder
		} else {
			fmt.Printf("Config Error: API Key '%s' is missing.\n", targetKey)
			fmt.Printf("  To set it, run: liteclaw models auth login %s\n", providerName)
			fmt.Printf("  Or add '%s' to the 'env' section of liteclaw.json\n", targetKey)
			os.Exit(1)
		}
	}

	// 3. Init Provider
	// Default behavior based on config "api" field
	switch p.API {
	case "anthropic-messages":
		prov := llm.NewAnthropicProvider(apiKey, baseURL)
		prov.Verbose = cfg.Logging.Verbose
		provider = prov
	case "openai-completions", "":
		// Default to OpenAI
		prov := llm.NewOpenAIProviderWithConfig(apiKey, baseURL)
		prov.Verbose = cfg.Logging.Verbose
		provider = prov
	default:
		// Fallback or error? For now default to OpenAI to be safe
		fmt.Printf("Warning: Unknown API type '%s' for provider '%s'. Defaulting to OpenAI.\n", p.API, providerName)
		prov := llm.NewOpenAIProviderWithConfig(apiKey, baseURL)
		prov.Verbose = cfg.Logging.Verbose
		provider = prov
	}

	// Create Agent
	ag := New("main", "LiteClaw", model, provider)
	ag.Policy = cfg.Agents.Defaults.Tools
	ag.Stream = cfg.Agents.Defaults.Stream
	// Try to resolve max tokens from model config
	maxTokens := 4096
	for _, m := range p.Models {
		if m.ID == model && m.MaxTokens > 0 {
			maxTokens = m.MaxTokens
			break
		}
	}
	// We'll pass this in ChatRequest during agent.Run
	ag.MaxTokens = maxTokens
	ag.Temperature = 0.7
	ag.LogSystemPrompt = cfg.Logging.PrintSystemPrompt

	// Ensure Workspace
	workspaceDir := cfg.Agents.Defaults.Workspace
	if workspaceDir == "" {
		workspaceDir = workspace.ResolveDefaultDir()
	}
	if err := workspace.EnsureWorkspace(workspaceDir); err != nil {
		if cfg.Logging.Verbose {
			fmt.Printf("Failed to ensure workspace: %v\n", err)
		}
	}

	// Init MCP Manager
	mcpConfigPath := config.ExtrasPath()
	if _, err := os.Stat("configs/liteclaw.extras.json"); err == nil {
		mcpConfigPath = "configs/liteclaw.extras.json"
	}

	mcpManager := mcp.NewManager(mcpConfigPath)
	if err := mcpManager.LoadConfig(); err == nil {
		if cfg.Logging.Verbose {
			fmt.Printf("MCP configured with servers (%s). Starting initial discovery...\n", mcpConfigPath)
		}
		// Initial discovery in background
		mcpManager.Verbose = cfg.Logging.Verbose
		go func() { _ = mcpManager.DiscoverTools(context.Background()) }()
		ag.MCPManager = mcpManager
	}

	// Init Scheduler
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if !cfg.Logging.Verbose {
		// If not verbose, we can silence the local logger used by the scheduler
		logger = logger.Level(zerolog.WarnLevel)
	}
	// Persist jobs in workspace/data/cron_jobs.json
	cronStorePath := filepath.Join(workspaceDir, "data", "cron_jobs.json")
	sched := cron.NewScheduler(cronStorePath, logger)

	// Executor allows the scheduler to simply trigger "job X is pending", and we handle logic here
	sched.SetExecutor(func(ctx context.Context, job *cron.Job) error {
		var text string
		var sessionID string

		if job.Payload.Kind == cron.PayloadKindAgentTurn {
			text = job.Payload.Message
			sessionID = "cron:" + job.ID
		} else {
			// SystemEvent
			text = job.Payload.Text
			sessionID = "main" // Or resolve main session key
		}

		if text == "" {
			return nil
		}

		fmt.Printf("[CRON] Executing %s job: %s (session: %s)\n", job.Payload.Kind, text, sessionID)

		// Run agent
		// Note: We might need a fresh context if the job ctx is cancelled too early,
		// but usually job ctx is tied to the task duration.
		stream, err := ag.Run(ctx, sessionID, text)
		if err != nil {
			return err
		}

		var rawBuffer strings.Builder
		var finalResponse strings.Builder

		// Must drain the stream for execution to complete
		for evt := range stream {
			switch evt.Type {
			case "text":
				rawBuffer.WriteString(evt.Content)
			case "error":
				fmt.Printf("[CRON] Error during job execution: %s\n", evt.Error)
			}
		}

		finalResponse.WriteString(sanitizeModelOutput(rawBuffer.String()))

		// Handle Delivery
		// Default to deliver=true for agentTurn if not specified (common expectation)
		shouldDeliver := job.Payload.Deliver
		if job.Payload.Kind == cron.PayloadKindAgentTurn && !shouldDeliver {
			// If explicitly false, we rely on user settings. But currently jobs.json might omit it.
			// Let's assume true for agentTurn unless Payload.Deliver is explicitly tracked.
			// Actually Payload struct usually defaults bool to false.
			// Let's rely on the explicit flag or force it for now if it feels like a user query?
			// The user explicitly added "deliver": true in their jobs.json example.
			// But for parity with TS: TS defaults bestEffortDeliver=true if not specified?
			// Let's trust the flag for now, BUT since the user complained, let's treat agentTurn as implicitly deliverable if it produced text.
			shouldDeliver = true
		}

		if shouldDeliver && sender != nil && finalResponse.Len() > 0 {
			// Use implicit delivery (empty channel/target) -> Gateway resolves last active session
			// Note: We pass original channel/to from payload if they exist, but here we assume implicity
			// payload.Channel / payload.To are available in jobStruct.

			// Try explicit payload target first
			channel := job.Payload.Channel
			target := job.Payload.To

			// If "last" is used or empty, pass empty to let gateway resolve
			if channel == "last" {
				channel = ""
			}

			fmt.Printf("[CRON] Delivering response via sender (len=%d)\n", finalResponse.Len())
			err := sender.SendMessage(ctx, channel, target, finalResponse.String())
			if err != nil {
				fmt.Printf("[CRON] Failed to deliver response: %v\n", err)
			}
		}

		return nil
	})

	// Load persisted jobs
	if err := sched.Load(); err != nil && cfg.Logging.Verbose {
		fmt.Printf("Warning: failed to load cron jobs: %v\n", err)
	}

	sched.Start()

	// Register Tools
	ag.RegisterTools(
		tools.NewExecTool(),
		tools.NewReadTool(),
		tools.NewWriteTool(),
		tools.NewEditTool(),
		tools.NewListTool(),
		tools.NewMemorySearchTool(workspaceDir),
		tools.NewMemoryGetTool(workspaceDir),
		// New tools for prompt parity
		tools.NewGatewayTool(),
		tools.NewMessageTool(sender),
		tools.NewAgentsListTool(),
		tools.NewSessionStatusTool(),
		tools.NewSessionsListTool(),
		tools.NewSessionsSendTool(),
		tools.NewSessionsHistoryTool(),
		tools.NewSessionsSpawnTool(),
		tools.NewWebSearchTool(),
		tools.NewWebFetchTool(),
		tools.NewProcessTool(),
		tools.NewBrowserTool(),
		tools.NewCanvasTool(),
		tools.NewNodesTool(),
		tools.NewCronTool(sched),
		tools.NewTtsTool(),
		tools.NewImageTool(workspaceDir),
	)

	// Dynamically extract tool names
	toolNames := ag.ExtractToolNames()

	// Load Skills
	// Scan workspace/skills, managed skills (~/.liteclaw/skills), and repo-root/skills (dev mode)
	cwd, _ := os.Getwd()
	repoSkillsDir := filepath.Join(cwd, "../skills")
	moltGoSkillsDir := filepath.Join(cwd, "skills")

	// Managed skills directory - where `skill install` puts downloaded skills
	managedSkillsDir := filepath.Join(config.StateDir(), "skills")

	skillLoader := skills.NewLoader(
		filepath.Join(workspaceDir, "skills"), // workspace skills (~/clawd/skills)
		managedSkillsDir,                      // managed skills (~/.liteclaw/skills) - from ClawdHub
		moltGoSkillsDir,                       // bundled skills (./skills)
		repoSkillsDir,                         // dev mode (../skills)
	)

	loadedSkills, err := skillLoader.LoadAll()
	if err != nil {
		fmt.Printf("Warning: failed to load skills: %v\n", err)
	} else {
		// Verify op skill specific requirement
		for _, s := range loadedSkills {
			if s.Name == "1password" {
				// Debug log for 1password eligibility
				if !s.IsEligible() && cfg.Logging.Verbose {
					fmt.Printf("Skill '1password' found but not eligible (missing 'op' binary? PATH=%s)\n", os.Getenv("PATH"))
				}
			}
		}
	}

	eligibleSkills := skills.FilterEligible(loadedSkills)
	if cfg.Logging.Verbose {
		fmt.Printf("Loaded %d skills (Eligible: %d) from [%s, %s, %s, %s]\n",
			len(loadedSkills), len(eligibleSkills),
			filepath.Join(workspaceDir, "skills"), managedSkillsDir, moltGoSkillsDir, repoSkillsDir)
	}

	skillsPrompt := skills.FormatForPrompt(eligibleSkills)

	// Build Model Aliases
	var modelAliases []string
	if cfg.Agents.Defaults.Models != nil {
		for name, mapCfg := range cfg.Agents.Defaults.Models {
			// Simple capitalization for display
			displayName := strings.ToUpper(name[:1]) + name[1:]
			modelAliases = append(modelAliases, fmt.Sprintf("%s: %s", displayName, mapCfg.Alias))
		}
	}

	hostname, _ := os.Hostname()

	// Build System Prompt using new Builder
	builder := prompt.NewBuilder(workspaceDir).
		WithTools(toolNames).
		WithSkillsPrompt(skillsPrompt).
		WithModelAliases(modelAliases).
		WithDocsPath("/opt/clawdbot/docs").
		WithWorkspaceNotes([]string{"Reminder: commit your changes in this workspace after edits."}).
		WithReasoningTagHint(false).
		WithConfig("off"). // Default reasoning
		WithRuntimeInfo(prompt.RuntimeInfo{
			OS:           runtime.GOOS,
			Arch:         runtime.GOARCH,
			Model:        model,
			DefaultModel: model,
			Channel:      "telegram", // TODO: make dynamic based on adapter used
			AgentID:      ag.ID,
			Host:         hostname,
			GoVersion:    runtime.Version(),
			RepoRoot:     cwd, // Assuming cwd is repo root
			Thinking:     "off",
		})

	if sysPrompt, err := builder.Build(); err == nil {
		ag.SystemPrompt = sysPrompt
	} else {
		// Fallback
		fmt.Printf("Error building system prompt: %v\n", err)
		ag.SystemPrompt = "You are a helpful assistant."
	}

	ag.Verbose = cfg.Logging.Verbose

	return &Service{
		Config:    cfg,
		Agent:     ag,
		Scheduler: sched,
		Verbose:   cfg.Logging.Verbose,
	}
}

func (s *Service) GetScheduler() *cron.Scheduler {
	return s.Scheduler
}

// ProcessChat handles the chat.send request
func (s *Service) ProcessChat(ctx context.Context, sessionID, message string, onDelta func(string)) error {
	if s.Agent.Provider == nil {
		onDelta("No API keys configured (MINIMAX_API_KEY or OPENAI_API_KEY). Echo: " + message)
		return nil
	}

	events, err := s.Agent.Run(ctx, sessionID, message)
	if err != nil {
		return err
	}

	var rawBuffer strings.Builder
	var lastOutput string
	for event := range events {
		switch event.Type {
		case "text":
			rawBuffer.WriteString(event.Content)
			sanitized := sanitizeModelOutput(rawBuffer.String())
			if len(sanitized) > len(lastOutput) {
				onDelta(sanitized[len(lastOutput):])
				lastOutput = sanitized
			}
		case "tool_call":
			// Tool calls are silent to the user
		case "tool_result":
			// Tool results are silent to the user
		case "error":
			return fmt.Errorf("agent error: %s", event.Error)
		}
	}

	return nil
}

// LoadSessionHistory loads persisted history into the agent's session.
// Call this before ProcessChat to restore conversation context.
func (s *Service) LoadSessionHistory(sessionID string, messages []Message) {
	if s.Agent != nil {
		s.Agent.LoadHistoryForSession(sessionID, messages)
	}
}

// HasSession checks if the agent has this session in memory.
func (s *Service) HasSession(sessionID string) bool {
	if s.Agent != nil {
		return s.Agent.HasSession(sessionID)
	}
	return false
}

func sanitizeModelOutput(raw string) string {
	if strings.Contains(raw, "<final>") {
		return extractFinalContent(raw)
	}
	return stripThinkContent(raw)
}

func stripThinkContent(raw string) string {
	var out strings.Builder
	for i := 0; i < len(raw); {
		if strings.HasPrefix(raw[i:], "<think>") {
			i += len("<think>")
			end := strings.Index(raw[i:], "</think>")
			if end == -1 {
				return out.String()
			}
			i += end + len("</think>")
			continue
		}
		if strings.HasPrefix(raw[i:], "</think>") {
			i += len("</think>")
			continue
		}
		out.WriteByte(raw[i])
		i++
	}
	return out.String()
}

func extractFinalContent(raw string) string {
	var out strings.Builder
	inFinal := false
	for i := 0; i < len(raw); {
		if !inFinal && strings.HasPrefix(raw[i:], "<final>") {
			inFinal = true
			i += len("<final>")
			continue
		}
		if inFinal && strings.HasPrefix(raw[i:], "</final>") {
			inFinal = false
			i += len("</final>")
			continue
		}
		if inFinal {
			out.WriteByte(raw[i])
		}
		i++
	}
	return out.String()
}
