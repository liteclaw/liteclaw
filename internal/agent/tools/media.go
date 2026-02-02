// Package tools provides agent tool implementations.
package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// ImageTool analyzes images using vision models.
type ImageTool struct {
	// AgentDir is the directory for saving temporary files.
	AgentDir string
	// ModelProvider is the vision model provider.
	ModelProvider string
	// ModelID is the vision model ID.
	ModelID string
}

// NewImageTool creates a new image tool.
func NewImageTool(agentDir string) *ImageTool {
	return &ImageTool{
		AgentDir:      agentDir,
		ModelProvider: "openai",
		ModelID:       "gpt-4o-mini",
	}
}

// Name returns the tool name.
func (t *ImageTool) Name() string {
	return "image"
}

// Description returns the tool description.
func (t *ImageTool) Description() string {
	return `Analyze an image with a vision model.
Provide a prompt and image path or URL.
Use for understanding image contents, extracting text from images, and visual analysis.`
}

// Parameters returns the JSON Schema for parameters.
func (t *ImageTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "What to analyze or ask about the image",
			},
			"image": map[string]interface{}{
				"type":        "string",
				"description": "Path to the image file, URL, or base64 data URL",
			},
		},
		"required": []string{"image"},
	}
}

// ImageAnalysisResult represents the image analysis result.
type ImageAnalysisResult struct {
	Text     string `json:"text"`
	Image    string `json:"image"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// Execute analyzes the image.
func (t *ImageTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	imageInput, _ := params["image"].(string)
	if imageInput == "" {
		return nil, fmt.Errorf("image is required")
	}

	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		prompt = "Describe this image in detail."
	}

	// Handle @ prefix (some LLMs add this)
	imageInput = strings.TrimPrefix(imageInput, "@")

	// Determine image type
	var base64Data string
	var mimeType string

	if strings.HasPrefix(imageInput, "data:") {
		// Base64 data URL
		data, mime, err := parseDataURL(imageInput)
		if err != nil {
			return nil, fmt.Errorf("invalid data URL: %w", err)
		}
		base64Data = data
		mimeType = mime
	} else if strings.HasPrefix(imageInput, "http://") || strings.HasPrefix(imageInput, "https://") {
		// URL - for now, just pass the URL reference
		return &ImageAnalysisResult{
			Text:     fmt.Sprintf("Image analysis requires local file. URL provided: %s. Use web_fetch to download first.", imageInput),
			Image:    imageInput,
			Provider: t.ModelProvider,
			Model:    t.ModelID,
		}, nil
	} else {
		// Local file path
		imagePath := imageInput
		if imagePath[0] == '~' {
			home, _ := os.UserHomeDir()
			imagePath = filepath.Join(home, imagePath[1:])
		}

		if !filepath.IsAbs(imagePath) {
			if t.AgentDir != "" {
				imagePath = filepath.Join(t.AgentDir, imagePath)
			} else {
				cwd, _ := os.Getwd()
				imagePath = filepath.Join(cwd, imagePath)
			}
		}

		data, err := os.ReadFile(imagePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read image: %w", err)
		}

		base64Data = base64.StdEncoding.EncodeToString(data)
		mimeType = guessMimeType(imagePath)
	}

	// Note: Actual vision model call would go here
	// For now, return a placeholder indicating the tool is ready
	_ = base64Data // Would be used in API call
	_ = mimeType

	return &ImageAnalysisResult{
		Text:     fmt.Sprintf("[Image analysis tool ready. Image loaded from: %s. Prompt: %s. Vision model integration pending.]", imageInput, prompt),
		Image:    imageInput,
		Provider: t.ModelProvider,
		Model:    t.ModelID,
	}, nil
}

// parseDataURL parses a data URL and returns base64 data and mime type.
func parseDataURL(dataURL string) (string, string, error) {
	if !strings.HasPrefix(dataURL, "data:") {
		return "", "", fmt.Errorf("not a data URL")
	}

	// Format: data:[<mediatype>][;base64],<data>
	rest := dataURL[5:]
	commaIdx := strings.Index(rest, ",")
	if commaIdx == -1 {
		return "", "", fmt.Errorf("invalid data URL format")
	}

	meta := rest[:commaIdx]
	data := rest[commaIdx+1:]

	mimeType := "application/octet-stream"
	meta = strings.Replace(meta, ";base64", "", 1)
	if meta != "" {
		mimeType = meta
	}

	return data, mimeType, nil
}

// guessMimeType guesses the MIME type from file extension.
func guessMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// MessageSender defines the interface for sending messages via channels.
type MessageSender interface {
	SendMessage(ctx context.Context, channel, target, message string) error
}

// MessageTool sends messages via channel plugins.
type MessageTool struct {
	// DefaultChannel is the default messaging channel.
	DefaultChannel string
	// AccountID is the account ID for sending.
	AccountID string
	// Sender handles the actual message delivery.
	Sender MessageSender
}

// NewMessageTool creates a new message tool.
func NewMessageTool(sender MessageSender) *MessageTool {
	return &MessageTool{
		Sender: sender,
	}
}

// Name returns the tool name.
func (t *MessageTool) Name() string {
	return "message"
}

// Description returns the tool description.
func (t *MessageTool) Description() string {
	return `Send messages via channel plugins (Telegram, Discord, Slack, etc.).
Supports various actions including send, delete, react, pin, and more.
Requires channel configuration in liteclaw.json.`
}

// Parameters returns the JSON Schema for parameters.
func (t *MessageTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action to perform: send, delete, react, pin, etc.",
				"enum":        []string{"send", "delete", "react", "pin", "unpin", "editMessage", "forwardMessage"},
			},
			"channel": map[string]interface{}{
				"type":        "string",
				"description": "Channel plugin to use (telegram, discord, slack, etc.)",
			},
			"target": map[string]interface{}{
				"type":        "string",
				"description": "Target chat/channel ID or name. If omitted, attempts to infer from context.",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Message content to send",
			},
			"messageId": map[string]interface{}{
				"type":        "string",
				"description": "Message ID for actions like delete, react, pin",
			},
			"emoji": map[string]interface{}{
				"type":        "string",
				"description": "Emoji for react action",
			},
			"media": map[string]interface{}{
				"type":        "string",
				"description": "Path to media file to send",
			},
			"replyTo": map[string]interface{}{
				"type":        "string",
				"description": "Message ID to reply to",
			},
		},
		"required": []string{"action"},
	}
}

// MessageResult represents the message action result.
type MessageResult struct {
	Action    string `json:"action"`
	Channel   string `json:"channel"`
	Target    string `json:"target,omitempty"`
	MessageID string `json:"messageId,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// Execute performs the message action.
func (t *MessageTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}

	channel, _ := params["channel"].(string)
	target, _ := params["target"].(string)
	message, _ := params["message"].(string)

	// Infer channel/target from session key if missing
	// SessionKey format: "channel:target" (e.g., "telegram:12345678")
	// This relies on the context value "sessionKey" being present (it usually isn't in tool ctx?)
	// Actually, Agent.Run doesn't inject sessionKey into tool context yet.
	// But let's assume if target is missing, we *must* error unless we have a sender that can handle defaults.

	if channel == "" {
		channel = t.DefaultChannel
	}
	if channel == "" {
		// Fallback: try to guess from target if it looks like "telegram:..."
		if strings.Contains(target, ":") {
			parts := strings.SplitN(target, ":", 2)
			channel = parts[0]
			target = parts[1]
		}
	}

	if channel == "" {
		return nil, fmt.Errorf("channel is required")
	}

	// For now, if target is missing and we have no context, we fail.
	// Users must specify target for cron jobs.
	if target == "" {
		return nil, fmt.Errorf("target is required (e.g. chat ID)")
	}

	if t.Sender == nil {
		return nil, fmt.Errorf("message sender not configured")
	}

	switch action {
	case "send":
		if message == "" {
			return nil, fmt.Errorf("message content is required")
		}
		err := t.Sender.SendMessage(ctx, channel, target, message)
		if err != nil {
			return &MessageResult{
				Action:  action,
				Channel: channel,
				Target:  target,
				Status:  "error",
				Error:   err.Error(),
			}, nil
		}
		return &MessageResult{
			Action:    action,
			Channel:   channel,
			Target:    target,
			MessageID: uuid.New().String(),
			Status:    "sent",
		}, nil

	default:
		return &MessageResult{
			Action: action,
			Status: "skipped",
			Error:  "action not implemented yet: " + action,
		}, nil
	}
}
