// Package config provides configuration management for LiteClaw.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liteclaw/liteclaw/internal/agent/policy"
	"github.com/spf13/viper"
)

// ErrConfigNotFound indicates no usable config file was found.
var ErrConfigNotFound = errors.New("config not found")

// Config matches the structure of liteclaw.json
type Config struct {
	Meta     MetaConfig        `json:"meta" yaml:"meta" mapstructure:"meta"`
	Env      map[string]string `json:"env" yaml:"env" mapstructure:"env"`
	Wizard   WizardConfig      `json:"wizard" yaml:"wizard" mapstructure:"wizard"`
	Auth     AuthConfig        `json:"auth" yaml:"auth" mapstructure:"auth"`
	Models   ModelsConfig      `json:"models" yaml:"models" mapstructure:"models"`
	Agents   AgentsConfig      `json:"agents" yaml:"agents" mapstructure:"agents"`
	Messages MessagesConfig    `json:"messages" yaml:"messages" mapstructure:"messages"`
	Commands CommandsConfig    `json:"commands" yaml:"commands" mapstructure:"commands"`
	Hooks    HooksConfig       `json:"hooks" yaml:"hooks" mapstructure:"hooks"`
	Channels ChannelsConfig    `json:"channels" yaml:"channels" mapstructure:"channels"`
	Gateway  GatewayConfig     `json:"gateway" yaml:"gateway" mapstructure:"gateway"`
	Skills   SkillsConfig      `json:"skills" yaml:"skills" mapstructure:"skills"`
	Plugins  PluginsConfig     `json:"plugins" yaml:"plugins" mapstructure:"plugins"`
	Logging  LoggingConfig     `json:"logging" yaml:"logging" mapstructure:"logging"`
}

type MetaConfig struct {
	LastTouchedVersion string `json:"lastTouchedVersion" yaml:"lastTouchedVersion" mapstructure:"lastTouchedVersion"`
	LastTouchedAt      string `json:"lastTouchedAt" yaml:"lastTouchedAt" mapstructure:"lastTouchedAt"`
}

type WizardConfig struct {
	LastRunAt      string `json:"lastRunAt" yaml:"lastRunAt" mapstructure:"lastRunAt"`
	LastRunVersion string `json:"lastRunVersion" yaml:"lastRunVersion" mapstructure:"lastRunVersion"`
	LastRunCommand string `json:"lastRunCommand" yaml:"lastRunCommand" mapstructure:"lastRunCommand"`
	LastRunMode    string `json:"lastRunMode" yaml:"lastRunMode" mapstructure:"lastRunMode"`
	Mock           bool   `json:"mock" yaml:"mock" mapstructure:"mock"`
}

type AuthConfig struct {
	Profiles map[string]AuthProfile `json:"profiles" yaml:"profiles" mapstructure:"profiles"`
}

type AuthProfile struct {
	Provider string `json:"provider" yaml:"provider" mapstructure:"provider"`
	Mode     string `json:"mode" yaml:"mode" mapstructure:"mode"`
}

type ModelsConfig struct {
	Mode      string                   `json:"mode" yaml:"mode" mapstructure:"mode"`
	Providers map[string]ModelProvider `json:"providers" yaml:"providers" mapstructure:"providers"`
}

type ModelProvider struct {
	BaseURL string       `json:"baseUrl" yaml:"baseUrl" mapstructure:"baseUrl"`
	API     string       `json:"api" yaml:"api" mapstructure:"api"`
	Models  []ModelEntry `json:"models" yaml:"models" mapstructure:"models"`
}

type ModelEntry struct {
	ID            string    `json:"id" yaml:"id" mapstructure:"id"`
	Name          string    `json:"name" yaml:"name" mapstructure:"name"`
	Reasoning     bool      `json:"reasoning" yaml:"reasoning" mapstructure:"reasoning"`
	Input         []string  `json:"input" yaml:"input" mapstructure:"input"`
	Cost          ModelCost `json:"cost" yaml:"cost" mapstructure:"cost"`
	ContextWindow int       `json:"contextWindow" yaml:"contextWindow" mapstructure:"contextWindow"`
	MaxTokens     int       `json:"maxTokens" yaml:"maxTokens" mapstructure:"maxTokens"`
}

type ModelCost struct {
	Input      float64 `json:"input" yaml:"input" mapstructure:"input"`
	Output     float64 `json:"output" yaml:"output" mapstructure:"output"`
	CacheRead  float64 `json:"cacheRead" yaml:"cacheRead" mapstructure:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite" yaml:"cacheWrite" mapstructure:"cacheWrite"`
}

type AgentsConfig struct {
	Defaults AgentDefaults `json:"defaults" yaml:"defaults" mapstructure:"defaults"`
}

type AgentDefaults struct {
	Model         AgentModelConfig         `json:"model" yaml:"model" mapstructure:"model"`
	Models        map[string]AgentModelMap `json:"models" yaml:"models" mapstructure:"models"`
	Workspace     string                   `json:"workspace" yaml:"workspace" mapstructure:"workspace"`
	Tools         policy.ToolPolicy        `json:"tools" yaml:"tools" mapstructure:"tools"`
	Compaction    CompactionConfig         `json:"compaction" yaml:"compaction" mapstructure:"compaction"`
	MaxConcurrent int                      `json:"maxConcurrent" yaml:"maxConcurrent" mapstructure:"maxConcurrent"`
	Subagents     SubagentsConfig          `json:"subagents" yaml:"subagents" mapstructure:"subagents"`
	Stream        bool                     `json:"stream" yaml:"stream" mapstructure:"stream"`
	ShowThinking  bool                     `json:"showThinking" yaml:"showThinking" mapstructure:"showThinking"`
}

type AgentModelConfig struct {
	Primary string `json:"primary" yaml:"primary" mapstructure:"primary"`
}

type AgentModelMap struct {
	Alias string `json:"alias" yaml:"alias" mapstructure:"alias"`
}

type CompactionConfig struct {
	Mode string `json:"mode" yaml:"mode" mapstructure:"mode"`
}

type SubagentsConfig struct {
	MaxConcurrent int               `json:"maxConcurrent" yaml:"maxConcurrent" mapstructure:"maxConcurrent"`
	Tools         policy.ToolPolicy `json:"tools" yaml:"tools" mapstructure:"tools"`
}

type MessagesConfig struct {
	AckReactionScope string `json:"ackReactionScope" yaml:"ackReactionScope" mapstructure:"ackReactionScope"`
}

type CommandsConfig struct {
	Native       string `json:"native" yaml:"native" mapstructure:"native"`
	NativeSkills string `json:"nativeSkills" yaml:"nativeSkills" mapstructure:"nativeSkills"`
}

type HooksConfig struct {
	Internal InternalHooks `json:"internal" yaml:"internal" mapstructure:"internal"`
}

type InternalHooks struct {
	Enabled bool                 `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Token   string               `json:"token" yaml:"token" mapstructure:"token"`
	Entries map[string]HookEntry `json:"entries" yaml:"entries" mapstructure:"entries"`
}

type HookEntry struct {
	Enabled bool `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
}

type ChannelsConfig struct {
	Telegram TelegramConfig `json:"telegram" yaml:"telegram" mapstructure:"telegram"`
	Discord  DiscordConfig  `json:"discord" yaml:"discord" mapstructure:"discord"`
	IMessage IMessageConfig `json:"imessage" yaml:"imessage" mapstructure:"imessage"`
	QQ       QQConfig       `json:"qq" yaml:"qq" mapstructure:"qq"`
	Feishu   FeishuConfig   `json:"feishu" yaml:"feishu" mapstructure:"feishu"`
	DingTalk DingTalkConfig `json:"dingtalk" yaml:"dingtalk" mapstructure:"dingtalk"`
	WeCom    WeComConfig    `json:"wecom" yaml:"wecom" mapstructure:"wecom"`
}

type IMessageConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	DBPath  string `json:"dbPath" yaml:"dbPath" mapstructure:"dbPath"`
}

type QQConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	AppID     uint64 `json:"appId" yaml:"appId" mapstructure:"appId"`
	AppSecret string `json:"appSecret" yaml:"appSecret" mapstructure:"appSecret"`
	Sandbox   bool   `json:"sandbox" yaml:"sandbox" mapstructure:"sandbox"`
}

type FeishuConfig struct {
	Enabled           bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	AppID             string `json:"appId" yaml:"appId" mapstructure:"appId"`
	AppSecret         string `json:"appSecret" yaml:"appSecret" mapstructure:"appSecret"`
	EncryptKey        string `json:"encryptKey" yaml:"encryptKey" mapstructure:"encryptKey"`
	VerificationToken string `json:"verificationToken" yaml:"verificationToken" mapstructure:"verificationToken"`
}

type DingTalkConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	AppKey    string `json:"appKey" yaml:"appKey" mapstructure:"appKey"`
	AppSecret string `json:"appSecret" yaml:"appSecret" mapstructure:"appSecret"`
}

type WeComConfig struct {
	Enabled        bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Token          string `json:"token" yaml:"token" mapstructure:"token"`
	EncodingAESKey string `json:"encodingAesKey" yaml:"encodingAesKey" mapstructure:"encodingAesKey"`
	Port           int    `json:"port" yaml:"port" mapstructure:"port"`
	BotID          string `json:"botId" yaml:"botId" mapstructure:"botId"`
}

type TelegramConfig struct {
	Enabled     bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	DMPolicy    string `json:"dmPolicy" yaml:"dmPolicy" mapstructure:"dmPolicy"`
	BotToken    string `json:"botToken" yaml:"botToken" mapstructure:"botToken"`
	GroupPolicy string `json:"groupPolicy" yaml:"groupPolicy" mapstructure:"groupPolicy"`
	StreamMode  string `json:"streamMode" yaml:"streamMode" mapstructure:"streamMode"`
}

type DiscordConfig struct {
	Enabled     bool                          `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Token       string                        `json:"token" yaml:"token" mapstructure:"token"`
	Intents     DiscordIntents                `json:"intents" yaml:"intents" mapstructure:"intents"`
	DMPolicy    string                        `json:"dmPolicy" yaml:"dmPolicy" mapstructure:"dmPolicy"`
	GroupPolicy string                        `json:"groupPolicy" yaml:"groupPolicy" mapstructure:"groupPolicy"`
	Guilds      map[string]DiscordGuildConfig `json:"guilds" yaml:"guilds" mapstructure:"guilds"`
}

type DiscordIntents struct {
	Presence     bool `json:"presence" yaml:"presence" mapstructure:"presence"`
	GuildMembers bool `json:"guildMembers" yaml:"guildMembers" mapstructure:"guildMembers"`
}

type DiscordGuildConfig struct {
	Slug           string                          `json:"slug" yaml:"slug" mapstructure:"slug"`
	RequireMention bool                            `json:"requireMention" yaml:"requireMention" mapstructure:"requireMention"`
	Channels       map[string]DiscordChannelConfig `json:"channels" yaml:"channels" mapstructure:"channels"`
}

type DiscordChannelConfig struct {
	Allow          bool `json:"allow" yaml:"allow" mapstructure:"allow"`
	RequireMention bool `json:"requireMention" yaml:"requireMention" mapstructure:"requireMention"`
	Enabled        bool `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
}

type GatewayConfig struct {
	Port      int             `json:"port" yaml:"port" mapstructure:"port"`
	Mode      string          `json:"mode" yaml:"mode" mapstructure:"mode"`
	Bind      string          `json:"bind" yaml:"bind" mapstructure:"bind"`
	Auth      GatewayAuth     `json:"auth" yaml:"auth" mapstructure:"auth"`
	RateLimit RateLimitConfig `json:"rateLimit" yaml:"rateLimit" mapstructure:"rateLimit"`
	Tailscale TailscaleConfig `json:"tailscale" yaml:"tailscale" mapstructure:"tailscale"`
}

type RateLimitConfig struct {
	Enabled bool    `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	RPS     float64 `json:"rps" yaml:"rps" mapstructure:"rps"`
	Burst   int     `json:"burst" yaml:"burst" mapstructure:"burst"`
}

type GatewayAuth struct {
	Mode  string `json:"mode" yaml:"mode" mapstructure:"mode"`
	Token string `json:"token" yaml:"token" mapstructure:"token"`
}

type TailscaleConfig struct {
	Mode        string `json:"mode" yaml:"mode" mapstructure:"mode"`
	ResetOnExit bool   `json:"resetOnExit" yaml:"resetOnExit" mapstructure:"resetOnExit"`
}

type SkillsConfig struct {
	Install SkillsInstallConfig `json:"install" yaml:"install" mapstructure:"install"`
}

type SkillsInstallConfig struct {
	NodeManager string `json:"nodeManager" yaml:"nodeManager" mapstructure:"nodeManager"`
}

type PluginsConfig struct {
	Entries map[string]PluginEntry `json:"entries" yaml:"entries" mapstructure:"entries"`
}

type PluginEntry struct {
	Enabled bool `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
}

type LoggingConfig struct {
	PrintSystemPrompt bool `json:"printSystemPrompt" yaml:"printSystemPrompt" mapstructure:"printSystemPrompt"`
	Verbose           bool `json:"verbose" yaml:"verbose" mapstructure:"verbose"`
}

// StateDir returns the LiteClaw state directory path.
// Can be overridden via LITECLAW_STATE_DIR environment variable.
// Default: ~/.liteclaw
func StateDir() string {
	// Check for override via environment variable
	if override := strings.TrimSpace(os.Getenv("LITECLAW_STATE_DIR")); override != "" {
		return expandPath(override)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ".liteclaw"
	}
	return filepath.Join(home, ".liteclaw")
}

// ConfigDir returns the config directory path (alias for StateDir for compatibility).
func ConfigDir() string {
	return StateDir()
}

// ConfigPath returns the default config file path.
// Can be overridden via LITECLAW_CONFIG_PATH environment variable.
// Default: ~/.liteclaw/liteclaw.json
func ConfigPath() string {
	// Check for override via environment variable
	if override := strings.TrimSpace(os.Getenv("LITECLAW_CONFIG_PATH")); override != "" {
		return expandPath(override)
	}
	return filepath.Join(StateDir(), "liteclaw.json")
}

// ExtrasPath returns the default extras config file path.
// Can be overridden via LITECLAW_EXTRAS_PATH environment variable.
// Default: ~/.liteclaw/liteclaw.extras.json
func ExtrasPath() string {
	if override := strings.TrimSpace(os.Getenv("LITECLAW_EXTRAS_PATH")); override != "" {
		return expandPath(override)
	}
	return filepath.Join(StateDir(), "liteclaw.extras.json")
}

// expandPath expands ~ to home directory and resolves the path.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = strings.Replace(path, "~", home, 1)
		}
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absPath
}

// ShellEnvExpectedKeys defines the standard environment variables used by Clawdbot/LiteClaw.
var ShellEnvExpectedKeys = []string{
	"OPENAI_API_KEY",
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_OAUTH_TOKEN",
	"GEMINI_API_KEY",
	"ZAI_API_KEY",
	"OPENROUTER_API_KEY",
	"AI_GATEWAY_API_KEY",
	"MINIMAX_API_KEY",
	"SYNTHETIC_API_KEY",
	"ELEVENLABS_API_KEY",
	"TELEGRAM_BOT_TOKEN",
	"DISCORD_BOT_TOKEN",
	"SLACK_BOT_TOKEN",
	"SLACK_APP_TOKEN",
	"LITECLAW_GATEWAY_TOKEN",
	"LITECLAW_GATEWAY_PASSWORD",
}

// LoadViper loads the configuration into a Viper instance.
func LoadViper() (*viper.Viper, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Check for explicit config path override
	if configPath := strings.TrimSpace(os.Getenv("LITECLAW_CONFIG_PATH")); configPath != "" {
		expandedPath := expandPath(configPath)
		fileInfo, err := os.Stat(expandedPath)
		if err == nil && fileInfo.IsDir() {
			// If it's a directory, add it to search path and look for liteclaw.json
			v.SetConfigName("liteclaw")
			v.AddConfigPath(expandedPath)
		} else {
			// If it's a file (or doesn't exist yet/unknown), assume it's the full file path
			v.SetConfigFile(expandedPath)
		}
	} else {
		// Config file settings for LiteClaw
		// Primary: liteclaw.json
		v.SetConfigName("liteclaw")
		v.AddConfigPath(StateDir()) // ~/.liteclaw/
	}

	// Env vars - use LITECLAW_ prefix
	v.SetEnvPrefix("LITECLAW")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file
	err := v.ReadInConfig()
	if err != nil {
		// Fallback: try liteclaw.json/yaml for backward compatibility
		v.SetConfigName("liteclaw")
		if err2 := v.ReadInConfig(); err2 != nil {
			// Fallback: try config.yaml
			v.SetConfigName("config")
			if err3 := v.ReadInConfig(); err3 != nil {
				if _, ok := err3.(viper.ConfigFileNotFoundError); ok {
					return nil, ErrConfigNotFound
				}
				return nil, err3
			}
		}
	}

	// Merge optional extras config if present.
	if extrasPath := ExtrasPath(); extrasPath != "" {
		if _, err := os.Stat(extrasPath); err == nil {
			v.SetConfigFile(extrasPath)
			_ = v.MergeInConfig()
		}
	}

	return v, nil
}

// Load reads the configuration from file or environment variables.
func Load() (*Config, error) {
	v, err := LoadViper()
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// COMPATIBILITY: Inject config.env block into the OS environment FIRST.
	// This happens before 'expandEnvVars' so that Expansion works correctly.
	// E.g. if config has {"env": {"KEY": "VAL"}} and {"someField": "${KEY}"},
	// we must Setenv("KEY", "VAL") before ExpandEnv("${KEY}").
	for k, v := range cfg.Env {
		// We expand the value first (in case it refers to existing shell envs), then set it.
		// e.g. "PATH": "${PATH}:/new/bin"
		expandedVal := os.ExpandEnv(v)
		_ = os.Setenv(k, expandedVal)
		// Update the map with the expanded value for consistency
		cfg.Env[k] = expandedVal
	}

	// Expand environment variables in sensitive fields
	expandEnvVars(&cfg)

	return &cfg, nil
}

// setDefaults sets default configuration values.
func setDefaults(v *viper.Viper) {
	// Gateway defaults
	v.SetDefault("gateway.bind", "loopback")
	v.SetDefault("gateway.port", 18789)
	v.SetDefault("gateway.mode", "local")
	v.SetDefault("gateway.auth.mode", "token")

	// Agent defaults
	v.SetDefault("agents.defaults.model.primary", "minimax/MiniMax-M2.1")
	v.SetDefault("agents.defaults.maxConcurrent", 4)
	v.SetDefault("agents.defaults.subagents.maxConcurrent", 8)

	// Skills defaults
	v.SetDefault("skills.install.nodeManager", "npm")
}

// expandEnvVars expands environment variables in the config.
func expandEnvVars(cfg *Config) {
	// Expand env vars in Gateway auth token
	cfg.Gateway.Auth.Token = os.ExpandEnv(cfg.Gateway.Auth.Token)

	// Expand env vars in Channels
	cfg.Channels.Telegram.BotToken = os.ExpandEnv(cfg.Channels.Telegram.BotToken)
	cfg.Channels.Discord.Token = os.ExpandEnv(cfg.Channels.Discord.Token)

	// Expand env vars in Hooks
	cfg.Hooks.Internal.Token = os.ExpandEnv(cfg.Hooks.Internal.Token)
}

// Save saves the configuration to the config file.
// Uses ConfigPath() for consistency with Load() - defaults to ~/.liteclaw/liteclaw.json
// Only JSON format is supported.
func Save(cfg *Config) error {
	// Use the same path that Load() uses
	configPath := ConfigPath()

	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	// Always use JSON format
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

// Validate checks for semantic errors in the config.
func (c *Config) Validate() error {
	primaryStr := c.Agents.Defaults.Model.Primary
	if primaryStr == "" {
		return fmt.Errorf("agents.defaults.model.primary is required")
	}

	parts := strings.SplitN(primaryStr, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid model format '%s'. Expected 'provider/model'", primaryStr)
	}

	providerName := parts[0]
	if _, ok := c.Models.Providers[providerName]; !ok {
		return fmt.Errorf("provider '%s' is not defined in 'models.providers'", providerName)
	}

	return nil
}
