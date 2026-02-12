package channels

import (
	"context"
)

// Channel 是消息渠道的抽象接口
type Channel interface {
	// ID 返回渠道唯一标识
	ID() string
	
	// Label 返回显示名称
	Label() string
	
	// Start 启动渠道连接
	Start(ctx context.Context) error
	
	// Stop 停止渠道连接
	Stop() error
	
	// Status 返回当前状态
	Status() ChannelStatus
	
	// Send 发送消息
	Send(ctx context.Context, msg *OutboundMessage) (*SendResult, error)
	
	// SetMessageHandler 设置消息处理回调
	SetMessageHandler(handler MessageHandler)
}

// ChannelStatus 渠道状态
type ChannelStatus struct {
	Connected bool   `json:"connected"`
	Error     string `json:"error,omitempty"`
	Account   string `json:"account,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// MessageHandler 消息处理回调
type MessageHandler func(msg *InboundMessage)

// InboundMessage 收到的消息
type InboundMessage struct {
	ID          string            `json:"id"`
	Channel     string            `json:"channel"`
	ChatID      string            `json:"chatId"`      // 会话/群组 ID
	ChatType    ChatType          `json:"chatType"`    // "direct" | "group"
	SenderID    string            `json:"senderId"`
	SenderName  string            `json:"senderName,omitempty"`
	Text        string            `json:"text"`
	Timestamp   int64             `json:"timestamp"`
	ReplyTo     string            `json:"replyTo,omitempty"`
	Attachments []Attachment      `json:"attachments,omitempty"`
	Mentions    []string          `json:"mentions,omitempty"`
	RawPayload  interface{}       `json:"rawPayload,omitempty"` // 原始平台数据
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// OutboundMessage 发送的消息
type OutboundMessage struct {
	ChatID      string       `json:"chatId"`
	Text        string       `json:"text"`
	ReplyTo     string       `json:"replyTo,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	
	// 平台特定选项
	Silent      bool              `json:"silent,omitempty"`
	ParseMode   string            `json:"parseMode,omitempty"` // "markdown" | "html"
	Buttons     []Button          `json:"buttons,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SendResult 发送结果
type SendResult struct {
	MessageID string `json:"messageId"`
	Timestamp int64  `json:"timestamp,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Attachment 附件
type Attachment struct {
	Type     AttachmentType `json:"type"` // "image" | "audio" | "video" | "document" | "voice"
	URL      string         `json:"url,omitempty"`
	Data     []byte         `json:"-"` // 二进制数据
	MimeType string         `json:"mimeType,omitempty"`
	Filename string         `json:"filename,omitempty"`
	Caption  string         `json:"caption,omitempty"`
	Duration int            `json:"duration,omitempty"` // 音视频时长 (秒)
}

// Button 交互按钮
type Button struct {
	Text         string `json:"text"`
	CallbackData string `json:"callbackData,omitempty"`
	URL          string `json:"url,omitempty"`
}

// ChatType 会话类型
type ChatType string

const (
	ChatTypeDirect ChatType = "direct"
	ChatTypeGroup  ChatType = "group"
)

// AttachmentType 附件类型
type AttachmentType string

const (
	AttachmentTypeImage    AttachmentType = "image"
	AttachmentTypeAudio    AttachmentType = "audio"
	AttachmentTypeVideo    AttachmentType = "video"
	AttachmentTypeDocument AttachmentType = "document"
	AttachmentTypeVoice    AttachmentType = "voice"
)

// ChannelCapabilities 渠道能力
type ChannelCapabilities struct {
	// 支持的会话类型
	ChatTypes []ChatType `json:"chatTypes"`
	
	// 媒体支持
	SupportsImages    bool `json:"supportsImages"`
	SupportsAudio     bool `json:"supportsAudio"`
	SupportsVideo     bool `json:"supportsVideo"`
	SupportsDocuments bool `json:"supportsDocuments"`
	SupportsVoice     bool `json:"supportsVoice"`
	
	// 交互支持
	SupportsButtons   bool `json:"supportsButtons"`
	SupportsReactions bool `json:"supportsReactions"`
	SupportsThreads   bool `json:"supportsThreads"`
	SupportsEditing   bool `json:"supportsEditing"`
	SupportsDeleting  bool `json:"supportsDeleting"`
	
	// 格式支持
	SupportsMarkdown bool `json:"supportsMarkdown"`
	SupportsHTML     bool `json:"supportsHTML"`
	
	// 限制
	MaxTextLength    int   `json:"maxTextLength,omitempty"`
	MaxImageSize     int64 `json:"maxImageSize,omitempty"`
	MaxFileSize      int64 `json:"maxFileSize,omitempty"`
}
