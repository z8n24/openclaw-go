package whatsapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mdp/qrterminal/v3"
	"github.com/rs/zerolog/log"
	"github.com/user/openclaw-go/internal/channels"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	_ "github.com/mattn/go-sqlite3"
)

// Config WhatsApp 配置
type Config struct {
	DataDir   string   `json:"dataDir"`
	AllowFrom []string `json:"allowFrom,omitempty"` // 允许的号码
}

// Channel WhatsApp 渠道实现
type Channel struct {
	cfg       *Config
	handler   channels.MessageHandler
	client    *whatsmeow.Client
	container *sqlstore.Container

	connected bool
	account   string
	mu        sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// New 创建 WhatsApp 渠道
func New(cfg *Config) *Channel {
	if cfg.DataDir == "" {
		home, _ := os.UserHomeDir()
		cfg.DataDir = filepath.Join(home, ".openclaw", "whatsapp")
	}
	return &Channel{
		cfg: cfg,
	}
}

func (c *Channel) ID() string {
	return channels.ChannelWhatsApp
}

func (c *Channel) Label() string {
	return "WhatsApp"
}

func (c *Channel) Start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	log.Info().Msg("Starting WhatsApp channel...")

	// 确保数据目录存在
	if err := os.MkdirAll(c.cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// 创建数据库存储
	dbPath := filepath.Join(c.cfg.DataDir, "whatsapp.db")
	dbLog := waLog.Noop
	container, err := sqlstore.New(ctx, "sqlite3", "file:"+dbPath+"?_foreign_keys=on", dbLog)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	c.container = container

	// 获取设备
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("get device: %w", err)
	}

	// 创建客户端
	client := whatsmeow.NewClient(device, waLog.Noop)
	c.client = client

	// 注册事件处理器
	client.AddEventHandler(c.handleEvent)

	// 连接
	if client.Store.ID == nil {
		// 需要扫描二维码登录
		qrChan, _ := client.GetQRChannel(ctx)
		err = client.Connect()
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}

		log.Info().Msg("WhatsApp: Please scan QR code to login")
		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				log.Info().Str("event", evt.Event).Msg("QR channel event")
			}
		}
	} else {
		err = client.Connect()
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
	}

	c.mu.Lock()
	c.connected = true
	if client.Store.ID != nil {
		c.account = client.Store.ID.User
	}
	c.mu.Unlock()

	log.Info().Str("account", c.account).Msg("WhatsApp connected")
	return nil
}

func (c *Channel) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}

	if c.client != nil {
		c.client.Disconnect()
	}

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	log.Info().Msg("WhatsApp channel stopped")
	return nil
}

func (c *Channel) Status() channels.ChannelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return channels.ChannelStatus{
		Connected: c.connected,
		Account:   c.account,
	}
}

func (c *Channel) Send(ctx context.Context, msg *channels.OutboundMessage) (*channels.SendResult, error) {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected || c.client == nil {
		return nil, fmt.Errorf("whatsapp not connected")
	}

	// 解析 JID
	jid, err := types.ParseJID(msg.ChatID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	// 发送消息
	resp, err := c.client.SendMessage(ctx, jid, &waE2E.Message{
		Conversation: proto.String(msg.Text),
	})
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}

	return &channels.SendResult{
		MessageID: resp.ID,
		Timestamp: resp.Timestamp.UnixMilli(),
	}, nil
}

func (c *Channel) SetMessageHandler(handler channels.MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// handleEvent 处理 WhatsApp 事件
func (c *Channel) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		c.handleMessage(v)
	case *events.Connected:
		log.Info().Msg("WhatsApp connected event")
		c.mu.Lock()
		c.connected = true
		c.mu.Unlock()
	case *events.Disconnected:
		log.Warn().Msg("WhatsApp disconnected")
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
	case *events.LoggedOut:
		log.Warn().Str("reason", v.Reason.String()).Msg("WhatsApp logged out")
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
	}
}

// handleMessage 处理消息
func (c *Channel) handleMessage(msg *events.Message) {
	// 忽略自己发送的消息
	if msg.Info.IsFromMe {
		return
	}

	// 检查 allowlist
	if len(c.cfg.AllowFrom) > 0 {
		allowed := false
		sender := msg.Info.Sender.User
		for _, id := range c.cfg.AllowFrom {
			if id == sender || id == "+"+sender {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Debug().Str("sender", sender).Msg("Message from non-allowed sender")
			return
		}
	}

	// 获取文本
	text := ""
	if msg.Message.GetConversation() != "" {
		text = msg.Message.GetConversation()
	} else if msg.Message.GetExtendedTextMessage() != nil {
		text = msg.Message.GetExtendedTextMessage().GetText()
	}

	// 确定会话类型
	chatType := channels.ChatTypeDirect
	if msg.Info.IsGroup {
		chatType = channels.ChatTypeGroup
	}

	// 构建消息
	inbound := &channels.InboundMessage{
		ID:         msg.Info.ID,
		Channel:    c.ID(),
		ChatID:     msg.Info.Chat.String(),
		ChatType:   chatType,
		SenderID:   msg.Info.Sender.User,
		SenderName: msg.Info.PushName,
		Text:       text,
		Timestamp:  msg.Info.Timestamp.UnixMilli(),
	}

	// 处理媒体附件
	if img := msg.Message.GetImageMessage(); img != nil {
		inbound.Attachments = append(inbound.Attachments, channels.Attachment{
			Type:     channels.AttachmentTypeImage,
			MimeType: img.GetMimetype(),
			Caption:  img.GetCaption(),
		})
		if text == "" {
			text = img.GetCaption()
			inbound.Text = text
		}
	}

	if doc := msg.Message.GetDocumentMessage(); doc != nil {
		inbound.Attachments = append(inbound.Attachments, channels.Attachment{
			Type:     channels.AttachmentTypeDocument,
			MimeType: doc.GetMimetype(),
			Filename: doc.GetFileName(),
			Caption:  doc.GetCaption(),
		})
	}

	if audio := msg.Message.GetAudioMessage(); audio != nil {
		attType := channels.AttachmentTypeAudio
		if audio.GetPTT() {
			attType = channels.AttachmentTypeVoice
		}
		inbound.Attachments = append(inbound.Attachments, channels.Attachment{
			Type:     attType,
			MimeType: audio.GetMimetype(),
			Duration: int(audio.GetSeconds()),
		})
	}

	// 回调处理器
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()

	if handler != nil {
		handler(inbound)
	}
}

// SendImage 发送图片
func (c *Channel) SendImage(ctx context.Context, chatID string, imageData []byte, mimeType, caption string) (*channels.SendResult, error) {
	jid, err := types.ParseJID(chatID)
	if err != nil {
		return nil, err
	}

	// 上传图片
	uploaded, err := c.client.Upload(ctx, imageData, whatsmeow.MediaImage)
	if err != nil {
		return nil, fmt.Errorf("upload image: %w", err)
	}

	imgMsg := &waE2E.ImageMessage{
		Caption:       proto.String(caption),
		Mimetype:      proto.String(mimeType),
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(imageData))),
	}

	resp, err := c.client.SendMessage(ctx, jid, &waE2E.Message{ImageMessage: imgMsg})
	if err != nil {
		return nil, err
	}

	return &channels.SendResult{
		MessageID: resp.ID,
		Timestamp: resp.Timestamp.UnixMilli(),
	}, nil
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
		SupportsButtons:   true,
		SupportsReactions: true,
		SupportsThreads:   false,
		SupportsEditing:   false,
		SupportsDeleting:  true,
		SupportsMarkdown:  false,
		SupportsHTML:      false,
		MaxTextLength:     65536,
		MaxFileSize:       100 * 1024 * 1024,
	}
}

// IsLoggedIn 检查是否已登录
func (c *Channel) IsLoggedIn() bool {
	if c.client == nil || c.client.Store == nil {
		return false
	}
	return c.client.Store.ID != nil
}

// Logout 登出
func (c *Channel) Logout(ctx context.Context) error {
	if c.client == nil {
		return nil
	}
	return c.client.Logout(ctx)
}
