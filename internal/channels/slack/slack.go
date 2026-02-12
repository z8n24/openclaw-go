package slack

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/z8n24/openclaw-go/internal/channels"
)

// Config Slack 配置
type Config struct {
	BotToken     string   `json:"botToken"`      // xoxb-...
	AppToken     string   `json:"appToken"`      // xapp-...
	AllowFrom    []string `json:"allowFrom"`     // 允许的用户 ID 或频道 ID
	DefaultChannel string `json:"defaultChannel"` // 默认频道
}

// Channel Slack 渠道实现
type Channel struct {
	cfg       *Config
	handler   channels.MessageHandler
	client    *slack.Client
	socket    *socketmode.Client

	connected bool
	botID     string
	botName   string
	mu        sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// New 创建 Slack 渠道
func New(cfg *Config) *Channel {
	return &Channel{
		cfg: cfg,
	}
}

func (c *Channel) ID() string {
	return channels.ChannelSlack
}

func (c *Channel) Label() string {
	return "Slack"
}

func (c *Channel) Start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	if c.cfg.BotToken == "" {
		return fmt.Errorf("slack bot token not configured")
	}

	log.Info().Msg("Starting Slack channel...")

	// 创建客户端
	c.client = slack.New(
		c.cfg.BotToken,
		slack.OptionAppLevelToken(c.cfg.AppToken),
	)

	// 获取 bot 信息
	authResp, err := c.client.AuthTest()
	if err != nil {
		return fmt.Errorf("auth test failed: %w", err)
	}
	c.botID = authResp.UserID
	c.botName = authResp.User

	log.Info().
		Str("botId", c.botID).
		Str("botName", c.botName).
		Msg("Slack bot authenticated")

	// 如果有 App Token，使用 Socket Mode
	if c.cfg.AppToken != "" {
		c.socket = socketmode.New(
			c.client,
			socketmode.OptionDebug(false),
		)

		go c.runSocketMode()
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	log.Info().Msg("Slack channel started")
	return nil
}

func (c *Channel) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	log.Info().Msg("Slack channel stopped")
	return nil
}

func (c *Channel) Status() channels.ChannelStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return channels.ChannelStatus{
		Connected: c.connected,
		Account:   c.botName,
		Details: map[string]interface{}{
			"botId": c.botID,
		},
	}
}

func (c *Channel) Send(ctx context.Context, msg *channels.OutboundMessage) (*channels.SendResult, error) {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected || c.client == nil {
		return nil, fmt.Errorf("slack not connected")
	}

	channelID := msg.ChatID
	if channelID == "" {
		channelID = c.cfg.DefaultChannel
	}

	options := []slack.MsgOption{
		slack.MsgOptionText(msg.Text, false),
	}

	// 处理 Markdown
	if msg.ParseMode == "markdown" {
		options = append(options, slack.MsgOptionEnableLinkUnfurl())
	}

	// 回复线程
	if msg.ReplyTo != "" {
		options = append(options, slack.MsgOptionTS(msg.ReplyTo))
	}

	// 发送按钮 (使用 Block Kit)
	if len(msg.Buttons) > 0 {
		var elements []slack.BlockElement
		for _, btn := range msg.Buttons {
			if btn.URL != "" {
				elements = append(elements, slack.NewButtonBlockElement(
					btn.CallbackData,
					btn.Text,
					slack.NewTextBlockObject("plain_text", btn.Text, false, false),
				).WithStyle(slack.StylePrimary).WithURL(btn.URL))
			} else {
				elements = append(elements, slack.NewButtonBlockElement(
					btn.CallbackData,
					btn.Text,
					slack.NewTextBlockObject("plain_text", btn.Text, false, false),
				))
			}
		}
		actionBlock := slack.NewActionBlock("", elements...)
		options = append(options, slack.MsgOptionBlocks(actionBlock))
	}

	_, timestamp, err := c.client.PostMessageContext(ctx, channelID, options...)
	if err != nil {
		return nil, fmt.Errorf("post message failed: %w", err)
	}

	return &channels.SendResult{
		MessageID: timestamp,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (c *Channel) SetMessageHandler(handler channels.MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// runSocketMode 运行 Socket Mode
func (c *Channel) runSocketMode() {
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				return
			case evt := <-c.socket.Events:
				c.handleSocketEvent(evt)
			}
		}
	}()

	if err := c.socket.RunContext(c.ctx); err != nil {
		log.Error().Err(err).Msg("Socket mode error")
	}
}

// handleSocketEvent 处理 Socket Mode 事件
func (c *Channel) handleSocketEvent(evt socketmode.Event) {
	switch evt.Type {
	case socketmode.EventTypeEventsAPI:
		eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			return
		}
		c.socket.Ack(*evt.Request)
		c.handleEventsAPI(eventsAPIEvent)

	case socketmode.EventTypeInteractive:
		callback, ok := evt.Data.(slack.InteractionCallback)
		if !ok {
			return
		}
		c.socket.Ack(*evt.Request)
		c.handleInteraction(&callback)

	case socketmode.EventTypeSlashCommand:
		cmd, ok := evt.Data.(slack.SlashCommand)
		if !ok {
			return
		}
		c.socket.Ack(*evt.Request)
		c.handleSlashCommand(&cmd)
	}
}

// handleEventsAPI 处理 Events API
func (c *Channel) handleEventsAPI(evt slackevents.EventsAPIEvent) {
	switch evt.Type {
	case slackevents.CallbackEvent:
		innerEvent := evt.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			c.handleMessage(ev)
		case *slackevents.AppMentionEvent:
			c.handleMention(ev)
		}
	}
}

// handleMessage 处理消息
func (c *Channel) handleMessage(ev *slackevents.MessageEvent) {
	// 忽略 bot 消息
	if ev.BotID != "" || ev.User == c.botID {
		return
	}

	// 忽略编辑和删除
	if ev.SubType == "message_changed" || ev.SubType == "message_deleted" {
		return
	}

	// 检查 allowlist
	if len(c.cfg.AllowFrom) > 0 {
		allowed := false
		for _, id := range c.cfg.AllowFrom {
			if id == ev.User || id == ev.Channel {
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
	if strings.HasPrefix(ev.Channel, "C") || strings.HasPrefix(ev.Channel, "G") {
		chatType = channels.ChatTypeGroup
	}

	// 构建消息
	inbound := &channels.InboundMessage{
		ID:        ev.TimeStamp,
		Channel:   c.ID(),
		ChatID:    ev.Channel,
		ChatType:  chatType,
		SenderID:  ev.User,
		Text:      ev.Text,
		Timestamp: parseSlackTS(ev.TimeStamp),
	}

	// 处理线程
	if ev.ThreadTimeStamp != "" {
		inbound.ReplyTo = ev.ThreadTimeStamp
	}

	// 注意: slackevents.MessageEvent 不包含 Files 字段
	// 文件共享通过 file_shared 事件单独处理
	// 或者使用 conversations.history API 获取完整消息

	// 回调处理器
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()

	if handler != nil {
		handler(inbound)
	}
}

// handleMention 处理 @提及
func (c *Channel) handleMention(ev *slackevents.AppMentionEvent) {
	// 移除 bot mention
	text := ev.Text
	if strings.Contains(text, "<@"+c.botID+">") {
		text = strings.ReplaceAll(text, "<@"+c.botID+">", "")
		text = strings.TrimSpace(text)
	}

	inbound := &channels.InboundMessage{
		ID:        ev.TimeStamp,
		Channel:   c.ID(),
		ChatID:    ev.Channel,
		ChatType:  channels.ChatTypeGroup,
		SenderID:  ev.User,
		Text:      text,
		Timestamp: parseSlackTS(ev.TimeStamp),
		Mentions:  []string{c.botID},
	}

	if ev.ThreadTimeStamp != "" {
		inbound.ReplyTo = ev.ThreadTimeStamp
	}

	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()

	if handler != nil {
		handler(inbound)
	}
}

// handleInteraction 处理交互事件
func (c *Channel) handleInteraction(callback *slack.InteractionCallback) {
	// 处理按钮点击
	for _, action := range callback.ActionCallback.BlockActions {
		inbound := &channels.InboundMessage{
			ID:        callback.MessageTs,
			Channel:   c.ID(),
			ChatID:    callback.Channel.ID,
			ChatType:  channels.ChatTypeDirect,
			SenderID:  callback.User.ID,
			Text:      action.Value,
			Timestamp: time.Now().UnixMilli(),
			Metadata: map[string]string{
				"type":     "interaction",
				"actionId": action.ActionID,
			},
		}

		c.mu.RLock()
		handler := c.handler
		c.mu.RUnlock()

		if handler != nil {
			handler(inbound)
		}
	}
}

// handleSlashCommand 处理斜杠命令
func (c *Channel) handleSlashCommand(cmd *slack.SlashCommand) {
	inbound := &channels.InboundMessage{
		ID:        cmd.TriggerID,
		Channel:   c.ID(),
		ChatID:    cmd.ChannelID,
		ChatType:  channels.ChatTypeDirect,
		SenderID:  cmd.UserID,
		Text:      cmd.Command + " " + cmd.Text,
		Timestamp: time.Now().UnixMilli(),
		Metadata: map[string]string{
			"type":    "slash_command",
			"command": cmd.Command,
		},
	}

	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()

	if handler != nil {
		handler(inbound)
	}
}

// parseSlackTS 解析 Slack 时间戳
func parseSlackTS(ts string) int64 {
	// Slack ts 格式: "1234567890.123456"
	var sec, usec int64
	fmt.Sscanf(ts, "%d.%d", &sec, &usec)
	return sec*1000 + usec/1000
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
		MaxTextLength:     40000,
		MaxFileSize:       1024 * 1024 * 1024, // 1GB
	}
}

// EditMessage 编辑消息
func (c *Channel) EditMessage(ctx context.Context, channelID, ts, text string) error {
	_, _, _, err := c.client.UpdateMessageContext(ctx, channelID, ts, slack.MsgOptionText(text, false))
	return err
}

// DeleteMessage 删除消息
func (c *Channel) DeleteMessage(ctx context.Context, channelID, ts string) error {
	_, _, err := c.client.DeleteMessageContext(ctx, channelID, ts)
	return err
}

// AddReaction 添加反应
func (c *Channel) AddReaction(channelID, ts, emoji string) error {
	return c.client.AddReaction(emoji, slack.ItemRef{Channel: channelID, Timestamp: ts})
}

// UploadFile 上传文件
func (c *Channel) UploadFile(ctx context.Context, channelID string, filename string, content []byte) error {
	_, err := c.client.UploadFileContext(ctx, slack.FileUploadParameters{
		Channels: []string{channelID},
		Filename: filename,
		Content:  string(content),
	})
	return err
}
