package channels

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Manager 管理所有渠道和消息路由
type Manager struct {
	channels   map[string]Channel
	mu         sync.RWMutex
	
	// 消息处理回调
	onMessage  func(msg *InboundMessage) error
	
	// Agent 响应回调 (用于流式输出)
	onResponse func(chatID, messageID, content string, done bool)
	
	ctx    context.Context
	cancel context.CancelFunc
}

// NewManager 创建渠道管理器
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		channels: make(map[string]Channel),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Register 注册渠道
func (m *Manager) Register(ch Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.channels[ch.ID()] = ch
	
	// 设置消息回调
	ch.SetMessageHandler(func(msg *InboundMessage) {
		m.handleInbound(msg)
	})
	
	log.Info().Str("channel", ch.ID()).Msg("Channel registered")
}

// Get 获取渠道
func (m *Manager) Get(id string) (Channel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ch, ok := m.channels[id]
	return ch, ok
}

// List 列出所有渠道
func (m *Manager) List() []Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		result = append(result, ch)
	}
	return result
}

// StartAll 启动所有渠道
func (m *Manager) StartAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for id, ch := range m.channels {
		if err := ch.Start(m.ctx); err != nil {
			log.Error().Err(err).Str("channel", id).Msg("Failed to start channel")
			// 继续启动其他渠道
		} else {
			log.Info().Str("channel", id).Msg("Channel started")
		}
	}
	return nil
}

// StopAll 停止所有渠道
func (m *Manager) StopAll() error {
	m.cancel()
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for id, ch := range m.channels {
		if err := ch.Stop(); err != nil {
			log.Error().Err(err).Str("channel", id).Msg("Failed to stop channel")
		}
	}
	return nil
}

// SetMessageHandler 设置全局消息处理回调
func (m *Manager) SetMessageHandler(handler func(msg *InboundMessage) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onMessage = handler
}

// SetResponseHandler 设置响应回调 (用于流式输出)
func (m *Manager) SetResponseHandler(handler func(chatID, messageID, content string, done bool)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onResponse = handler
}

// handleInbound 处理入站消息
func (m *Manager) handleInbound(msg *InboundMessage) {
	log.Debug().
		Str("channel", msg.Channel).
		Str("chatId", msg.ChatID).
		Str("senderId", msg.SenderID).
		Str("text", truncate(msg.Text, 50)).
		Msg("Inbound message")
	
	m.mu.RLock()
	handler := m.onMessage
	m.mu.RUnlock()
	
	if handler != nil {
		if err := handler(msg); err != nil {
			log.Error().Err(err).Msg("Message handler error")
		}
	}
}

// Send 发送消息到指定渠道
func (m *Manager) Send(ctx context.Context, channelID string, msg *OutboundMessage) (*SendResult, error) {
	m.mu.RLock()
	ch, ok := m.channels[channelID]
	m.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("channel not found: %s", channelID)
	}
	
	return ch.Send(ctx, msg)
}

// SendToChat 发送消息到指定聊天 (自动查找渠道)
func (m *Manager) SendToChat(ctx context.Context, channelID, chatID, text string) (*SendResult, error) {
	msg := &OutboundMessage{
		ChatID: chatID,
		Text:   text,
	}
	return m.Send(ctx, channelID, msg)
}

// Reply 回复消息
func (m *Manager) Reply(ctx context.Context, original *InboundMessage, text string) (*SendResult, error) {
	msg := &OutboundMessage{
		ChatID:  original.ChatID,
		Text:    text,
		ReplyTo: original.ID,
	}
	return m.Send(ctx, original.Channel, msg)
}

// Status 获取所有渠道状态
func (m *Manager) Status() map[string]ChannelStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	status := make(map[string]ChannelStatus, len(m.channels))
	for id, ch := range m.channels {
		status[id] = ch.Status()
	}
	return status
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// StreamingResponder 流式响应器，用于向渠道发送流式输出
type StreamingResponder struct {
	manager   *Manager
	channelID string
	chatID    string
	messageID string
	buffer    string
	mu        sync.Mutex
}

// NewStreamingResponder 创建流式响应器
func (m *Manager) NewStreamingResponder(channelID, chatID, messageID string) *StreamingResponder {
	return &StreamingResponder{
		manager:   m,
		channelID: channelID,
		chatID:    chatID,
		messageID: messageID,
	}
}

// Write 写入内容 (实现 io.Writer)
func (r *StreamingResponder) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	r.buffer += string(p)
	content := r.buffer
	r.mu.Unlock()
	
	r.manager.mu.RLock()
	handler := r.manager.onResponse
	r.manager.mu.RUnlock()
	
	if handler != nil {
		handler(r.chatID, r.messageID, content, false)
	}
	
	return len(p), nil
}

// Done 完成响应
func (r *StreamingResponder) Done() {
	r.mu.Lock()
	content := r.buffer
	r.mu.Unlock()
	
	r.manager.mu.RLock()
	handler := r.manager.onResponse
	r.manager.mu.RUnlock()
	
	if handler != nil {
		handler(r.chatID, r.messageID, content, true)
	}
}

// Content 获取当前内容
func (r *StreamingResponder) Content() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buffer
}

// MessageRouter 消息路由器，连接渠道和会话
type MessageRouter struct {
	channels    *Manager
	getSession  func(channelID, chatID string) (sessionKey string, isNew bool)
	runAgent    func(ctx context.Context, sessionKey string, message *InboundMessage) (string, error)
	sendMessage func(ctx context.Context, channelID, chatID, text string) error
}

// NewMessageRouter 创建消息路由器
func NewMessageRouter(channels *Manager) *MessageRouter {
	router := &MessageRouter{
		channels: channels,
	}
	
	// 设置消息处理
	channels.SetMessageHandler(func(msg *InboundMessage) error {
		return router.handleMessage(msg)
	})
	
	return router
}

// SetSessionResolver 设置会话解析器
func (r *MessageRouter) SetSessionResolver(resolver func(channelID, chatID string) (sessionKey string, isNew bool)) {
	r.getSession = resolver
}

// SetAgentRunner 设置 Agent 运行器
func (r *MessageRouter) SetAgentRunner(runner func(ctx context.Context, sessionKey string, message *InboundMessage) (string, error)) {
	r.runAgent = runner
}

// handleMessage 处理入站消息
func (r *MessageRouter) handleMessage(msg *InboundMessage) error {
	if r.getSession == nil || r.runAgent == nil {
		return fmt.Errorf("router not configured")
	}
	
	// 获取或创建会话
	sessionKey, isNew := r.getSession(msg.Channel, msg.ChatID)
	if isNew {
		log.Info().
			Str("channel", msg.Channel).
			Str("chatId", msg.ChatID).
			Str("sessionKey", sessionKey).
			Msg("New session created")
	}
	
	// 运行 Agent (异步)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		
		response, err := r.runAgent(ctx, sessionKey, msg)
		if err != nil {
			log.Error().Err(err).Str("sessionKey", sessionKey).Msg("Agent error")
			response = "抱歉，处理消息时出错: " + err.Error()
		}
		
		// 发送响应
		if response != "" {
			if _, err := r.channels.Reply(ctx, msg, response); err != nil {
				log.Error().Err(err).Msg("Failed to send response")
			}
		}
	}()
	
	return nil
}
