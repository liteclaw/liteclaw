package commands

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xen0n/go-workwx"
	"golang.org/x/term"

	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/liteclaw/liteclaw/internal/version"
)

//go:embed onboard_models.json
var onboardModelsFS embed.FS

//go:embed onboard_channels.json
var onboardChannelsFS embed.FS

type onboardModelEntry struct {
	Provider       string           `json:"provider"`
	ModelID        string           `json:"id"`
	Name           string           `json:"name"`
	API            string           `json:"api"`
	BaseURL        string           `json:"baseUrl"`
	Reasoning      bool             `json:"reasoning"`
	Input          []string         `json:"input"`
	ContextWindow  int              `json:"contextWindow"`
	MaxTokens      int              `json:"maxTokens"`
	Cost           config.ModelCost `json:"cost"`
	Display        string           `json:"display"`
	DefaultDisplay string           `json:"defaultDisplay"`
}

type onboardChannelField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"` // string|int|bool
	Required bool   `json:"required"`
	Default  string `json:"default"`
	Secret   bool   `json:"secret"`
}

type onboardChannelEntry struct {
	Key         string                `json:"key"`
	Label       string                `json:"label"`
	Description string                `json:"description"`
	Fields      []onboardChannelField `json:"fields"`
}

func runConfigureWizard(cmd interface {
	OutOrStdout() io.Writer
	ErrOrStderr() io.Writer
}, runCommand string) error {
	out := cmd.OutOrStdout()
	reader := bufio.NewReader(os.Stdin)

	fmt.Fprintln(out, "LiteClaw Onboarding Wizard")
	fmt.Fprintln(out, "==========================")
	fmt.Fprintln(out, "Press Enter to accept defaults.")
	fmt.Fprintln(out, "")

	home, _ := os.UserHomeDir()
	defaultWorkspace := filepath.Join(home, "clawd")

	models, err := loadOnboardModels()
	if err != nil {
		return err
	}

	selected, err := selectOnboardModel(reader, out, models)
	if err != nil {
		return err
	}

	provider := selected.Provider
	modelID := selected.ModelID
	baseURL := selected.BaseURL
	apiType := selected.API

	apiKeyEnv := strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_API_KEY"
	apiKey := promptSecret(reader, out, fmt.Sprintf("API key (%s)", apiKeyEnv), provider == "ollama")
	if apiKey == "" && provider != "ollama" {
		return fmt.Errorf("API key is required for provider %s", provider)
	}

	workspace := prompt(reader, out, "Workspace directory", defaultWorkspace)
	if workspace == "" {
		workspace = defaultWorkspace
	}

	channelEntries, err := loadOnboardChannels()
	if err != nil {
		return err
	}
	channelValues, err := selectOnboardChannels(reader, out, channelEntries)
	if err != nil {
		return err
	}

	cfg := &config.Config{
		Meta: config.MetaConfig{
			LastTouchedVersion: version.Version,
			LastTouchedAt:      time.Now().UTC().Format(time.RFC3339),
		},
		Env: map[string]string{},
		Wizard: config.WizardConfig{
			LastRunAt:      time.Now().UTC().Format(time.RFC3339),
			LastRunVersion: version.Version,
			LastRunCommand: runCommand,
			LastRunMode:    "local",
		},
		Models: config.ModelsConfig{
			Mode: "merge",
			Providers: map[string]config.ModelProvider{
				provider: {
					BaseURL: baseURL,
					API:     apiType,
					Models: []config.ModelEntry{
						{
							ID:            modelID,
							Name:          selected.Name,
							Reasoning:     selected.Reasoning,
							Input:         selected.Input,
							Cost:          selected.Cost,
							ContextWindow: selected.ContextWindow,
							MaxTokens:     selected.MaxTokens,
						},
					},
				},
			},
		},
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Model:     config.AgentModelConfig{Primary: fmt.Sprintf("%s/%s", provider, modelID)},
				Workspace: workspace,
			},
		},
		Gateway: config.GatewayConfig{
			Port: 18789,
			Mode: "local",
			Bind: "loopback",
			Auth: config.GatewayAuth{
				Mode:  "token",
				Token: uuid.NewString(),
			},
		},
		Channels: config.ChannelsConfig{
			IMessage: config.IMessageConfig{Enabled: false},
		},
	}

	if apiKey != "" {
		cfg.Env[apiKeyEnv] = apiKey
	}

	if err := applyChannelSelections(cfg, channelValues); err != nil {
		return err
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "✅ Config saved to %s\n", config.ConfigPath())
	fmt.Fprintln(out, "Next steps:")
	fmt.Fprintln(out, "  liteclaw gateway start --detached")
	fmt.Fprintln(out, `  liteclaw agent --message "hello"`)
	return nil
}

func loadOnboardChannels() ([]onboardChannelEntry, error) {
	data, err := onboardChannelsFS.ReadFile("onboard_channels.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load onboard channels: %w", err)
	}
	var channels []onboardChannelEntry
	if err := json.Unmarshal(data, &channels); err != nil {
		return nil, fmt.Errorf("invalid onboard channels: %w", err)
	}
	// Keep original order from JSON file (no sorting)
	return channels, nil
}

func selectOnboardChannels(reader *bufio.Reader, out io.Writer, channels []onboardChannelEntry) (map[string]map[string]string, error) {
	if len(channels) == 0 {
		return map[string]map[string]string{}, nil
	}

	selected := map[int]bool{}
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
		keys, err := runChannelMultiSelect(channels)
		if err != nil {
			return nil, err
		}
		for i, ch := range channels {
			for _, key := range keys {
				if ch.Key == key {
					selected[i] = true
				}
			}
		}
	} else {
		fmt.Fprintln(out, "Channel setup (optional)")
		for i, ch := range channels {
			label := ch.Label
			if ch.Description != "" {
				label = fmt.Sprintf("%s — %s", label, ch.Description)
			}
			fmt.Fprintf(out, "  %2d) %s\n", i+1, label)
		}
		fmt.Fprintln(out, "")

		choice := prompt(reader, out, "Select channels by number (comma-separated)", "")
		choice = strings.TrimSpace(choice)
		if choice == "" {
			return map[string]map[string]string{}, nil
		}

		for _, part := range strings.Split(choice, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 1 || idx > len(channels) {
				return nil, fmt.Errorf("invalid channel selection: %s", part)
			}
			selected[idx-1] = true
		}
	}

	values := map[string]map[string]string{}
	for i, ch := range channels {
		if !selected[i] {
			continue
		}

		// Special handling for WeCom: start server after port, ask botId last
		if ch.Key == "wecom" {
			fieldValues, err := collectWeComFields(reader, out, ch)
			if err != nil {
				return nil, err
			}
			values[ch.Key] = fieldValues
			continue
		}

		fieldValues := map[string]string{}
		for _, field := range ch.Fields {
			val := ""
			if field.Secret {
				val = promptSecret(reader, out, field.Label, !field.Required)
			} else if field.Type == "bool" {
				val = strconv.FormatBool(promptYesNo(reader, out, field.Label, field.Default == "true"))
			} else {
				val = prompt(reader, out, field.Label, field.Default)
			}

			val = strings.TrimSpace(val)
			if val == "" && field.Required {
				return nil, fmt.Errorf("%s is required for %s", field.Label, ch.Label)
			}
			fieldValues[field.Key] = val
		}
		values[ch.Key] = fieldValues
	}

	return values, nil
}

// collectWeComFields handles WeCom onboarding with special flow:
// 1. Collect token, encodingAesKey, port
// 2. Start temporary callback server
// 3. User creates bot on WeCom platform (triggers URL verification)
// 4. Collect botId
// 5. Stop server
func collectWeComFields(reader *bufio.Reader, out io.Writer, ch onboardChannelEntry) (map[string]string, error) {
	fieldValues := map[string]string{}

	// Step 1: Collect token
	token := promptSecret(reader, out, "WeCom token", false)
	if token == "" {
		return nil, fmt.Errorf("WeCom token is required")
	}
	fieldValues["token"] = token

	// Step 2: Collect encodingAesKey
	encodingAesKey := promptSecret(reader, out, "WeCom encodingAesKey", false)
	if encodingAesKey == "" {
		return nil, fmt.Errorf("WeCom encodingAesKey is required")
	}
	fieldValues["encodingAesKey"] = encodingAesKey

	// Step 3: Collect port
	portStr := prompt(reader, out, "WeCom callback port", "10456")
	port := 10456
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %s", portStr)
		}
		port = p
	}
	fieldValues["port"] = strconv.Itoa(port)

	// Step 4: Start temporary WeCom callback server
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "🚀 Starting WeCom callback server...")

	server, err := startWeComCallbackServer(token, encodingAesKey, port, out)
	if err != nil {
		return nil, fmt.Errorf("failed to start WeCom callback server: %w", err)
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "✅ WeCom callback server is running on port", port)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "📋 Next steps:")
	fmt.Fprintln(out, "   1. Go to WeCom admin console (企业微信管理后台)")
	fmt.Fprintln(out, "   2. Create a new Intelligent Bot (智能机器人)")
	fmt.Fprintln(out, "   3. Set callback URL to: http://YOUR_PUBLIC_IP:"+strconv.Itoa(port)+"/wecom/callback")
	fmt.Fprintln(out, "   4. WeCom will verify the URL by calling our server")
	fmt.Fprintln(out, "   5. After verification, you'll get a Bot ID")
	fmt.Fprintln(out, "")

	// Step 5: Collect botId (after user creates bot on WeCom platform)
	botId := prompt(reader, out, "WeCom botId (from WeCom after URL verification)", "")
	if botId == "" {
		// Stop server before returning error
		stopWeComCallbackServer(server)
		return nil, fmt.Errorf("WeCom botId is required")
	}
	fieldValues["botId"] = botId

	// Step 6: Collect showThinking (optional)
	showThinking := promptYesNo(reader, out, "Show thinking in WeCom responses", false)
	fieldValues["showThinking"] = strconv.FormatBool(showThinking)

	// Step 7: Stop server
	stopWeComCallbackServer(server)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "✅ WeCom callback server stopped. Configuration complete.")

	return fieldValues, nil
}

// wecomVerifyHandler implements workwx.RxMessageHandler for URL verification only
type wecomVerifyHandler struct {
	out io.Writer
}

func (h *wecomVerifyHandler) OnIncomingMessage(msg *workwx.RxMessage) error {
	fmt.Fprintln(h.out, "📨 Received WeCom message (verification or test)")
	return nil
}

func startWeComCallbackServer(token, encodingAesKey string, port int, out io.Writer) (*http.Server, error) {
	handler := &wecomVerifyHandler{out: out}
	wxHandler, err := workwx.NewHTTPHandler(token, encodingAesKey, handler)
	if err != nil {
		return nil, fmt.Errorf("failed to create WeCom handler: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/wecom/callback", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(out, "🔔 WeCom callback request: %s %s\n", r.Method, r.URL.String())
		wxHandler.ServeHTTP(w, r)
	})
	// Also handle root path in case user configures it differently
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/wecom/callback" {
			fmt.Fprintf(out, "🔔 WeCom callback request: %s %s\n", r.Method, r.URL.String())
			wxHandler.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(out, "⚠️  WeCom callback server error: %v\n", err)
		}
	}()

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	return server, nil
}

func stopWeComCallbackServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func applyChannelSelections(cfg *config.Config, values map[string]map[string]string) error {
	for key, fields := range values {
		switch key {
		case "telegram":
			cfg.Channels.Telegram.Enabled = true
			cfg.Channels.Telegram.DMPolicy = "pairing"
			cfg.Channels.Telegram.GroupPolicy = "allowlist"
			cfg.Channels.Telegram.StreamMode = "partial"
			cfg.Channels.Telegram.BotToken = fields["botToken"]
		case "discord":
			cfg.Channels.Discord.Enabled = true
			cfg.Channels.Discord.DMPolicy = "pairing"
			cfg.Channels.Discord.GroupPolicy = "allowlist"
			cfg.Channels.Discord.Token = fields["token"]
		case "imessage":
			cfg.Channels.IMessage.Enabled = true
			cfg.Channels.IMessage.DBPath = fields["dbPath"]
		case "qq":
			cfg.Channels.QQ.Enabled = true
			if v := fields["appId"]; v != "" {
				if n, err := strconv.ParseUint(v, 10, 64); err == nil {
					cfg.Channels.QQ.AppID = n
				}
			}
			cfg.Channels.QQ.AppSecret = fields["appSecret"]
			if v := fields["sandbox"]; v != "" {
				cfg.Channels.QQ.Sandbox = v == "true"
			}
		case "feishu":
			cfg.Channels.Feishu.Enabled = true
			cfg.Channels.Feishu.AppID = fields["appId"]
			cfg.Channels.Feishu.AppSecret = fields["appSecret"]
			cfg.Channels.Feishu.EncryptKey = fields["encryptKey"]
			cfg.Channels.Feishu.VerificationToken = fields["verificationToken"]
		case "dingtalk":
			cfg.Channels.DingTalk.Enabled = true
			cfg.Channels.DingTalk.AppKey = fields["appKey"]
			cfg.Channels.DingTalk.AppSecret = fields["appSecret"]
		case "wecom":
			cfg.Channels.WeCom.Enabled = true
			cfg.Channels.WeCom.Token = fields["token"]
			cfg.Channels.WeCom.EncodingAESKey = fields["encodingAesKey"]
			cfg.Channels.WeCom.BotID = fields["botId"]
			if v := fields["port"]; v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					cfg.Channels.WeCom.Port = n
				}
			}
			if v := fields["showThinking"]; v != "" {
				cfg.Channels.WeCom.ShowThinking = v == "true"
			}
		}
	}
	return nil
}

func loadOnboardModels() ([]onboardModelEntry, error) {
	data, err := onboardModelsFS.ReadFile("onboard_models.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load onboard models: %w", err)
	}
	var models []onboardModelEntry
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, fmt.Errorf("invalid onboard models: %w", err)
	}
	for i := range models {
		if models[i].Display == "" {
			models[i].Display = fmt.Sprintf("%s/%s", models[i].Provider, models[i].ModelID)
		}
	}
	// Keep original order from JSON file (no sorting)
	return models, nil
}

func selectOnboardModel(reader *bufio.Reader, out io.Writer, models []onboardModelEntry) (onboardModelEntry, error) {
	if len(models) == 0 {
		return onboardModelEntry{}, fmt.Errorf("no onboard models configured")
	}

	fmt.Fprintln(out, "Model / auth provider")
	for i, m := range models {
		label := m.Display
		if m.Name != "" && m.Name != m.ModelID {
			label = fmt.Sprintf("%s — %s", label, m.Name)
		}
		fmt.Fprintf(out, "  %2d) %s\n", i+1, label)
	}
	// Add custom option
	customIdx := len(models) + 1
	fmt.Fprintf(out, "  %2d) ✏️  Custom model (enter provider/model)\n", customIdx)
	fmt.Fprintln(out, "")

	for {
		choice := prompt(reader, out, "Select model by number", "1")
		idx, err := strconv.Atoi(choice)
		if err != nil || idx < 1 || idx > customIdx {
			fmt.Fprintf(out, "Please enter a number between 1 and %d.\n", customIdx)
			continue
		}

		// User selected custom model
		if idx == customIdx {
			return promptCustomModel(reader, out)
		}

		return models[idx-1], nil
	}
}

func promptCustomModel(reader *bufio.Reader, out io.Writer) (onboardModelEntry, error) {
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Enter custom model in format: provider/model")
	fmt.Fprintln(out, "Example: openai/gpt-4-turbo, anthropic/claude-3-opus-20240229")
	fmt.Fprintln(out, "")

	for {
		input := prompt(reader, out, "Custom model (provider/model)", "")
		if input == "" {
			fmt.Fprintln(out, "❌ Model is required. Please enter in format: provider/model")
			continue
		}

		// Validate format: must contain exactly one /
		parts := strings.Split(input, "/")
		if len(parts) != 2 {
			fmt.Fprintln(out, "❌ Invalid format. Must be: provider/model (e.g., openai/gpt-4)")
			continue
		}

		provider := strings.TrimSpace(parts[0])
		modelID := strings.TrimSpace(parts[1])

		if provider == "" {
			fmt.Fprintln(out, "❌ Provider cannot be empty. Example: openai/gpt-4")
			continue
		}
		if modelID == "" {
			fmt.Fprintln(out, "❌ Model cannot be empty. Example: openai/gpt-4")
			continue
		}

		// Prompt for base URL
		fmt.Fprintln(out, "")
		baseURL := prompt(reader, out, "Base URL (API endpoint)", "https://api.openai.com/v1")

		// Create custom model entry
		entry := onboardModelEntry{
			Provider:      provider,
			ModelID:       modelID,
			Name:          fmt.Sprintf("Custom: %s/%s", provider, modelID),
			Display:       fmt.Sprintf("%s/%s", provider, modelID),
			API:           "openai-completions", // Default to OpenAI-compatible
			BaseURL:       baseURL,
			ContextWindow: 128000,
			MaxTokens:     8192,
		}

		fmt.Fprintf(out, "✅ Using custom model: %s/%s\n", provider, modelID)
		return entry, nil
	}
}

func prompt(reader *bufio.Reader, out io.Writer, label, def string) string {
	if def != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(text)
	if text == "" {
		return def
	}
	return text
}

func promptYesNo(reader *bufio.Reader, out io.Writer, label string, def bool) bool {
	defStr := "n"
	if def {
		defStr = "y"
	}
	for {
		fmt.Fprintf(out, "%s (y/N) [%s]: ", label, defStr)
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(strings.ToLower(text))
		if text == "" {
			return def
		}
		if text == "y" || text == "yes" {
			return true
		}
		if text == "n" || text == "no" {
			return false
		}
		fmt.Fprintln(out, "Please enter y or n.")
	}
}

func promptSecret(reader *bufio.Reader, out io.Writer, label string, optional bool) string {
	fmt.Fprintf(out, "%s: ", label)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(out, "")
		if err == nil {
			text := strings.TrimSpace(string(bytes))
			if text == "" && optional {
				return ""
			}
			return text
		}
	}
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(text)
	if text == "" && optional {
		return ""
	}
	return text
}
