package webchat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/user/openclaw-go/internal/channels"
)

// Channel WebChat 渠道实现 (SSE)
type Channel struct {
	handler   channels.MessageHandler
	clients   map[string]*Client
	mu        sync.RWMutex
	connected bool

	ctx    context.Context
	cancel context.CancelFunc
}

// Client 表示一个 WebChat 客户端连接
type Client struct {
	ID        string
	SessionID string
	Events    chan *SSEEvent
	Done      chan struct{}
	CreatedAt time.Time
}

// SSEEvent SSE 事件
type SSEEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// WebMessage 前端发送的消息格式
type WebMessage struct {
	SessionID string       `json:"sessionId"`
	Text      string       `json:"text"`
	ReplyTo   string       `json:"replyTo,omitempty"`
	Files     []FileUpload `json:"files,omitempty"`
}

// FileUpload 文件上传
type FileUpload struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	Data     string `json:"data"` // base64
}

// WebResponse 发送给前端的响应
type WebResponse struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	Role      string `json:"role"` // "user" | "assistant"
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
	Done      bool   `json:"done,omitempty"`
}

// New 创建 WebChat 渠道
func New() *Channel {
	return &Channel{
		clients: make(map[string]*Client),
	}
}

func (c *Channel) ID() string {
	return channels.ChannelWebChat
}

func (c *Channel) Label() string {
	return "WebChat"
}

func (c *Channel) Start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)
	
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	
	log.Info().Msg("WebChat channel started")
	return nil
}

func (c *Channel) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	
	// 关闭所有客户端连接
	c.mu.Lock()
	for _, client := range c.clients {
		close(client.Done)
	}
	c.clients = make(map[string]*Client)
	c.connected = false
	c.mu.Unlock()
	
	log.Info().Msg("WebChat channel stopped")
	return nil
}

func (c *Channel) Status() channels.ChannelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return channels.ChannelStatus{
		Connected: c.connected,
		Details: map[string]interface{}{
			"clients": len(c.clients),
		},
	}
}

func (c *Channel) Send(ctx context.Context, msg *channels.OutboundMessage) (*channels.SendResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if !c.connected {
		return nil, fmt.Errorf("webchat channel not connected")
	}
	
	// 查找目标客户端
	sessionID := msg.ChatID
	
	response := &WebResponse{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      "assistant",
		Content:   msg.Text,
		Timestamp: time.Now().UnixMilli(),
		Done:      true,
	}
	
	event := &SSEEvent{
		Event: "message",
		Data:  response,
	}
	
	// 发送给所有匹配 session 的客户端
	sent := false
	for _, client := range c.clients {
		if client.SessionID == sessionID || sessionID == "" {
			select {
			case client.Events <- event:
				sent = true
			default:
				// 客户端缓冲区满，跳过
			}
		}
	}
	
	if !sent {
		return nil, fmt.Errorf("no active client for session: %s", sessionID)
	}
	
	return &channels.SendResult{
		MessageID: response.ID,
		Timestamp: response.Timestamp,
	}, nil
}

func (c *Channel) SetMessageHandler(handler channels.MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// RegisterRoutes 注册 HTTP 路由
func (c *Channel) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/message", c.handleMessage)
	router.GET("/events", c.handleSSE)
	router.GET("/status", c.handleStatus)
}

// handleMessage 处理消息 POST
func (c *Channel) handleMessage(ctx *gin.Context) {
	var msg WebMessage
	if err := ctx.ShouldBindJSON(&msg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if msg.Text == "" && len(msg.Files) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "text or files required"})
		return
	}
	
	// 确保有 session ID
	if msg.SessionID == "" {
		msg.SessionID = uuid.New().String()
	}
	
	// 构建 inbound 消息
	inbound := &channels.InboundMessage{
		ID:        uuid.New().String(),
		Channel:   c.ID(),
		ChatID:    msg.SessionID,
		ChatType:  channels.ChatTypeDirect,
		SenderID:  "web-user",
		SenderName: "Web User",
		Text:      msg.Text,
		Timestamp: time.Now().UnixMilli(),
		ReplyTo:   msg.ReplyTo,
	}
	
	// 处理文件附件
	for _, file := range msg.Files {
		inbound.Attachments = append(inbound.Attachments, channels.Attachment{
			Type:     detectAttachmentType(file.Type),
			Filename: file.Name,
			MimeType: file.Type,
		})
	}
	
	// 调用处理器
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()
	
	if handler != nil {
		go handler(inbound)
	}
	
	ctx.JSON(http.StatusOK, gin.H{
		"id":        inbound.ID,
		"sessionId": msg.SessionID,
	})
}

// handleSSE 处理 SSE 连接
func (c *Channel) handleSSE(ctx *gin.Context) {
	sessionID := ctx.Query("sessionId")
	if sessionID == "" {
		sessionID = uuid.New().String()
	}
	
	// 创建客户端
	client := &Client{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Events:    make(chan *SSEEvent, 100),
		Done:      make(chan struct{}),
		CreatedAt: time.Now(),
	}
	
	// 注册客户端
	c.mu.Lock()
	c.clients[client.ID] = client
	c.mu.Unlock()
	
	defer func() {
		c.mu.Lock()
		delete(c.clients, client.ID)
		c.mu.Unlock()
	}()
	
	// 设置 SSE headers
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")
	
	// 发送初始连接事件
	c.writeSSE(ctx.Writer, &SSEEvent{
		Event: "connected",
		Data: map[string]string{
			"clientId":  client.ID,
			"sessionId": sessionID,
		},
	})
	ctx.Writer.Flush()
	
	// 心跳 ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	// 事件循环
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-client.Done:
			return
		case <-ctx.Request.Context().Done():
			return
		case <-ticker.C:
			c.writeSSE(ctx.Writer, &SSEEvent{
				Event: "ping",
				Data:  map[string]int64{"ts": time.Now().UnixMilli()},
			})
			ctx.Writer.Flush()
		case event := <-client.Events:
			c.writeSSE(ctx.Writer, event)
			ctx.Writer.Flush()
		}
	}
}

// handleStatus 返回状态
func (c *Channel) handleStatus(ctx *gin.Context) {
	c.mu.RLock()
	clientCount := len(c.clients)
	c.mu.RUnlock()
	
	ctx.JSON(http.StatusOK, gin.H{
		"connected": c.connected,
		"clients":   clientCount,
	})
}

// writeSSE 写入 SSE 事件
func (c *Channel) writeSSE(w http.ResponseWriter, event *SSEEvent) {
	data, _ := json.Marshal(event.Data)
	fmt.Fprintf(w, "event: %s\n", event.Event)
	fmt.Fprintf(w, "data: %s\n\n", data)
}

// SendDelta 发送流式 delta (用于流式响应)
func (c *Channel) SendDelta(sessionID, messageID, content string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	event := &SSEEvent{
		Event: "delta",
		Data: map[string]interface{}{
			"id":        messageID,
			"sessionId": sessionID,
			"content":   content,
		},
	}
	
	for _, client := range c.clients {
		if client.SessionID == sessionID {
			select {
			case client.Events <- event:
			default:
			}
		}
	}
}

// SendDone 发送完成事件
func (c *Channel) SendDone(sessionID, messageID string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	event := &SSEEvent{
		Event: "done",
		Data: map[string]interface{}{
			"id":        messageID,
			"sessionId": sessionID,
		},
	}
	
	for _, client := range c.clients {
		if client.SessionID == sessionID {
			select {
			case client.Events <- event:
			default:
			}
		}
	}
}

// detectAttachmentType 检测附件类型
func detectAttachmentType(mimeType string) channels.AttachmentType {
	switch {
	case len(mimeType) > 5 && mimeType[:5] == "image":
		return channels.AttachmentTypeImage
	case len(mimeType) > 5 && mimeType[:5] == "audio":
		return channels.AttachmentTypeAudio
	case len(mimeType) > 5 && mimeType[:5] == "video":
		return channels.AttachmentTypeVideo
	default:
		return channels.AttachmentTypeDocument
	}
}

// Capabilities 返回渠道能力
func (c *Channel) Capabilities() channels.ChannelCapabilities {
	return channels.ChannelCapabilities{
		ChatTypes:         []channels.ChatType{channels.ChatTypeDirect},
		SupportsImages:    true,
		SupportsAudio:     true,
		SupportsVideo:     true,
		SupportsDocuments: true,
		SupportsVoice:     false,
		SupportsButtons:   false,
		SupportsReactions: false,
		SupportsThreads:   false,
		SupportsEditing:   false,
		SupportsDeleting:  false,
		SupportsMarkdown:  true,
		SupportsHTML:      false,
		MaxTextLength:     0, // 无限制
		MaxFileSize:       100 * 1024 * 1024, // 100MB
	}
}
