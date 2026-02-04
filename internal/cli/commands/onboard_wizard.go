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

	_, _ = fmt.Fprintln(out, "LiteClaw Onboarding Wizard")
	_, _ = fmt.Fprintln(out, "==========================")
	_, _ = fmt.Fprintln(out, "Press Enter to accept defaults.")
	_, _ = fmt.Fprintln(out, "")

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

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintf(out, "✅ Config saved to %s\n", config.ConfigPath())
	_, _ = fmt.Fprintln(out, "Next steps:")
	_, _ = fmt.Fprintln(out, "  liteclaw gateway start --detached")
	_, _ = fmt.Fprintln(out, `  liteclaw agent --message "hello"`)
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
		_, _ = fmt.Fprintln(out, "Channel setup (optional)")
		for i, ch := range channels {
			label := ch.Label
			if ch.Description != "" {
				label = fmt.Sprintf("%s — %s", label, ch.Description)
			}
			_, _ = fmt.Fprintf(out, "  %2d) %s\n", i+1, label)
		}
		_, _ = fmt.Fprintln(out, "")

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
// 2. Start temporary callback server immediately after port is entered
// 3. User configures WeCom platform with callback URL and gets botId
// 4. Collect botId
// 5. Stop server
func collectWeComFields(reader *bufio.Reader, out io.Writer, ch onboardChannelEntry) (map[string]string, error) {
	fieldValues := map[string]string{}

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "🔧 企业微信 (WeCom) 配置向导")
	_, _ = fmt.Fprintln(out, "════════════════════════════")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "首先，请在企业微信管理后台创建一个机器人应用：")
	_, _ = fmt.Fprintln(out, "📋 请按以下步骤完成企业微信配置：")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "   1️⃣  打开：企业微信管理后台 → 安全与管理 → 管理工具 → 智能机器人 → 创建机器人")
	_, _ = fmt.Fprintln(out, "   创建机器人后，先不要填写URL，先获取以下信息：")
	_, _ = fmt.Fprintln(out, "  • Token (令牌)")
	_, _ = fmt.Fprintln(out, "  • EncodingAESKey (消息加解密密钥)")
	_, _ = fmt.Fprintln(out, "")

	// Step 1: Collect token
	token := promptSecret(reader, out, "Token (令牌)", false)
	if token == "" {
		return nil, fmt.Errorf("WeCom token is required")
	}
	fieldValues["token"] = token

	// Step 2: Collect encodingAesKey
	encodingAesKey := promptSecret(reader, out, "EncodingAESKey (消息加解密密钥)", false)
	if encodingAesKey == "" {
		return nil, fmt.Errorf("WeCom encodingAesKey is required")
	}
	fieldValues["encodingAesKey"] = encodingAesKey

	// Step 3: Collect port
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "📡 回调服务器端口配置")
	_, _ = fmt.Fprintln(out, "   请确保此端口可从公网访问（可能需要配置防火墙或端口转发）")
	portStr := prompt(reader, out, "回调端口 (Callback Port)", "10456")
	port := 10456
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %s", portStr)
		}
		port = p
	}
	fieldValues["port"] = strconv.Itoa(port)

	// Step 4: Start temporary WeCom callback server IMMEDIATELY after port is entered
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "🚀 正在启动 WeCom 回调服务器...")

	server, err := startWeComCallbackServer(token, encodingAesKey, port, out)
	if err != nil {
		return nil, fmt.Errorf("failed to start WeCom callback server: %w", err)
	}

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "✅ 回调服务器已启动！")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "   2️⃣  填入以下回调 URL：")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintf(out, "       🔗  http://<你的公网IP>:%d/wecom\n", port)
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "       ⚠️  请将 <你的公网IP> 替换为你的服务器公网 IP 地址")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "   3️⃣  点击 \"保存\" 后，企业微信会验证此 URL")
	_, _ = fmt.Fprintln(out, "       如果验证成功，你将看到 \"📨 Received WeCom message\" 的日志")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "   4️⃣  配置成功后，复制页面上显示的 \"BotID\"")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "════════════════════════════════════════════════════════════")

	// Step 5: Collect botId (after user configures WeCom platform)
	botId := prompt(reader, out, "请输入 BotID (从企业微信后台获取)", "")
	if botId == "" {
		// Stop server before returning error
		stopWeComCallbackServer(server)
		return nil, fmt.Errorf("WeCom botId is required")
	}
	fieldValues["botId"] = botId

	// showThinking defaults to false
	fieldValues["showThinking"] = "false"

	// Step 6: Stop server
	stopWeComCallbackServer(server)
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "✅ 企业微信配置完成！回调服务器已停止。")
	_, _ = fmt.Fprintln(out, "   启动 Gateway 后，回调服务器将自动运行。")

	return fieldValues, nil
}

// wecomVerifyHandler implements workwx.RxMessageHandler for URL verification only
type wecomVerifyHandler struct {
	out io.Writer
}

func (h *wecomVerifyHandler) OnIncomingMessage(msg *workwx.RxMessage) error {
	_, _ = fmt.Fprintln(h.out, "📨 Received WeCom message (verification or test)")
	return nil
}

func startWeComCallbackServer(token, encodingAesKey string, port int, out io.Writer) (*http.Server, error) {
	handler := &wecomVerifyHandler{out: out}
	wxHandler, err := workwx.NewHTTPHandler(token, encodingAesKey, handler)
	if err != nil {
		return nil, fmt.Errorf("failed to create WeCom handler: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/wecom", func(w http.ResponseWriter, r *http.Request) {
		// Print newline first to avoid mixing with user input prompt
		_, _ = fmt.Fprintln(out, "")
		_, _ = fmt.Fprintln(out, "────────────────────────────────────────")
		_, _ = fmt.Fprintf(out, "🔔 收到企业微信验证请求: %s %s\n", r.Method, r.URL.Path)
		wxHandler.ServeHTTP(w, r)
		_, _ = fmt.Fprintln(out, "✅ 验证成功！企业微信已确认回调 URL 有效")
		_, _ = fmt.Fprintln(out, "────────────────────────────────────────")
		_, _ = fmt.Fprintln(out, "")
		_, _ = fmt.Fprint(out, "请输入 BotID (从企业微信后台获取): ")
	})
	// Also handle root path in case user configures it differently
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/wecom" {
			_, _ = fmt.Fprintln(out, "")
			_, _ = fmt.Fprintln(out, "────────────────────────────────────────")
			_, _ = fmt.Fprintf(out, "🔔 收到企业微信验证请求: %s %s\n", r.Method, r.URL.Path)
			wxHandler.ServeHTTP(w, r)
			_, _ = fmt.Fprintln(out, "✅ 验证成功！企业微信已确认回调 URL 有效")
			_, _ = fmt.Fprintln(out, "────────────────────────────────────────")
			_, _ = fmt.Fprintln(out, "")
			_, _ = fmt.Fprint(out, "请输入 BotID (从企业微信后台获取): ")
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
			_, _ = fmt.Fprintf(out, "⚠️  WeCom callback server error: %v\n", err)
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

	_, _ = fmt.Fprintln(out, "Model / auth provider")
	for i, m := range models {
		label := m.Display
		if m.Name != "" && m.Name != m.ModelID {
			label = fmt.Sprintf("%s — %s", label, m.Name)
		}
		_, _ = fmt.Fprintf(out, "  %2d) %s\n", i+1, label)
	}
	// Add custom option
	customIdx := len(models) + 1
	_, _ = fmt.Fprintf(out, "  %2d) ✏️  Custom model (enter provider/model)\n", customIdx)
	_, _ = fmt.Fprintln(out, "")

	for {
		choice := prompt(reader, out, "Select model by number", "1")
		idx, err := strconv.Atoi(choice)
		if err != nil || idx < 1 || idx > customIdx {
			_, _ = fmt.Fprintf(out, "Please enter a number between 1 and %d.\n", customIdx)
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
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Enter custom model in format: provider/model")
	_, _ = fmt.Fprintln(out, "Example: openai/gpt-4-turbo, anthropic/claude-3-opus-20240229")
	_, _ = fmt.Fprintln(out, "")

	for {
		input := prompt(reader, out, "Custom model (provider/model)", "")
		if input == "" {
			_, _ = fmt.Fprintln(out, "❌ Model is required. Please enter in format: provider/model")
			continue
		}

		// Validate format: must contain exactly one /
		parts := strings.Split(input, "/")
		if len(parts) != 2 {
			_, _ = fmt.Fprintln(out, "❌ Invalid format. Must be: provider/model (e.g., openai/gpt-4)")
			continue
		}

		provider := strings.TrimSpace(parts[0])
		modelID := strings.TrimSpace(parts[1])

		if provider == "" {
			_, _ = fmt.Fprintln(out, "❌ Provider cannot be empty. Example: openai/gpt-4")
			continue
		}
		if modelID == "" {
			_, _ = fmt.Fprintln(out, "❌ Model cannot be empty. Example: openai/gpt-4")
			continue
		}

		// Prompt for base URL
		_, _ = fmt.Fprintln(out, "")
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

		_, _ = fmt.Fprintf(out, "✅ Using custom model: %s/%s\n", provider, modelID)
		return entry, nil
	}
}

func prompt(reader *bufio.Reader, out io.Writer, label, def string) string {
	if def != "" {
		_, _ = fmt.Fprintf(out, "%s [%s]: ", label, def)
	} else {
		_, _ = fmt.Fprintf(out, "%s: ", label)
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
		_, _ = fmt.Fprintf(out, "%s (y/N) [%s]: ", label, defStr)
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
		_, _ = fmt.Fprintln(out, "Please enter y or n.")
	}
}

func promptSecret(reader *bufio.Reader, out io.Writer, label string, optional bool) string {
	_, _ = fmt.Fprintf(out, "%s: ", label)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		_, _ = fmt.Fprintln(out, "")
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
