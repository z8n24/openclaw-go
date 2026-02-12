package sessions

import (
	"context"
	"fmt"
	"time"

	"github.com/z8n24/openclaw-go/internal/agents"
	"github.com/z8n24/openclaw-go/internal/agents/tools"
)

// ToolsSessionManager 实现 tools.SessionManager 接口
type ToolsSessionManager struct {
	manager      *EnhancedManager
	provider     agents.Provider
	defaultModel string
	runAgent     AgentRunner
}

// AgentRunner 运行 agent 的函数类型
type AgentRunner func(ctx context.Context, session *EnhancedSession, message string) (string, error)

// NewToolsSessionManager 创建工具会话管理器
func NewToolsSessionManager(manager *EnhancedManager, provider agents.Provider, defaultModel string) *ToolsSessionManager {
	return &ToolsSessionManager{
		manager:      manager,
		provider:     provider,
		defaultModel: defaultModel,
	}
}

// SetAgentRunner 设置 agent 运行器
func (m *ToolsSessionManager) SetAgentRunner(runner AgentRunner) {
	m.runAgent = runner
}

// ListSessions 实现 tools.SessionManager
func (m *ToolsSessionManager) ListSessions(kinds []string, activeMinutes int, limit int) []tools.SessionInfo {
	var sessionKinds []SessionKind
	for _, k := range kinds {
		sessionKinds = append(sessionKinds, SessionKind(k))
	}
	
	sessions := m.manager.List(SessionFilter{
		Kinds:         sessionKinds,
		ActiveMinutes: activeMinutes,
		Limit:         limit,
	})
	
	result := make([]tools.SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, tools.SessionInfo{
			Key:           s.Key,
			Kind:          string(s.Kind),
			Label:         s.Label,
			Channel:       s.Channel,
			Model:         s.GetEffectiveModel(m.defaultModel),
			CreatedAt:     s.CreatedAt,
			LastMessageAt: s.LastMessageAt,
			MessageCount:  s.Usage.MessageCount,
		})
	}
	
	return result
}

// GetHistory 实现 tools.SessionManager
func (m *ToolsSessionManager) GetHistory(sessionKey string, limit int, includeTools bool) ([]tools.SessionMessage, error) {
	session, ok := m.manager.Get(sessionKey)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionKey)
	}
	
	messages := session.GetMessages()
	
	// 应用 limit
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	
	result := make([]tools.SessionMessage, 0, len(messages))
	for _, msg := range messages {
		// 过滤工具消息
		if !includeTools && (msg.Role == "tool" || containsToolContent(msg)) {
			continue
		}
		
		content := extractTextContent(msg)
		result = append(result, tools.SessionMessage{
			Role:    msg.Role,
			Content: content,
		})
	}
	
	return result, nil
}

// SendToSession 实现 tools.SessionManager
func (m *ToolsSessionManager) SendToSession(sessionKey, label, message string, timeoutSeconds int) (string, error) {
	// 查找会话
	var session *EnhancedSession
	var ok bool
	
	if sessionKey != "" {
		session, ok = m.manager.Get(sessionKey)
	} else if label != "" {
		// 通过 label 查找
		sessions := m.manager.List(SessionFilter{})
		for _, s := range sessions {
			if s.Label == label {
				session = s
				ok = true
				break
			}
		}
	}
	
	if !ok || session == nil {
		return "", fmt.Errorf("session not found")
	}
	
	// 添加消息到会话
	session.AddMessage(agents.Message{
		Role:    "user",
		Content: message,
	})
	
	// 如果有 agent runner，运行 agent
	if m.runAgent != nil {
		timeout := time.Duration(timeoutSeconds) * time.Second
		if timeout == 0 {
			timeout = 60 * time.Second
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		
		return m.runAgent(ctx, session, message)
	}
	
	return "Message delivered to session", nil
}

// SpawnSession 实现 tools.SessionManager
func (m *ToolsSessionManager) SpawnSession(task, label, model, agentID string, timeoutSeconds int) (string, error) {
	// 创建隔离会话
	if label == "" {
		label = fmt.Sprintf("Task: %s", truncate(task, 50))
	}
	if model == "" {
		model = m.defaultModel
	}
	
	session := m.manager.CreateIsolatedSession("main", label, model)
	
	// 添加任务消息
	session.AddMessage(agents.Message{
		Role:    "user",
		Content: task,
	})
	
	// 如果有 agent runner，异步运行
	if m.runAgent != nil {
		go func() {
			timeout := time.Duration(timeoutSeconds) * time.Second
			if timeout == 0 {
				timeout = 5 * 60 * time.Second // 默认 5 分钟
			}
			
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			
			m.runAgent(ctx, session, task)
		}()
	}
	
	return session.Key, nil
}

// GetSessionStatus 实现 tools.SessionManager
func (m *ToolsSessionManager) GetSessionStatus(sessionKey string) (map[string]interface{}, error) {
	if sessionKey == "" {
		sessionKey = "main"
	}
	
	session, ok := m.manager.Get(sessionKey)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionKey)
	}
	
	session.mu.RLock()
	defer session.mu.RUnlock()
	
	return map[string]interface{}{
		"key":           session.Key,
		"kind":          session.Kind,
		"label":         session.Label,
		"model":         session.GetEffectiveModel(m.defaultModel),
		"modelOverride": session.ModelOverride,
		"channel":       session.Channel,
		"chatId":        session.ChatID,
		"createdAt":     session.CreatedAt.Format(time.RFC3339),
		"lastMessageAt": session.LastMessageAt.Format(time.RFC3339),
		"usage": map[string]interface{}{
			"inputTokens":   session.Usage.InputTokens,
			"outputTokens":  session.Usage.OutputTokens,
			"totalTokens":   session.Usage.TotalTokens,
			"messageCount":  session.Usage.MessageCount,
			"toolCallCount": session.Usage.ToolCallCount,
		},
		"messageCount":    len(session.messages),
		"hasCompaction":   session.compactedSummary != "",
	}, nil
}

// SetSessionModel 实现 tools.SessionManager
func (m *ToolsSessionManager) SetSessionModel(sessionKey, model string) error {
	if sessionKey == "" {
		sessionKey = "main"
	}
	
	session, ok := m.manager.Get(sessionKey)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionKey)
	}
	
	session.SetModelOverride(model)
	return nil
}

// 辅助函数

func containsToolContent(msg agents.Message) bool {
	if blocks, ok := msg.Content.([]agents.ContentBlock); ok {
		for _, b := range blocks {
			if b.Type == "tool_use" || b.Type == "tool_result" {
				return true
			}
		}
	}
	return false
}

func extractTextContent(msg agents.Message) string {
	switch c := msg.Content.(type) {
	case string:
		return c
	case []agents.ContentBlock:
		var texts []string
		for _, b := range c {
			if b.Type == "text" {
				texts = append(texts, b.Text)
			}
		}
		if len(texts) > 0 {
			return texts[0]
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
