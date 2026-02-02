// Package channels provides the communication channel framework.
package channels

import (
	"fmt"

	"github.com/rs/zerolog"
)

// ChannelConfig holds configuration for all channels.
type ChannelConfig struct {
	Telegram   *TelegramConfig   `json:"telegram,omitempty" yaml:"telegram,omitempty"`
	Discord    *DiscordConfig    `json:"discord,omitempty" yaml:"discord,omitempty"`
	Slack      *SlackConfig      `json:"slack,omitempty" yaml:"slack,omitempty"`
	WhatsApp   *WhatsAppConfig   `json:"whatsapp,omitempty" yaml:"whatsapp,omitempty"`
	Matrix     *MatrixConfig     `json:"matrix,omitempty" yaml:"matrix,omitempty"`
	Signal     *SignalConfig     `json:"signal,omitempty" yaml:"signal,omitempty"`
	MSTeams    *MSTeamsConfig    `json:"msteams,omitempty" yaml:"msteams,omitempty"`
	GoogleChat *GoogleChatConfig `json:"googlechat,omitempty" yaml:"googlechat,omitempty"`
	Line       *LineConfig       `json:"line,omitempty" yaml:"line,omitempty"`
	IMessage   *IMessageConfig   `json:"imessage,omitempty" yaml:"imessage,omitempty"`
	VoiceCall  *VoiceCallConfig  `json:"voiceCall,omitempty" yaml:"voiceCall,omitempty"`
	Defaults   *DefaultsConfig   `json:"defaults,omitempty" yaml:"defaults,omitempty"`
}

// TelegramConfig holds Telegram-specific configuration.
type TelegramConfig struct {
	Enabled     bool                              `json:"enabled" yaml:"enabled"`
	BotToken    string                            `json:"botToken,omitempty" yaml:"botToken,omitempty"`
	TokenFile   string                            `json:"tokenFile,omitempty" yaml:"tokenFile,omitempty"`
	WebhookURL  string                            `json:"webhookUrl,omitempty" yaml:"webhookUrl,omitempty"`
	Proxy       string                            `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	AllowFrom   []string                          `json:"allowFrom,omitempty" yaml:"allowFrom,omitempty"`
	DMPolicy    string                            `json:"dmPolicy,omitempty" yaml:"dmPolicy,omitempty"`       // "open", "pairing", "allowlist"
	GroupPolicy string                            `json:"groupPolicy,omitempty" yaml:"groupPolicy,omitempty"` // "open", "allowlist"
	Accounts    map[string]*TelegramAccountConfig `json:"accounts,omitempty" yaml:"accounts,omitempty"`
}

// TelegramAccountConfig holds per-account Telegram configuration.
type TelegramAccountConfig struct {
	Enabled   bool     `json:"enabled" yaml:"enabled"`
	Name      string   `json:"name,omitempty" yaml:"name,omitempty"`
	BotToken  string   `json:"botToken,omitempty" yaml:"botToken,omitempty"`
	TokenFile string   `json:"tokenFile,omitempty" yaml:"tokenFile,omitempty"`
	AllowFrom []string `json:"allowFrom,omitempty" yaml:"allowFrom,omitempty"`
}

// DiscordConfig holds Discord-specific configuration.
type DiscordConfig struct {
	Enabled   bool     `json:"enabled" yaml:"enabled"`
	BotToken  string   `json:"botToken,omitempty" yaml:"botToken,omitempty"`
	TokenFile string   `json:"tokenFile,omitempty" yaml:"tokenFile,omitempty"`
	GuildID   string   `json:"guildId,omitempty" yaml:"guildId,omitempty"`
	AllowFrom []string `json:"allowFrom,omitempty" yaml:"allowFrom,omitempty"`
}

// SlackConfig holds Slack-specific configuration.
type SlackConfig struct {
	Enabled       bool   `json:"enabled" yaml:"enabled"`
	BotToken      string `json:"botToken,omitempty" yaml:"botToken,omitempty"`
	AppToken      string `json:"appToken,omitempty" yaml:"appToken,omitempty"`
	SigningSecret string `json:"signingSecret,omitempty" yaml:"signingSecret,omitempty"`
}

// WhatsAppConfig holds WhatsApp-specific configuration.
type WhatsAppConfig struct {
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	PhoneNumber string `json:"phoneNumber,omitempty" yaml:"phoneNumber,omitempty"`
	SessionPath string `json:"sessionPath,omitempty" yaml:"sessionPath,omitempty"`
}

// MatrixConfig holds Matrix-specific configuration.
type MatrixConfig struct {
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	Homeserver  string `json:"homeserver,omitempty" yaml:"homeserver,omitempty"`
	UserID      string `json:"userId,omitempty" yaml:"userId,omitempty"`
	AccessToken string `json:"accessToken,omitempty" yaml:"accessToken,omitempty"`
}

// SignalConfig holds Signal-specific configuration.
type SignalConfig struct {
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	PhoneNumber string `json:"phoneNumber,omitempty" yaml:"phoneNumber,omitempty"`
}

// MSTeamsConfig holds Microsoft Teams-specific configuration.
type MSTeamsConfig struct {
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	AppID       string `json:"appId,omitempty" yaml:"appId,omitempty"`
	AppPassword string `json:"appPassword,omitempty" yaml:"appPassword,omitempty"`
}

// GoogleChatConfig holds Google Chat-specific configuration.
type GoogleChatConfig struct {
	Enabled           bool   `json:"enabled" yaml:"enabled"`
	ServiceAccountKey string `json:"serviceAccountKey,omitempty" yaml:"serviceAccountKey,omitempty"`
}

// LineConfig holds LINE-specific configuration.
type LineConfig struct {
	Enabled       bool   `json:"enabled" yaml:"enabled"`
	ChannelSecret string `json:"channelSecret,omitempty" yaml:"channelSecret,omitempty"`
	ChannelToken  string `json:"channelToken,omitempty" yaml:"channelToken,omitempty"`
}

// IMessageConfig holds iMessage-specific configuration (macOS only).
type IMessageConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// VoiceCallConfig holds voice call configuration.
type VoiceCallConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	Provider   string `json:"provider,omitempty" yaml:"provider,omitempty"` // "twilio", "telnyx", "plivo"
	FromNumber string `json:"fromNumber,omitempty" yaml:"fromNumber,omitempty"`
}

// DefaultsConfig holds default settings for all channels.
type DefaultsConfig struct {
	DMPolicy    string `json:"dmPolicy,omitempty" yaml:"dmPolicy,omitempty"`
	GroupPolicy string `json:"groupPolicy,omitempty" yaml:"groupPolicy,omitempty"`
}

// Factory creates channel adapters from configuration.
type Factory struct {
	logger *zerolog.Logger
}

// NewFactory creates a new channel factory.
func NewFactory(logger *zerolog.Logger) *Factory {
	return &Factory{logger: logger}
}

// CreateAll creates all enabled adapters from configuration.
func (f *Factory) CreateAll(cfg *ChannelConfig) ([]Adapter, error) {
	var adapters []Adapter

	// Telegram
	if cfg.Telegram != nil && cfg.Telegram.Enabled {
		// This would import from internal/channels/telegram
		// For now, just log - actual creation happens in each adapter package
		f.logger.Info().Msg("Telegram channel enabled in config")
	}

	// Discord
	if cfg.Discord != nil && cfg.Discord.Enabled {
		f.logger.Info().Msg("Discord channel enabled in config")
	}

	// Slack
	if cfg.Slack != nil && cfg.Slack.Enabled {
		f.logger.Info().Msg("Slack channel enabled in config")
	}

	// WhatsApp
	if cfg.WhatsApp != nil && cfg.WhatsApp.Enabled {
		f.logger.Info().Msg("WhatsApp channel enabled in config")
	}

	// Matrix
	if cfg.Matrix != nil && cfg.Matrix.Enabled {
		f.logger.Info().Msg("Matrix channel enabled in config")
	}

	return adapters, nil
}

// CreateTelegram creates a Telegram adapter placeholder.
// The actual implementation is in internal/channels/telegram.
func (f *Factory) CreateTelegram(cfg *TelegramConfig) (Adapter, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("telegram is disabled")
	}
	// Import and create from telegram package
	return nil, fmt.Errorf("telegram adapter not linked - import internal/channels/telegram")
}

// CreateDiscord creates a Discord adapter placeholder.
// The actual implementation is in internal/channels/discord.
func (f *Factory) CreateDiscord(cfg *DiscordConfig) (Adapter, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("discord is disabled")
	}
	// Import and create from discord package
	return nil, fmt.Errorf("discord adapter not linked - import internal/channels/discord")
}
