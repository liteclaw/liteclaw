// Package channels provides the communication channel framework for LiteClaw.
// Channels act as adapters/translators between external messaging platforms
// and the LiteClaw Gateway, providing a unified interface for message handling.
package channels

import (
	"context"
	"time"
)

// ChannelType represents supported channel types.
type ChannelType string

const (
	ChannelTypeTelegram   ChannelType = "telegram"
	ChannelTypeDiscord    ChannelType = "discord"
	ChannelTypeSlack      ChannelType = "slack"
	ChannelTypeWhatsApp   ChannelType = "whatsapp"
	ChannelTypeIMessage   ChannelType = "imessage"
	ChannelTypeQQ         ChannelType = "qq"
	ChannelTypeFeishu     ChannelType = "feishu"
	ChannelTypeDingTalk   ChannelType = "dingtalk"
	ChannelTypeWeCom      ChannelType = "wecom"
	ChannelTypeLine       ChannelType = "line"
	ChannelTypeSignal     ChannelType = "signal"
	ChannelTypeMatrix     ChannelType = "matrix"
	ChannelTypeMSTeams    ChannelType = "msteams"
	ChannelTypeMattermost ChannelType = "mattermost"
	ChannelTypeGoogleChat ChannelType = "googlechat"
	ChannelTypeTwitch     ChannelType = "twitch"
	ChannelTypeNostr      ChannelType = "nostr"
	ChannelTypeWeb        ChannelType = "web"
	ChannelTypeVoiceCall  ChannelType = "voice-call"
)

// ChatType represents the type of chat.
type ChatType string

const (
	ChatTypeDirect  ChatType = "direct"
	ChatTypeGroup   ChatType = "group"
	ChatTypeChannel ChatType = "channel"
	ChatTypeThread  ChatType = "thread"
)

// MessageType represents the type of message.
type MessageType string

const (
	MessageTypeText     MessageType = "text"
	MessageTypeImage    MessageType = "image"
	MessageTypeAudio    MessageType = "audio"
	MessageTypeVideo    MessageType = "video"
	MessageTypeFile     MessageType = "file"
	MessageTypeSticker  MessageType = "sticker"
	MessageTypeReaction MessageType = "reaction"
	MessageTypeCommand  MessageType = "command"
)

// Capabilities describes what a channel supports.
type Capabilities struct {
	ChatTypes      []ChatType `json:"chatTypes"`
	Reactions      bool       `json:"reactions"`
	Threads        bool       `json:"threads"`
	Media          bool       `json:"media"`
	Stickers       bool       `json:"stickers"`
	Voice          bool       `json:"voice"`
	NativeCommands bool       `json:"nativeCommands"`
	BlockStreaming bool       `json:"blockStreaming"`
	Webhooks       bool       `json:"webhooks"`
	Polling        bool       `json:"polling"`
}

// Config holds channel configuration.
type Config struct {
	Enabled    bool              `json:"enabled" yaml:"enabled"`
	Token      string            `json:"token,omitempty" yaml:"token,omitempty"`
	TokenFile  string            `json:"tokenFile,omitempty" yaml:"tokenFile,omitempty"`
	APIKey     string            `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	WebhookURL string            `json:"webhookUrl,omitempty" yaml:"webhookUrl,omitempty"`
	Proxy      string            `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	Options    map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
}

// Account represents a channel account (for multi-account support).
type Account struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Enabled   bool   `json:"enabled"`
	Config    Config `json:"config"`
	IsDefault bool   `json:"isDefault,omitempty"`
}

// Sender represents the sender of a message.
type Sender struct {
	ID          string `json:"id"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	IsBot       bool   `json:"isBot,omitempty"`
}

// Chat represents a chat/conversation.
type Chat struct {
	ID       string   `json:"id"`
	Type     ChatType `json:"type"`
	Title    string   `json:"title,omitempty"`
	ThreadID string   `json:"threadId,omitempty"`
}

// RuntimeState holds runtime state of a channel.
type RuntimeState struct {
	Running        bool       `json:"running"`
	Mode           string     `json:"mode,omitempty"` // "polling", "webhook", "websocket"
	LastStartAt    *time.Time `json:"lastStartAt,omitempty"`
	LastStopAt     *time.Time `json:"lastStopAt,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	LastInboundAt  *time.Time `json:"lastInboundAt,omitempty"`
	LastOutboundAt *time.Time `json:"lastOutboundAt,omitempty"`
	MessageCount   int64      `json:"messageCount"`
}

// ProbeResult holds the result of probing a channel.
type ProbeResult struct {
	OK        bool   `json:"ok"`
	BotID     string `json:"botId,omitempty"`
	BotName   string `json:"botName,omitempty"`
	Username  string `json:"username,omitempty"`
	Error     string `json:"error,omitempty"`
	LatencyMs int64  `json:"latencyMs,omitempty"`
}

// SendResult holds the result of sending a message.
type SendResult struct {
	MessageID string `json:"messageId"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// Event represents a channel event.
type Event struct {
	Type      string                 `json:"type"` // "message", "reaction", "typing", "presence"
	Channel   ChannelType            `json:"channel"`
	ChatID    string                 `json:"chatId"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// EventHandler handles channel events.
type EventHandler interface {
	OnEvent(ctx context.Context, event *Event) error
}
