package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/z8n24/openclaw-go/internal/agents"
)

// SessionKind 会话类型
type SessionKind string

const (
	SessionKindMain     SessionKind = "main"     // 主会话 (1:1 用户对话)
	SessionKindGroup    SessionKind = "group"    // 群组会话 (多人)
	SessionKindIsolated SessionKind = "isolated" // 隔离会话 (子任务)
)

// EnhancedSession 增强的会话
type EnhancedSession struct {
	Key           string      `json:"key"`
	Kind          SessionKind `json:"kind"`
	Label         string      `json:"label,omitempty"`
	Channel       string      `json:"channel,omitempty"`
	ChatID        string      `json:"chatId,omitempty"`
	Model         string      `json:"model,omitempty"`
	ModelOverride string      `json:"modelOverride,omitempty"`
	ParentKey     string      `json:"parentKey,omitempty"` // 父会话 (用于 isolated)
	CreatedAt     time.Time   `json:"createdAt"`
	LastMessageAt time.Time   `json:"lastMessageAt"`
	
	// 使用量统计
	Usage SessionUsage `json:"usage"`
	
	// 消息
	messages []agents.Message
	
	// Compaction 状态
	compactedSummary string
	compactedAt      time.Time
	
	mu sync.RWMutex
}

// SessionUsage 会话使用统计
type SessionUsage struct {
	InputTokens    int64   `json:"inputTokens"`
	OutputTokens   int64   `json:"outputTokens"`
	TotalTokens    int64   `json:"totalTokens"`
	MessageCount   int     `json:"messageCount"`
	ToolCallCount  int     `json:"toolCallCount"`
	EstimatedCost  float64 `json:"estimatedCost,omitempty"`
}

// EnhancedManager 增强的会话管理器
type EnhancedManager struct {
	sessions     map[string]*EnhancedSession
	mu           sync.RWMutex
	
	// 持久化
	dataDir      string
	autosave     bool
	saveInterval time.Duration
	
	// Compaction 设置
	compactionThreshold int // 触发压缩的消息数量
	compactionKeep      int // 压缩后保留的最近消息数
	
	ctx    context.Context
	cancel context.CancelFunc
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	DataDir             string
	Autosave            bool
	SaveInterval        time.Duration
	CompactionThreshold int
	CompactionKeep      int
}

// NewEnhancedManager 创建增强的会话管理器
func NewEnhancedManager(cfg ManagerConfig) *EnhancedManager {
	if cfg.DataDir == "" {
		// 使用项目目录下的 sessions/
		cfg.DataDir = "sessions"
	}
	if cfg.SaveInterval == 0 {
		cfg.SaveInterval = 30 * time.Second
	}
	if cfg.CompactionThreshold == 0 {
		cfg.CompactionThreshold = 50
	}
	if cfg.CompactionKeep == 0 {
		cfg.CompactionKeep = 10
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	m := &EnhancedManager{
		sessions:            make(map[string]*EnhancedSession),
		dataDir:             cfg.DataDir,
		autosave:            cfg.Autosave,
		saveInterval:        cfg.SaveInterval,
		compactionThreshold: cfg.CompactionThreshold,
		compactionKeep:      cfg.CompactionKeep,
		ctx:                 ctx,
		cancel:              cancel,
	}
	
	// 创建主会话
	m.sessions["main"] = &EnhancedSession{
		Key:       "main",
		Kind:      SessionKindMain,
		Label:     "Main Session",
		CreatedAt: time.Now(),
	}
	
	// 加载持久化的会话
	m.loadSessions()
	
	// 启动自动保存
	if cfg.Autosave {
		go m.autosaveLoop()
	}
	
	return m
}

// Get 获取会话
func (m *EnhancedManager) Get(key string) (*EnhancedSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[key]
	return s, ok
}

// GetOrCreate 获取或创建会话
func (m *EnhancedManager) GetOrCreate(key string, kind SessionKind, label string) *EnhancedSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if s, ok := m.sessions[key]; ok {
		return s
	}
	
	s := &EnhancedSession{
		Key:       key,
		Kind:      kind,
		Label:     label,
		CreatedAt: time.Now(),
	}
	m.sessions[key] = s
	return s
}

// CreateGroupSession 创建群组会话
func (m *EnhancedManager) CreateGroupSession(channel, chatID, label string) *EnhancedSession {
	key := fmt.Sprintf("group:%s:%s", channel, chatID)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if s, ok := m.sessions[key]; ok {
		return s
	}
	
	s := &EnhancedSession{
		Key:       key,
		Kind:      SessionKindGroup,
		Label:     label,
		Channel:   channel,
		ChatID:    chatID,
		CreatedAt: time.Now(),
	}
	m.sessions[key] = s
	return s
}

// CreateIsolatedSession 创建隔离会话 (用于子任务)
func (m *EnhancedManager) CreateIsolatedSession(parentKey, label, model string) *EnhancedSession {
	key := fmt.Sprintf("isolated:%s", uuid.New().String()[:8])
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	s := &EnhancedSession{
		Key:       key,
		Kind:      SessionKindIsolated,
		Label:     label,
		ParentKey: parentKey,
		Model:     model,
		CreatedAt: time.Now(),
	}
	m.sessions[key] = s
	return s
}

// List 列出会话
func (m *EnhancedManager) List(filter SessionFilter) []*EnhancedSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var result []*EnhancedSession
	
	for _, s := range m.sessions {
		// 应用过滤器
		if len(filter.Kinds) > 0 {
			found := false
			for _, k := range filter.Kinds {
				if s.Kind == k {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		
		if filter.ActiveMinutes > 0 {
			cutoff := time.Now().Add(-time.Duration(filter.ActiveMinutes) * time.Minute)
			if s.LastMessageAt.Before(cutoff) {
				continue
			}
		}
		
		if filter.Channel != "" && s.Channel != filter.Channel {
			continue
		}
		
		result = append(result, s)
	}
	
	// 按最后消息时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastMessageAt.After(result[j].LastMessageAt)
	})
	
	// 应用 limit
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	
	return result
}

// SessionFilter 会话过滤器
type SessionFilter struct {
	Kinds         []SessionKind
	ActiveMinutes int
	Channel       string
	Limit         int
}

// Delete 删除会话
func (m *EnhancedManager) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if key == "main" {
		return fmt.Errorf("cannot delete main session")
	}
	
	if _, ok := m.sessions[key]; !ok {
		return fmt.Errorf("session not found: %s", key)
	}
	
	delete(m.sessions, key)
	
	// 删除持久化文件
	if m.dataDir != "" {
		transcriptPath := filepath.Join(m.dataDir, key+".json")
		os.Remove(transcriptPath)
	}
	
	return nil
}

// Close 关闭管理器
func (m *EnhancedManager) Close() error {
	m.cancel()
	
	// 保存所有会话
	if m.autosave {
		m.saveAllSessions()
	}
	
	return nil
}

// ============================================================================
// Session 方法
// ============================================================================

// AddMessage 添加消息
func (s *EnhancedSession) AddMessage(msg agents.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.messages = append(s.messages, msg)
	s.LastMessageAt = time.Now()
	s.Usage.MessageCount++
}

// GetMessages 获取消息
func (s *EnhancedSession) GetMessages() []agents.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	msgs := make([]agents.Message, len(s.messages))
	copy(msgs, s.messages)
	return msgs
}

// GetMessagesWithCompaction 获取消息 (包含压缩摘要)
func (s *EnhancedSession) GetMessagesWithCompaction() []agents.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.compactedSummary == "" {
		return s.GetMessages()
	}
	
	// 在消息前添加压缩摘要
	summaryMsg := agents.Message{
		Role:    "user",
		Content: fmt.Sprintf("[Previous conversation summary]\n%s\n[End of summary - continue from here]", s.compactedSummary),
	}
	
	msgs := make([]agents.Message, 0, len(s.messages)+1)
	msgs = append(msgs, summaryMsg)
	msgs = append(msgs, s.messages...)
	return msgs
}

// ClearMessages 清空消息
func (s *EnhancedSession) ClearMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
	s.compactedSummary = ""
}

// UpdateUsage 更新使用量
func (s *EnhancedSession) UpdateUsage(input, output int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.Usage.InputTokens += int64(input)
	s.Usage.OutputTokens += int64(output)
	s.Usage.TotalTokens += int64(input + output)
}

// IncrementToolCalls 增加工具调用计数
func (s *EnhancedSession) IncrementToolCalls(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Usage.ToolCallCount += count
}

// GetEffectiveModel 获取有效模型 (考虑 override)
func (s *EnhancedSession) GetEffectiveModel(defaultModel string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.ModelOverride != "" && s.ModelOverride != "default" {
		return s.ModelOverride
	}
	if s.Model != "" {
		return s.Model
	}
	return defaultModel
}

// SetModelOverride 设置模型覆盖
func (s *EnhancedSession) SetModelOverride(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if model == "default" {
		s.ModelOverride = ""
	} else {
		s.ModelOverride = model
	}
}

// ============================================================================
// Compaction (上下文压缩)
// ============================================================================

// NeedsCompaction 检查是否需要压缩
func (s *EnhancedSession) NeedsCompaction(threshold int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.messages) >= threshold
}

// Compact 压缩会话历史
func (s *EnhancedSession) Compact(summaryFunc func([]agents.Message) (string, error), keepCount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if len(s.messages) <= keepCount {
		return nil // 不需要压缩
	}
	
	// 分离要压缩的消息和要保留的消息
	toCompact := s.messages[:len(s.messages)-keepCount]
	toKeep := s.messages[len(s.messages)-keepCount:]
	
	// 生成摘要
	summary, err := summaryFunc(toCompact)
	if err != nil {
		return fmt.Errorf("generate summary: %w", err)
	}
	
	// 如果已有摘要，合并
	if s.compactedSummary != "" {
		summary = s.compactedSummary + "\n\n" + summary
	}
	
	s.compactedSummary = summary
	s.compactedAt = time.Now()
	s.messages = toKeep
	
	log.Info().
		Str("session", s.Key).
		Int("compacted", len(toCompact)).
		Int("kept", len(toKeep)).
		Msg("Session compacted")
	
	return nil
}

// ============================================================================
// 持久化
// ============================================================================

// Transcript 会话记录 (用于持久化)
type Transcript struct {
	Session  *EnhancedSession `json:"session"`
	Messages []agents.Message `json:"messages"`
	Summary  string           `json:"summary,omitempty"`
}

// loadSessions 加载持久化的会话
func (m *EnhancedManager) loadSessions() {
	if m.dataDir == "" {
		return
	}
	
	files, err := os.ReadDir(m.dataDir)
	if err != nil {
		return
	}
	
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		
		path := filepath.Join(m.dataDir, f.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		
		var transcript Transcript
		if err := json.Unmarshal(data, &transcript); err != nil {
			continue
		}
		
		if transcript.Session == nil {
			continue
		}
		
		s := transcript.Session
		s.messages = transcript.Messages
		s.compactedSummary = transcript.Summary
		
		m.sessions[s.Key] = s
		log.Debug().Str("session", s.Key).Msg("Loaded session")
	}
}

// saveAllSessions 保存所有会话
func (m *EnhancedManager) saveAllSessions() {
	if m.dataDir == "" {
		return
	}
	
	os.MkdirAll(m.dataDir, 0755)
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, s := range m.sessions {
		m.saveSession(s)
	}
}

// saveSession 保存单个会话
func (m *EnhancedManager) saveSession(s *EnhancedSession) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	transcript := Transcript{
		Session:  s,
		Messages: s.messages,
		Summary:  s.compactedSummary,
	}
	
	data, err := json.MarshalIndent(transcript, "", "  ")
	if err != nil {
		return
	}
	
	path := filepath.Join(m.dataDir, s.Key+".json")
	os.WriteFile(path, data, 0644)
}

// autosaveLoop 自动保存循环
func (m *EnhancedManager) autosaveLoop() {
	ticker := time.NewTicker(m.saveInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.saveAllSessions()
		}
	}
}

// SaveSession 手动保存会话
func (m *EnhancedManager) SaveSession(key string) error {
	s, ok := m.Get(key)
	if !ok {
		return fmt.Errorf("session not found: %s", key)
	}
	m.saveSession(s)
	return nil
}
