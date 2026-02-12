package signal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/user/openclaw-go/internal/channels"
)

// Config Signal 配置
type Config struct {
	// signal-cli REST API 地址
	APIURL    string   `json:"apiUrl"`    // 例如 http://localhost:8080
	Number    string   `json:"number"`    // 注册的手机号
	AllowFrom []string `json:"allowFrom"` // 允许的号码
}

// Channel Signal 渠道实现 (通过 signal-cli REST API)
type Channel struct {
	cfg       *Config
	handler   channels.MessageHandler
	client    *http.Client
	
	connected bool
	mu        sync.RWMutex
	
	ctx       context.Context
	cancel    context.CancelFunc
}

// signal-cli REST API 结构
type sendMessageRequest struct {
	Recipients []string `json:"recipients,omitempty"`
	Number     string   `json:"number,omitempty"`
	Message    string   `json:"message"`
	Base64Attachments []string `json:"base64_attachments,omitempty"`
	QuoteTimestamp int64  `json:"quote_timestamp,omitempty"`
	QuoteAuthor    string `json:"quote_author,omitempty"`
}

type sendResponse struct {
	Timestamp int64 `json:"timestamp"`
}

type receiveMessage struct {
	Envelope envelope `json:"envelope"`
}

type envelope struct {
	Source        string        `json:"source"`
	SourceName    string        `json:"sourceName"`
	SourceNumber  string        `json:"sourceNumber"`
	SourceUUID    string        `json:"sourceUuid"`
	Timestamp     int64         `json:"timestamp"`
	DataMessage   *dataMessage  `json:"dataMessage,omitempty"`
	SyncMessage   *syncMessage  `json:"syncMessage,omitempty"`
	TypingMessage *typingMessage `json:"typingMessage,omitempty"`
}

type dataMessage struct {
	Timestamp   int64        `json:"timestamp"`
	Message     string       `json:"message"`
	GroupInfo   *groupInfo   `json:"groupInfo,omitempty"`
	Quote       *quote       `json:"quote,omitempty"`
	Attachments []attachment `json:"attachments,omitempty"`
	Mentions    []mention    `json:"mentions,omitempty"`
}

type syncMessage struct {
	SentMessage *sentMessage `json:"sentMessage,omitempty"`
}

type sentMessage struct {
	Destination string       `json:"destination"`
	Timestamp   int64        `json:"timestamp"`
	Message     string       `json:"message"`
	DataMessage *dataMessage `json:"dataMessage,omitempty"`
}

type typingMessage struct {
	Action    string `json:"action"` // "STARTED" | "STOPPED"
	Timestamp int64  `json:"timestamp"`
}

type groupInfo struct {
	GroupID string `json:"groupId"`
	Type    string `json:"type"`
}

type quote struct {
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
}

type attachment struct {
	ContentType string `json:"contentType"`
	Filename    string `json:"filename"`
	ID          string `json:"id"`
	Size        int64  `json:"size"`
}

type mention struct {
	Start  int    `json:"start"`
	Length int    `json:"length"`
	UUID   string `json:"uuid"`
}

// New 创建 Signal 渠道
func New(cfg *Config) *Channel {
	if cfg.APIURL == "" {
		cfg.APIURL = "http://localhost:8080"
	}
	return &Channel{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Channel) ID() string {
	return channels.ChannelSignal
}

func (c *Channel) Label() string {
	return "Signal"
}

func (c *Channel) Start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)
	
	log.Info().Str("api", c.cfg.APIURL).Str("number", c.cfg.Number).Msg("Starting Signal channel...")
	
	// 验证连接
	if err := c.checkConnection(); err != nil {
		return fmt.Errorf("signal-cli connection failed: %w", err)
	}
	
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	
	// 启动消息接收轮询
	go c.pollMessages()
	
	log.Info().Msg("Signal channel started")
	return nil
}

func (c *Channel) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	
	log.Info().Msg("Signal channel stopped")
	return nil
}

func (c *Channel) Status() channels.ChannelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return channels.ChannelStatus{
		Connected: c.connected,
		Account:   c.cfg.Number,
		Details: map[string]interface{}{
			"apiUrl": c.cfg.APIURL,
		},
	}
}

func (c *Channel) Send(ctx context.Context, msg *channels.OutboundMessage) (*channels.SendResult, error) {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()
	
	if !connected {
		return nil, fmt.Errorf("signal not connected")
	}
	
	// 构建请求
	reqBody := sendMessageRequest{
		Number:     c.cfg.Number,
		Recipients: []string{msg.ChatID},
		Message:    msg.Text,
	}
	
	// 处理回复
	if msg.ReplyTo != "" {
		// msg.ReplyTo 应该是 timestamp
		var ts int64
		fmt.Sscanf(msg.ReplyTo, "%d", &ts)
		reqBody.QuoteTimestamp = ts
	}
	
	body, _ := json.Marshal(reqBody)
	
	url := fmt.Sprintf("%s/v2/send", c.cfg.APIURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("send failed: %s", string(respBody))
	}
	
	var sendResp sendResponse
	json.NewDecoder(resp.Body).Decode(&sendResp)
	
	return &channels.SendResult{
		MessageID: fmt.Sprintf("%d", sendResp.Timestamp),
		Timestamp: sendResp.Timestamp,
	}, nil
}

func (c *Channel) SetMessageHandler(handler channels.MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// checkConnection 检查 signal-cli 连接
func (c *Channel) checkConnection() error {
	url := fmt.Sprintf("%s/v1/about", c.cfg.APIURL)
	resp, err := c.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("signal-cli returned %d", resp.StatusCode)
	}
	
	return nil
}

// pollMessages 轮询消息
func (c *Channel) pollMessages() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.receiveMessages()
		}
	}
}

// receiveMessages 接收消息
func (c *Channel) receiveMessages() {
	url := fmt.Sprintf("%s/v1/receive/%s", c.cfg.APIURL, c.cfg.Number)
	resp, err := c.client.Get(url)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to receive signal messages")
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return
	}
	
	var messages []receiveMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return
	}
	
	for _, msg := range messages {
		c.handleMessage(&msg)
	}
}

// handleMessage 处理消息
func (c *Channel) handleMessage(msg *receiveMessage) {
	env := msg.Envelope
	
	// 忽略 typing 消息
	if env.TypingMessage != nil {
		return
	}
	
	// 获取数据消息
	var dataMsg *dataMessage
	if env.DataMessage != nil {
		dataMsg = env.DataMessage
	} else if env.SyncMessage != nil && env.SyncMessage.SentMessage != nil {
		// 自己发送的消息，忽略
		return
	}
	
	if dataMsg == nil || dataMsg.Message == "" {
		return
	}
	
	// 检查 allowlist
	if len(c.cfg.AllowFrom) > 0 {
		allowed := false
		for _, num := range c.cfg.AllowFrom {
			if num == env.SourceNumber || num == env.Source || num == "+"+env.SourceNumber {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Debug().Str("sender", env.SourceNumber).Msg("Signal message from non-allowed sender")
			return
		}
	}
	
	// 确定会话类型和 chat ID
	chatType := channels.ChatTypeDirect
	chatID := env.SourceNumber
	if dataMsg.GroupInfo != nil {
		chatType = channels.ChatTypeGroup
		chatID = dataMsg.GroupInfo.GroupID
	}
	
	// 构建消息
	inbound := &channels.InboundMessage{
		ID:         fmt.Sprintf("%d", dataMsg.Timestamp),
		Channel:    c.ID(),
		ChatID:     chatID,
		ChatType:   chatType,
		SenderID:   env.SourceNumber,
		SenderName: env.SourceName,
		Text:       dataMsg.Message,
		Timestamp:  dataMsg.Timestamp,
	}
	
	// 处理引用
	if dataMsg.Quote != nil {
		inbound.ReplyTo = fmt.Sprintf("%d", dataMsg.Quote.ID)
	}
	
	// 处理附件
	for _, att := range dataMsg.Attachments {
		attType := channels.AttachmentTypeDocument
		if len(att.ContentType) > 5 {
			switch att.ContentType[:5] {
			case "image":
				attType = channels.AttachmentTypeImage
			case "audio":
				attType = channels.AttachmentTypeAudio
			case "video":
				attType = channels.AttachmentTypeVideo
			}
		}
		inbound.Attachments = append(inbound.Attachments, channels.Attachment{
			Type:     attType,
			MimeType: att.ContentType,
			Filename: att.Filename,
		})
	}
	
	// 处理 mentions
	for _, m := range dataMsg.Mentions {
		inbound.Mentions = append(inbound.Mentions, m.UUID)
	}
	
	// 回调处理器
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()
	
	if handler != nil {
		handler(inbound)
	}
}

// Capabilities 返回渠道能力
func (c *Channel) Capabilities() channels.ChannelCapabilities {
	return channels.ChannelCapabilities{
		ChatTypes:         []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup},
		SupportsImages:    true,
		SupportsAudio:     true,
		SupportsVideo:     true,
		SupportsDocuments: true,
		SupportsVoice:     true,
		SupportsButtons:   false,
		SupportsReactions: true,
		SupportsThreads:   false,
		SupportsEditing:   false,
		SupportsDeleting:  true,
		SupportsMarkdown:  false,
		SupportsHTML:      false,
		MaxTextLength:     0,
		MaxFileSize:       100 * 1024 * 1024,
	}
}

// SendAttachment 发送附件
func (c *Channel) SendAttachment(ctx context.Context, chatID string, data []byte, filename, mimeType, caption string) (*channels.SendResult, error) {
	// Base64 编码
	encoded := "data:" + mimeType + ";base64," + string(data) // 需要实际 base64 编码
	
	reqBody := sendMessageRequest{
		Number:            c.cfg.Number,
		Recipients:        []string{chatID},
		Message:           caption,
		Base64Attachments: []string{encoded},
	}
	
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/v2/send", c.cfg.APIURL)
	
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var sendResp sendResponse
	json.NewDecoder(resp.Body).Decode(&sendResp)
	
	return &channels.SendResult{
		MessageID: fmt.Sprintf("%d", sendResp.Timestamp),
		Timestamp: sendResp.Timestamp,
	}, nil
}
