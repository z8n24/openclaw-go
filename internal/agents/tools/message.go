package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// MessageTool 跨渠道消息发送工具
type MessageTool struct {
	// 发送函数回调 (由 Gateway 注入)
	SendFunc func(ctx context.Context, channel, target, message string, opts MessageOptions) error
}

// MessageOptions 消息选项
type MessageOptions struct {
	ReplyTo   string `json:"replyTo,omitempty"`
	Silent    bool   `json:"silent,omitempty"`
	AsVoice   bool   `json:"asVoice,omitempty"`
	FilePath  string `json:"filePath,omitempty"`
	Buffer    string `json:"buffer,omitempty"`    // Base64 encoded
	MimeType  string `json:"mimeType,omitempty"`
	Caption   string `json:"caption,omitempty"`
	Effect    string `json:"effect,omitempty"`    // Message effect
}

// MessageParams message 工具参数
type MessageParams struct {
	Action    string `json:"action"` // send, broadcast
	Channel   string `json:"channel,omitempty"` // telegram, discord, whatsapp, etc.
	Target    string `json:"target,omitempty"`  // Channel/user id or name
	Targets   []string `json:"targets,omitempty"` // Multiple targets for broadcast
	Message   string `json:"message,omitempty"`
	
	// 可选参数
	ReplyTo   string `json:"replyTo,omitempty"`
	Silent    bool   `json:"silent,omitempty"`
	AsVoice   bool   `json:"asVoice,omitempty"`
	FilePath  string `json:"filePath,omitempty"`
	Buffer    string `json:"buffer,omitempty"`
	MimeType  string `json:"mimeType,omitempty"`
	Caption   string `json:"caption,omitempty"`
	Effect    string `json:"effect,omitempty"`
	
	// Discord 特有
	GuildID   string `json:"guildId,omitempty"`
	ChannelID string `json:"channelId,omitempty"`
	
	// Telegram 特有
	ChatID    string `json:"chatId,omitempty"`
}

// NewMessageTool 创建 message 工具
func NewMessageTool() *MessageTool {
	return &MessageTool{}
}

func (t *MessageTool) Name() string {
	return ToolMessage
}

func (t *MessageTool) Description() string {
	return "Send, delete, and manage messages via channel plugins. Supports actions: send, broadcast."
}

func (t *MessageTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["send", "broadcast"],
				"description": "Message action"
			},
			"channel": {
				"type": "string",
				"description": "Channel name (telegram, discord, whatsapp, signal, imessage, slack)"
			},
			"target": {
				"type": "string",
				"description": "Target channel/user id or name"
			},
			"targets": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Multiple targets for broadcast"
			},
			"message": {
				"type": "string",
				"description": "Message content"
			},
			"replyTo": {
				"type": "string",
				"description": "Message ID to reply to"
			},
			"silent": {
				"type": "boolean",
				"description": "Send silently (no notification)"
			},
			"asVoice": {
				"type": "boolean",
				"description": "Send as voice message"
			},
			"filePath": {
				"type": "string",
				"description": "Path to file to attach"
			},
			"buffer": {
				"type": "string",
				"description": "Base64 payload for attachments"
			},
			"mimeType": {
				"type": "string",
				"description": "MIME type for buffer"
			},
			"caption": {
				"type": "string",
				"description": "Caption for media"
			},
			"effect": {
				"type": "string",
				"description": "Message effect (e.g., invisible-ink, balloons)"
			},
			"guildId": {
				"type": "string",
				"description": "Discord guild ID"
			},
			"channelId": {
				"type": "string",
				"description": "Discord/Telegram channel ID"
			},
			"chatId": {
				"type": "string",
				"description": "Telegram chat ID"
			}
		},
		"required": ["action"]
	}`
	return json.RawMessage(schema)
}

func (t *MessageTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params MessageParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	switch params.Action {
	case "send":
		return t.send(ctx, &params)
	case "broadcast":
		return t.broadcast(ctx, &params)
	default:
		return &Result{Content: "Unknown action: " + params.Action, IsError: true}, nil
	}
}

func (t *MessageTool) send(ctx context.Context, params *MessageParams) (*Result, error) {
	if params.Message == "" && params.FilePath == "" && params.Buffer == "" {
		return &Result{Content: "Message or file is required", IsError: true}, nil
	}

	// 确定目标
	target := params.Target
	if target == "" {
		target = params.ChatID
	}
	if target == "" {
		target = params.ChannelID
	}
	if target == "" {
		return &Result{Content: "Target is required (target, chatId, or channelId)", IsError: true}, nil
	}

	// 检查发送函数是否已注入
	if t.SendFunc == nil {
		return &Result{Content: "Message sending not configured (no channel connected)", IsError: true}, nil
	}

	opts := MessageOptions{
		ReplyTo:  params.ReplyTo,
		Silent:   params.Silent,
		AsVoice:  params.AsVoice,
		FilePath: params.FilePath,
		Buffer:   params.Buffer,
		MimeType: params.MimeType,
		Caption:  params.Caption,
		Effect:   params.Effect,
	}

	err := t.SendFunc(ctx, params.Channel, target, params.Message, opts)
	if err != nil {
		return &Result{Content: "Send failed: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Message sent to %s", target)}, nil
}

func (t *MessageTool) broadcast(ctx context.Context, params *MessageParams) (*Result, error) {
	if params.Message == "" {
		return &Result{Content: "Message is required for broadcast", IsError: true}, nil
	}

	targets := params.Targets
	if len(targets) == 0 && params.Target != "" {
		targets = []string{params.Target}
	}
	if len(targets) == 0 {
		return &Result{Content: "At least one target is required", IsError: true}, nil
	}

	if t.SendFunc == nil {
		return &Result{Content: "Message sending not configured", IsError: true}, nil
	}

	opts := MessageOptions{
		Silent:  params.Silent,
		AsVoice: params.AsVoice,
	}

	var sent, failed int
	var errors []string

	for _, target := range targets {
		err := t.SendFunc(ctx, params.Channel, target, params.Message, opts)
		if err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("%s: %s", target, err.Error()))
		} else {
			sent++
		}
	}

	result := map[string]interface{}{
		"sent":   sent,
		"failed": failed,
	}
	if len(errors) > 0 {
		result["errors"] = errors
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &Result{Content: string(data)}, nil
}

// SetSendFunc 设置发送函数
func (t *MessageTool) SetSendFunc(fn func(ctx context.Context, channel, target, message string, opts MessageOptions) error) {
	t.SendFunc = fn
}
