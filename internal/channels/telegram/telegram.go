package telegram

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
	"github.com/user/openclaw-go/internal/channels"
	"github.com/user/openclaw-go/internal/config"
)

// Channel 是 Telegram 渠道实现
type Channel struct {
	cfg     *config.TelegramConfig
	handler channels.MessageHandler
	bot     *tgbotapi.BotAPI
	
	connected bool
	botInfo   *BotInfo
	mu        sync.RWMutex
	
	ctx    context.Context
	cancel context.CancelFunc
}

// BotInfo Telegram bot 信息
type BotInfo struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"firstName"`
}

// New 创建 Telegram 渠道
func New(cfg *config.TelegramConfig) *Channel {
	return &Channel{
		cfg: cfg,
	}
}

func (c *Channel) ID() string {
	return channels.ChannelTelegram
}

func (c *Channel) Label() string {
	return "Telegram"
}

func (c *Channel) Start(ctx context.Context) error {
	if c.cfg == nil || c.cfg.BotToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}
	
	c.ctx, c.cancel = context.WithCancel(ctx)
	
	log.Info().Msg("Starting Telegram channel...")
	
	bot, err := tgbotapi.NewBotAPI(c.cfg.BotToken)
	if err != nil {
		return fmt.Errorf("failed to create telegram bot: %w", err)
	}
	
	c.bot = bot
	c.botInfo = &BotInfo{
		ID:        bot.Self.ID,
		Username:  bot.Self.UserName,
		FirstName: bot.Self.FirstName,
	}
	
	log.Info().
		Str("username", c.botInfo.Username).
		Int64("id", c.botInfo.ID).
		Msg("Telegram bot connected")
	
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	
	// 启动消息轮询
	go c.pollUpdates()
	
	return nil
}

func (c *Channel) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	
	if c.bot != nil {
		c.bot.StopReceivingUpdates()
	}
	
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	
	log.Info().Msg("Telegram channel stopped")
	return nil
}

func (c *Channel) Status() channels.ChannelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	status := channels.ChannelStatus{
		Connected: c.connected,
	}
	
	if c.botInfo != nil {
		status.Account = "@" + c.botInfo.Username
		status.Details = map[string]interface{}{
			"botId":    c.botInfo.ID,
			"username": c.botInfo.Username,
		}
	}
	
	return status
}

func (c *Channel) Send(ctx context.Context, msg *channels.OutboundMessage) (*channels.SendResult, error) {
	if !c.connected || c.bot == nil {
		return nil, fmt.Errorf("telegram channel not connected")
	}
	
	chatID, err := strconv.ParseInt(msg.ChatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}
	
	tgMsg := tgbotapi.NewMessage(chatID, msg.Text)
	
	// 设置格式
	if msg.ParseMode == "markdown" {
		tgMsg.ParseMode = tgbotapi.ModeMarkdown
	} else if msg.ParseMode == "html" {
		tgMsg.ParseMode = tgbotapi.ModeHTML
	}
	
	// 回复消息
	if msg.ReplyTo != "" {
		replyID, _ := strconv.Atoi(msg.ReplyTo)
		tgMsg.ReplyToMessageID = replyID
	}
	
	// 静音发送
	if msg.Silent {
		tgMsg.DisableNotification = true
	}
	
	// 发送按钮
	if len(msg.Buttons) > 0 {
		var buttons [][]tgbotapi.InlineKeyboardButton
		for _, btn := range msg.Buttons {
			if btn.URL != "" {
				buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
					tgbotapi.NewInlineKeyboardButtonURL(btn.Text, btn.URL),
				})
			} else if btn.CallbackData != "" {
				buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
					tgbotapi.NewInlineKeyboardButtonData(btn.Text, btn.CallbackData),
				})
			}
		}
		if len(buttons) > 0 {
			tgMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
		}
	}
	
	sent, err := c.bot.Send(tgMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to send telegram message: %w", err)
	}
	
	return &channels.SendResult{
		MessageID: strconv.Itoa(sent.MessageID),
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (c *Channel) SetMessageHandler(handler channels.MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// pollUpdates 轮询更新
func (c *Channel) pollUpdates() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	
	updates := c.bot.GetUpdatesChan(u)
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case update := <-updates:
			if update.Message != nil {
				c.handleMessage(update.Message)
			} else if update.CallbackQuery != nil {
				c.handleCallback(update.CallbackQuery)
			}
		}
	}
}

// handleMessage 处理消息
func (c *Channel) handleMessage(msg *tgbotapi.Message) {
	// 检查 allowlist
	if len(c.cfg.AllowFrom) > 0 {
		allowed := false
		senderID := strconv.FormatInt(msg.From.ID, 10)
		username := msg.From.UserName
		
		for _, id := range c.cfg.AllowFrom {
			if id == senderID || id == username || id == "@"+username {
				allowed = true
				break
			}
		}
		
		if !allowed {
			log.Debug().
				Str("sender", senderID).
				Str("username", username).
				Msg("Message from non-allowed sender, ignoring")
			return
		}
	}
	
	// 确定会话类型
	chatType := channels.ChatTypeDirect
	if msg.Chat.IsGroup() || msg.Chat.IsSuperGroup() {
		chatType = channels.ChatTypeGroup
	}
	
	// 获取文本
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}
	
	// 构建消息
	inbound := &channels.InboundMessage{
		ID:         strconv.Itoa(msg.MessageID),
		Channel:    c.ID(),
		ChatID:     strconv.FormatInt(msg.Chat.ID, 10),
		ChatType:   chatType,
		SenderID:   strconv.FormatInt(msg.From.ID, 10),
		SenderName: getName(msg.From),
		Text:       text,
		Timestamp:  int64(msg.Date) * 1000,
		Metadata: map[string]string{
			"chatTitle": msg.Chat.Title,
			"chatType":  msg.Chat.Type,
		},
	}
	
	// 回复消息
	if msg.ReplyToMessage != nil {
		inbound.ReplyTo = strconv.Itoa(msg.ReplyToMessage.MessageID)
	}
	
	// 处理附件
	if msg.Photo != nil && len(msg.Photo) > 0 {
		// 获取最大的图片
		photo := msg.Photo[len(msg.Photo)-1]
		inbound.Attachments = append(inbound.Attachments, channels.Attachment{
			Type:     channels.AttachmentTypeImage,
			URL:      photo.FileID, // 需要调用 GetFileDirectURL 获取真实 URL
			Filename: "photo.jpg",
		})
	}
	
	if msg.Document != nil {
		inbound.Attachments = append(inbound.Attachments, channels.Attachment{
			Type:     channels.AttachmentTypeDocument,
			URL:      msg.Document.FileID,
			Filename: msg.Document.FileName,
			MimeType: msg.Document.MimeType,
		})
	}
	
	if msg.Voice != nil {
		inbound.Attachments = append(inbound.Attachments, channels.Attachment{
			Type:     channels.AttachmentTypeVoice,
			URL:      msg.Voice.FileID,
			Duration: msg.Voice.Duration,
			MimeType: msg.Voice.MimeType,
		})
	}
	
	// 提取 mentions
	if msg.Entities != nil {
		for _, entity := range msg.Entities {
			if entity.Type == "mention" && entity.Offset >= 0 {
				end := entity.Offset + entity.Length
				if end <= len(text) {
					inbound.Mentions = append(inbound.Mentions, text[entity.Offset:end])
				}
			}
		}
	}
	
	// 回调处理器
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()
	
	if handler != nil {
		handler(inbound)
	}
}

// handleCallback 处理回调
func (c *Channel) handleCallback(callback *tgbotapi.CallbackQuery) {
	// 确认回调
	c.bot.Send(tgbotapi.NewCallback(callback.ID, ""))
	
	// 作为消息处理
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()
	
	if handler != nil {
		inbound := &channels.InboundMessage{
			ID:         callback.ID,
			Channel:    c.ID(),
			ChatID:     strconv.FormatInt(callback.Message.Chat.ID, 10),
			ChatType:   channels.ChatTypeDirect,
			SenderID:   strconv.FormatInt(callback.From.ID, 10),
			SenderName: getName(callback.From),
			Text:       callback.Data,
			Timestamp:  time.Now().UnixMilli(),
			Metadata: map[string]string{
				"type": "callback",
			},
		}
		handler(inbound)
	}
}

// getName 获取用户名称
func getName(user *tgbotapi.User) string {
	if user.FirstName != "" {
		if user.LastName != "" {
			return user.FirstName + " " + user.LastName
		}
		return user.FirstName
	}
	return user.UserName
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
		SupportsEditing:   true,
		SupportsDeleting:  true,
		SupportsMarkdown:  true,
		SupportsHTML:      true,
		MaxTextLength:     4096,
		MaxFileSize:       50 * 1024 * 1024,
	}
}

// SendPhoto 发送图片
func (c *Channel) SendPhoto(ctx context.Context, chatID string, photoURL string, caption string) (*channels.SendResult, error) {
	if !c.connected || c.bot == nil {
		return nil, fmt.Errorf("telegram channel not connected")
	}
	
	id, _ := strconv.ParseInt(chatID, 10, 64)
	photo := tgbotapi.NewPhoto(id, tgbotapi.FileURL(photoURL))
	photo.Caption = caption
	
	sent, err := c.bot.Send(photo)
	if err != nil {
		return nil, err
	}
	
	return &channels.SendResult{
		MessageID: strconv.Itoa(sent.MessageID),
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

// EditMessage 编辑消息
func (c *Channel) EditMessage(ctx context.Context, chatID, messageID, text string) error {
	if !c.connected || c.bot == nil {
		return fmt.Errorf("telegram channel not connected")
	}
	
	id, _ := strconv.ParseInt(chatID, 10, 64)
	msgID, _ := strconv.Atoi(messageID)
	
	edit := tgbotapi.NewEditMessageText(id, msgID, text)
	_, err := c.bot.Send(edit)
	return err
}

// DeleteMessage 删除消息
func (c *Channel) DeleteMessage(ctx context.Context, chatID, messageID string) error {
	if !c.connected || c.bot == nil {
		return fmt.Errorf("telegram channel not connected")
	}
	
	id, _ := strconv.ParseInt(chatID, 10, 64)
	msgID, _ := strconv.Atoi(messageID)
	
	_, err := c.bot.Request(tgbotapi.NewDeleteMessage(id, msgID))
	return err
}
