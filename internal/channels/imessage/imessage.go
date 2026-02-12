package imessage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/z8n24/openclaw-go/internal/channels"

	_ "github.com/mattn/go-sqlite3"
)

// Config iMessage 配置
type Config struct {
	AllowFrom   []string `json:"allowFrom"`   // 允许的联系人 (手机号/邮箱)
	PollInterval int     `json:"pollInterval"` // 轮询间隔 (毫秒)
}

// Channel iMessage 渠道实现 (macOS only)
type Channel struct {
	cfg       *Config
	handler   channels.MessageHandler
	db        *sql.DB
	
	connected    bool
	lastRowID    int64
	account      string
	mu           sync.RWMutex
	
	ctx    context.Context
	cancel context.CancelFunc
}

// New 创建 iMessage 渠道
func New(cfg *Config) *Channel {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 1000 // 默认 1 秒
	}
	return &Channel{
		cfg: cfg,
	}
}

func (c *Channel) ID() string {
	return channels.ChannelIMessage
}

func (c *Channel) Label() string {
	return "iMessage"
}

func (c *Channel) Start(ctx context.Context) error {
	// 检查平台
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("iMessage is only available on macOS")
	}
	
	c.ctx, c.cancel = context.WithCancel(ctx)
	
	log.Info().Msg("Starting iMessage channel...")
	
	// 打开 Messages 数据库
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, "Library", "Messages", "chat.db")
	
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("Messages database not found: %s", dbPath)
	}
	
	// 使用只读模式打开
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro&_journal_mode=WAL")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	c.db = db
	
	// 获取最新的消息 ID
	row := db.QueryRow("SELECT MAX(ROWID) FROM message")
	row.Scan(&c.lastRowID)
	
	// 获取账户信息
	c.account = c.getAccount()
	
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	
	// 启动消息轮询
	go c.pollMessages()
	
	log.Info().Str("account", c.account).Msg("iMessage channel started")
	return nil
}

func (c *Channel) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	
	if c.db != nil {
		c.db.Close()
	}
	
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	
	log.Info().Msg("iMessage channel stopped")
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
	
	if !connected {
		return nil, fmt.Errorf("iMessage not connected")
	}
	
	// 使用 AppleScript 发送消息
	script := fmt.Sprintf(`
		tell application "Messages"
			set targetService to 1st account whose service type = iMessage
			set targetBuddy to participant "%s" of targetService
			send "%s" to targetBuddy
		end tell
	`, msg.ChatID, escapeAppleScript(msg.Text))
	
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("send failed: %w, output: %s", err, string(output))
	}
	
	return &channels.SendResult{
		MessageID: fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (c *Channel) SetMessageHandler(handler channels.MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// pollMessages 轮询新消息
func (c *Channel) pollMessages() {
	ticker := time.NewTicker(time.Duration(c.cfg.PollInterval) * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.checkNewMessages()
		}
	}
}

// checkNewMessages 检查新消息
func (c *Channel) checkNewMessages() {
	query := `
		SELECT 
			m.ROWID,
			m.guid,
			m.text,
			m.date,
			m.is_from_me,
			m.handle_id,
			h.id as sender,
			c.chat_identifier,
			c.group_id
		FROM message m
		LEFT JOIN handle h ON m.handle_id = h.ROWID
		LEFT JOIN chat_message_join cmj ON m.ROWID = cmj.message_id
		LEFT JOIN chat c ON cmj.chat_id = c.ROWID
		WHERE m.ROWID > ?
		ORDER BY m.ROWID ASC
		LIMIT 100
	`
	
	rows, err := c.db.Query(query, c.lastRowID)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to query messages")
		return
	}
	defer rows.Close()
	
	for rows.Next() {
		var (
			rowID          int64
			guid           string
			text           sql.NullString
			date           int64
			isFromMe       int
			handleID       sql.NullInt64
			sender         sql.NullString
			chatIdentifier sql.NullString
			groupID        sql.NullString
		)
		
		err := rows.Scan(&rowID, &guid, &text, &date, &isFromMe, &handleID, &sender, &chatIdentifier, &groupID)
		if err != nil {
			continue
		}
		
		c.lastRowID = rowID
		
		// 忽略自己发送的消息
		if isFromMe == 1 {
			continue
		}
		
		// 忽略空消息
		if !text.Valid || text.String == "" {
			continue
		}
		
		// 检查 allowlist
		senderID := sender.String
		if len(c.cfg.AllowFrom) > 0 {
			allowed := false
			for _, id := range c.cfg.AllowFrom {
				if id == senderID || normalizePhone(id) == normalizePhone(senderID) {
					allowed = true
					break
				}
			}
			if !allowed {
				log.Debug().Str("sender", senderID).Msg("iMessage from non-allowed sender")
				continue
			}
		}
		
		// 确定会话类型和 chat ID
		chatType := channels.ChatTypeDirect
		chatID := senderID
		if groupID.Valid && groupID.String != "" {
			chatType = channels.ChatTypeGroup
			chatID = chatIdentifier.String
		}
		
		// 转换时间戳 (Apple Cocoa timestamp: seconds since 2001-01-01)
		// 需要加上 978307200 秒转换为 Unix timestamp
		timestamp := (date/1000000000 + 978307200) * 1000
		
		// 构建消息
		inbound := &channels.InboundMessage{
			ID:        guid,
			Channel:   c.ID(),
			ChatID:    chatID,
			ChatType:  chatType,
			SenderID:  senderID,
			Text:      text.String,
			Timestamp: timestamp,
		}
		
		// 回调处理器
		c.mu.RLock()
		handler := c.handler
		c.mu.RUnlock()
		
		if handler != nil {
			handler(inbound)
		}
	}
}

// getAccount 获取当前 iMessage 账户
func (c *Channel) getAccount() string {
	script := `
		tell application "Messages"
			set accts to every account whose service type is iMessage
			if (count of accts) > 0 then
				return id of item 1 of accts
			end if
		end tell
	`
	
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// escapeAppleScript 转义 AppleScript 字符串
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// normalizePhone 规范化手机号
func normalizePhone(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	return digits.String()
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
		SupportsButtons:   false,
		SupportsReactions: true,
		SupportsThreads:   false,
		SupportsEditing:   false,
		SupportsDeleting:  false,
		SupportsMarkdown:  false,
		SupportsHTML:      false,
		MaxTextLength:     20000,
		MaxFileSize:       100 * 1024 * 1024,
	}
}

// SendImage 发送图片
func (c *Channel) SendImage(ctx context.Context, chatID, imagePath string) (*channels.SendResult, error) {
	script := fmt.Sprintf(`
		tell application "Messages"
			set targetService to 1st account whose service type = iMessage
			set targetBuddy to participant "%s" of targetService
			send POSIX file "%s" to targetBuddy
		end tell
	`, chatID, imagePath)
	
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	if _, err := cmd.CombinedOutput(); err != nil {
		return nil, err
	}
	
	return &channels.SendResult{
		MessageID: fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

// GetContacts 获取联系人列表
func (c *Channel) GetContacts() ([]string, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not connected")
	}
	
	rows, err := c.db.Query("SELECT DISTINCT id FROM handle WHERE service = 'iMessage'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var contacts []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		contacts = append(contacts, id)
	}
	return contacts, nil
}
