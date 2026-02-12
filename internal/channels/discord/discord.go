package discord

import (
	"context"
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"github.com/user/openclaw-go/internal/channels"
	"github.com/user/openclaw-go/internal/config"
)

// Channel 是 Discord 渠道实现
type Channel struct {
	cfg     *config.DiscordConfig
	handler channels.MessageHandler
	session *discordgo.Session
	
	connected bool
	botUser   *discordgo.User
	mu        sync.RWMutex
	
	ctx    context.Context
	cancel context.CancelFunc
}

// New 创建 Discord 渠道
func New(cfg *config.DiscordConfig) *Channel {
	return &Channel{
		cfg: cfg,
	}
}

func (c *Channel) ID() string {
	return channels.ChannelDiscord
}

func (c *Channel) Label() string {
	return "Discord"
}

func (c *Channel) Start(ctx context.Context) error {
	if c.cfg == nil || c.cfg.BotToken == "" {
		return fmt.Errorf("discord bot token not configured")
	}
	
	c.ctx, c.cancel = context.WithCancel(ctx)
	
	log.Info().Msg("Starting Discord channel...")
	
	// 创建 Discord session
	session, err := discordgo.New("Bot " + c.cfg.BotToken)
	if err != nil {
		return fmt.Errorf("failed to create discord session: %w", err)
	}
	
	// 设置 intents
	session.Identify.Intents = discordgo.IntentsGuildMessages | 
		discordgo.IntentsDirectMessages | 
		discordgo.IntentsGuildMessageReactions |
		discordgo.IntentsMessageContent
	
	// 注册事件处理器
	session.AddHandler(c.onReady)
	session.AddHandler(c.onMessageCreate)
	session.AddHandler(c.onInteractionCreate)
	
	// 连接
	if err := session.Open(); err != nil {
		return fmt.Errorf("failed to connect to discord: %w", err)
	}
	
	c.session = session
	
	return nil
}

func (c *Channel) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	
	if c.session != nil {
		c.session.Close()
	}
	
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	
	log.Info().Msg("Discord channel stopped")
	return nil
}

func (c *Channel) Status() channels.ChannelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	status := channels.ChannelStatus{
		Connected: c.connected,
	}
	
	if c.botUser != nil {
		status.Account = c.botUser.Username + "#" + c.botUser.Discriminator
		status.Details = map[string]interface{}{
			"botId":    c.botUser.ID,
			"username": c.botUser.Username,
		}
	}
	
	return status
}

func (c *Channel) Send(ctx context.Context, msg *channels.OutboundMessage) (*channels.SendResult, error) {
	if !c.connected || c.session == nil {
		return nil, fmt.Errorf("discord channel not connected")
	}
	
	// 构建消息
	content := msg.Text
	
	// 截断过长消息
	if len(content) > 2000 {
		content = content[:1997] + "..."
	}
	
	sent, err := c.session.ChannelMessageSend(msg.ChatID, content)
	if err != nil {
		return nil, fmt.Errorf("failed to send discord message: %w", err)
	}
	
	return &channels.SendResult{
		MessageID: sent.ID,
		Timestamp: sent.Timestamp.UnixMilli(),
	}, nil
}

func (c *Channel) SetMessageHandler(handler channels.MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// onReady 连接就绪
func (c *Channel) onReady(s *discordgo.Session, r *discordgo.Ready) {
	c.mu.Lock()
	c.botUser = r.User
	c.connected = true
	c.mu.Unlock()
	
	log.Info().
		Str("username", r.User.Username).
		Str("id", r.User.ID).
		Int("guilds", len(r.Guilds)).
		Msg("Discord bot connected")
}

// onMessageCreate 收到消息
func (c *Channel) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// 忽略自己的消息
	if m.Author.ID == s.State.User.ID {
		return
	}
	
	// 检查 allowlist
	if len(c.cfg.AllowFrom) > 0 {
		allowed := false
		for _, id := range c.cfg.AllowFrom {
			if id == m.Author.ID || id == m.Author.Username {
				allowed = true
				break
			}
		}
		if !allowed {
			return
		}
	}
	
	// 检查 guild 限制
	if len(c.cfg.Guilds) > 0 && m.GuildID != "" {
		allowed := false
		for _, guildID := range c.cfg.Guilds {
			if guildID == m.GuildID {
				allowed = true
				break
			}
		}
		if !allowed {
			return
		}
	}
	
	// 确定会话类型
	chatType := channels.ChatTypeDirect
	if m.GuildID != "" {
		chatType = channels.ChatTypeGroup
	}
	
	// 构建消息
	inbound := &channels.InboundMessage{
		ID:         m.ID,
		Channel:    c.ID(),
		ChatID:     m.ChannelID,
		ChatType:   chatType,
		SenderID:   m.Author.ID,
		SenderName: m.Author.Username,
		Text:       m.Content,
		Timestamp:  m.Timestamp.UnixMilli(),
		Metadata: map[string]string{
			"guildId": m.GuildID,
		},
	}
	
	// 回复消息
	if m.MessageReference != nil {
		inbound.ReplyTo = m.MessageReference.MessageID
	}
	
	// 处理附件
	for _, att := range m.Attachments {
		attType := channels.AttachmentTypeDocument
		if att.ContentType != "" {
			if contains(att.ContentType, "image") {
				attType = channels.AttachmentTypeImage
			} else if contains(att.ContentType, "audio") {
				attType = channels.AttachmentTypeAudio
			} else if contains(att.ContentType, "video") {
				attType = channels.AttachmentTypeVideo
			}
		}
		
		inbound.Attachments = append(inbound.Attachments, channels.Attachment{
			Type:     attType,
			URL:      att.URL,
			Filename: att.Filename,
			MimeType: att.ContentType,
		})
	}
	
	// 提取 mentions
	for _, user := range m.Mentions {
		inbound.Mentions = append(inbound.Mentions, "<@"+user.ID+">")
	}
	
	// 回调处理器
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()
	
	if handler != nil {
		handler(inbound)
	}
}

// onInteractionCreate 处理交互
func (c *Channel) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// TODO: 处理 slash commands 和 buttons
}

// Capabilities 返回渠道能力
func (c *Channel) Capabilities() channels.ChannelCapabilities {
	return channels.ChannelCapabilities{
		ChatTypes:         []channels.ChatType{channels.ChatTypeDirect, channels.ChatTypeGroup},
		SupportsImages:    true,
		SupportsAudio:     true,
		SupportsVideo:     true,
		SupportsDocuments: true,
		SupportsVoice:     false,
		SupportsButtons:   true,
		SupportsReactions: true,
		SupportsThreads:   true,
		SupportsEditing:   true,
		SupportsDeleting:  true,
		SupportsMarkdown:  true,
		SupportsHTML:      false,
		MaxTextLength:     2000,
		MaxFileSize:       25 * 1024 * 1024,
	}
}

// SendEmbed 发送嵌入消息
func (c *Channel) SendEmbed(ctx context.Context, channelID string, embed *discordgo.MessageEmbed) (*channels.SendResult, error) {
	if !c.connected || c.session == nil {
		return nil, fmt.Errorf("discord channel not connected")
	}
	
	sent, err := c.session.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		return nil, err
	}
	
	return &channels.SendResult{
		MessageID: sent.ID,
		Timestamp: sent.Timestamp.UnixMilli(),
	}, nil
}

// AddReaction 添加反应
func (c *Channel) AddReaction(ctx context.Context, channelID, messageID, emoji string) error {
	if !c.connected || c.session == nil {
		return fmt.Errorf("discord channel not connected")
	}
	
	return c.session.MessageReactionAdd(channelID, messageID, emoji)
}

// EditMessage 编辑消息
func (c *Channel) EditMessage(ctx context.Context, channelID, messageID, content string) error {
	if !c.connected || c.session == nil {
		return fmt.Errorf("discord channel not connected")
	}
	
	_, err := c.session.ChannelMessageEdit(channelID, messageID, content)
	return err
}

// DeleteMessage 删除消息
func (c *Channel) DeleteMessage(ctx context.Context, channelID, messageID string) error {
	if !c.connected || c.session == nil {
		return fmt.Errorf("discord channel not connected")
	}
	
	return c.session.ChannelMessageDelete(channelID, messageID)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
