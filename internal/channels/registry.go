package channels

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
)

// Registry 管理所有渠道
type Registry struct {
	channels map[string]Channel
	mu       sync.RWMutex
	handler  MessageHandler
}

// NewRegistry 创建渠道注册表
func NewRegistry() *Registry {
	return &Registry{
		channels: make(map[string]Channel),
	}
}

// Register 注册一个渠道
func (r *Registry) Register(channel Channel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	id := channel.ID()
	if _, exists := r.channels[id]; exists {
		return fmt.Errorf("channel %s already registered", id)
	}
	
	// 设置消息处理器
	if r.handler != nil {
		channel.SetMessageHandler(r.handler)
	}
	
	r.channels[id] = channel
	log.Info().Str("channel", id).Msg("Channel registered")
	return nil
}

// Unregister 注销一个渠道
func (r *Registry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if channel, exists := r.channels[id]; exists {
		channel.Stop()
		delete(r.channels, id)
		log.Info().Str("channel", id).Msg("Channel unregistered")
	}
}

// Get 获取指定渠道
func (r *Registry) Get(id string) (Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, ok := r.channels[id]
	return ch, ok
}

// List 列出所有渠道
func (r *Registry) List() []Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	channels := make([]Channel, 0, len(r.channels))
	for _, ch := range r.channels {
		channels = append(channels, ch)
	}
	return channels
}

// SetMessageHandler 设置全局消息处理器
func (r *Registry) SetMessageHandler(handler MessageHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.handler = handler
	for _, ch := range r.channels {
		ch.SetMessageHandler(handler)
	}
}

// StartAll 启动所有渠道
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for id, ch := range r.channels {
		if err := ch.Start(ctx); err != nil {
			log.Error().Err(err).Str("channel", id).Msg("Failed to start channel")
			// 继续启动其他渠道
		}
	}
	return nil
}

// StopAll 停止所有渠道
func (r *Registry) StopAll() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for id, ch := range r.channels {
		if err := ch.Stop(); err != nil {
			log.Error().Err(err).Str("channel", id).Msg("Failed to stop channel")
		}
	}
}

// Send 通过指定渠道发送消息
func (r *Registry) Send(ctx context.Context, channelID string, msg *OutboundMessage) (*SendResult, error) {
	ch, ok := r.Get(channelID)
	if !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	return ch.Send(ctx, msg)
}

// GetStatus 获取所有渠道状态
func (r *Registry) GetStatus() map[string]ChannelStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	status := make(map[string]ChannelStatus)
	for id, ch := range r.channels {
		status[id] = ch.Status()
	}
	return status
}

// 预定义的渠道 ID
const (
	ChannelTelegram  = "telegram"
	ChannelWhatsApp  = "whatsapp"
	ChannelDiscord   = "discord"
	ChannelSignal    = "signal"
	ChannelIMessage  = "imessage"
	ChannelWebChat   = "webchat"
	ChannelSlack     = "slack"
	ChannelMatrix    = "matrix"
)

// ChannelOrder 渠道显示顺序
var ChannelOrder = []string{
	ChannelTelegram,
	ChannelWhatsApp,
	ChannelDiscord,
	ChannelSlack,
	ChannelSignal,
	ChannelIMessage,
	ChannelWebChat,
}

// ChannelMeta 渠道元信息
type ChannelMeta struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	SelectionLabel string `json:"selectionLabel"`
	DocsPath       string `json:"docsPath"`
	Blurb          string `json:"blurb"`
}

// ChannelMetadata 所有渠道的元信息
var ChannelMetadata = map[string]ChannelMeta{
	ChannelTelegram: {
		ID:             ChannelTelegram,
		Label:          "Telegram",
		SelectionLabel: "Telegram (Bot API)",
		DocsPath:       "/channels/telegram",
		Blurb:          "simplest way to get started — register a bot with @BotFather and get going.",
	},
	ChannelWhatsApp: {
		ID:             ChannelWhatsApp,
		Label:          "WhatsApp",
		SelectionLabel: "WhatsApp (QR link)",
		DocsPath:       "/channels/whatsapp",
		Blurb:          "works with your own number; recommend a separate phone + eSIM.",
	},
	ChannelDiscord: {
		ID:             ChannelDiscord,
		Label:          "Discord",
		SelectionLabel: "Discord (Bot API)",
		DocsPath:       "/channels/discord",
		Blurb:          "very well supported right now.",
	},
	ChannelSignal: {
		ID:             ChannelSignal,
		Label:          "Signal",
		SelectionLabel: "Signal (signal-cli)",
		DocsPath:       "/channels/signal",
		Blurb:          "signal-cli linked device; more setup required.",
	},
	ChannelIMessage: {
		ID:             ChannelIMessage,
		Label:          "iMessage",
		SelectionLabel: "iMessage (imsg)",
		DocsPath:       "/channels/imessage",
		Blurb:          "macOS only, still a work in progress.",
	},
	ChannelSlack: {
		ID:             ChannelSlack,
		Label:          "Slack",
		SelectionLabel: "Slack (Socket Mode)",
		DocsPath:       "/channels/slack",
		Blurb:          "supported (Socket Mode).",
	},
}
