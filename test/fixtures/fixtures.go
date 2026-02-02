// Package fixtures provides test fixtures for LiteClaw tests.
package fixtures

// SampleConfig is a sample configuration for testing.
const SampleConfig = `
gateway:
  host: "127.0.0.1"
  port: 3456
  auth:
    enabled: false

agents:
  default:
    model: "mock/test-model"
    maxTurns: 10
    temperature: 0.7

telegram:
  enabled: false
  token: "test-token"

discord:
  enabled: false
  token: "test-token"

llm:
  providers:
    mock:
      enabled: true

logging:
  level: "debug"
  format: "console"
`

// SampleSkill is a sample SKILL.md content for testing.
const SampleSkill = `---
name: test-skill
description: A test skill for testing purposes
homepage: https://example.com
metadata: {"clawdbot":{"emoji":"🧪"}}
---

# Test Skill

This is a test skill.

## Usage

Just use it!
`

// SampleExtensionManifest is a sample extension manifest.
const SampleExtensionManifest = `
id: test-extension
name: Test Extension
description: A test extension
version: 1.0.0
author: Test Author

config:
  properties:
    enabled:
      type: boolean
      default: true
`

// LLMResponses provides sample LLM responses for testing.
var LLMResponses = struct {
	SimpleText     string
	WithToolCall   string
	StreamingParts []string
}{
	SimpleText:   "This is a simple text response from the LLM.",
	WithToolCall: `{"tool_calls": [{"id": "call_1", "name": "exec", "arguments": {"command": "ls -la"}}]}`,
	StreamingParts: []string{
		"This ",
		"is ",
		"a ",
		"streaming ",
		"response.",
	},
}

// IncomingMessages provides sample incoming messages.
var IncomingMessages = struct {
	SimpleText   map[string]interface{}
	WithMedia    map[string]interface{}
	FromTelegram map[string]interface{}
}{
	SimpleText: map[string]interface{}{
		"id":          "msg_123",
		"channelType": "test",
		"chatId":      "chat_456",
		"senderId":    "user_789",
		"senderName":  "Test User",
		"text":        "Hello, bot!",
		"timestamp":   1609459200,
	},
	WithMedia: map[string]interface{}{
		"id":          "msg_124",
		"channelType": "test",
		"chatId":      "chat_456",
		"senderId":    "user_789",
		"senderName":  "Test User",
		"text":        "Check out this image",
		"timestamp":   1609459201,
		"attachments": []map[string]interface{}{
			{
				"type":     "image",
				"url":      "https://example.com/image.jpg",
				"mimeType": "image/jpeg",
			},
		},
	},
	FromTelegram: map[string]interface{}{
		"id":          "12345",
		"channelType": "telegram",
		"chatId":      "-1001234567890",
		"senderId":    "987654321",
		"senderName":  "Telegram User",
		"text":        "/start",
		"timestamp":   1609459202,
	},
}
