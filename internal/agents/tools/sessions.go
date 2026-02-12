package tools

import (
	"context"
	"encoding/json"
	"time"
)

// SessionInfo 会话信息
type SessionInfo struct {
	Key           string    `json:"key"`
	Kind          string    `json:"kind"`
	Label         string    `json:"label,omitempty"`
	Channel       string    `json:"channel,omitempty"`
	Model         string    `json:"model,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	LastMessageAt time.Time `json:"lastMessageAt,omitempty"`
	MessageCount  int       `json:"messageCount,omitempty"`
}

// SessionMessage 会话消息
type SessionMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// SessionManager 会话管理接口 (由 Gateway 实现)
type SessionManager interface {
	ListSessions(kinds []string, activeMinutes int, limit int) []SessionInfo
	GetHistory(sessionKey string, limit int, includeTools bool) ([]SessionMessage, error)
	SendToSession(sessionKey, label, message string, timeoutSeconds int) (string, error)
	SpawnSession(task, label, model, agentID string, timeoutSeconds int) (string, error)
	GetSessionStatus(sessionKey string) (map[string]interface{}, error)
	SetSessionModel(sessionKey, model string) error
}

// =============================================================================
// sessions_list - 列出会话
// =============================================================================

type SessionsListTool struct {
	manager SessionManager
}

type SessionsListParams struct {
	Kinds         []string `json:"kinds,omitempty"`
	ActiveMinutes int      `json:"activeMinutes,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	MessageLimit  int      `json:"messageLimit,omitempty"`
}

func NewSessionsListTool() *SessionsListTool {
	return &SessionsListTool{}
}

func (t *SessionsListTool) Name() string {
	return ToolSessionsList
}

func (t *SessionsListTool) Description() string {
	return "List sessions with optional filters and last messages."
}

func (t *SessionsListTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"kinds": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Filter by session kinds (main, group, isolated)"
			},
			"activeMinutes": {
				"type": "number",
				"description": "Only show sessions active in last N minutes"
			},
			"limit": {
				"type": "number",
				"description": "Maximum sessions to return"
			},
			"messageLimit": {
				"type": "number",
				"description": "Include last N messages per session"
			}
		}
	}`
	return json.RawMessage(schema)
}

func (t *SessionsListTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params SessionsListParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if t.manager == nil {
		return &Result{Content: "Session manager not configured", IsError: true}, nil
	}

	sessions := t.manager.ListSessions(params.Kinds, params.ActiveMinutes, params.Limit)
	
	data, _ := json.MarshalIndent(sessions, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *SessionsListTool) SetManager(m SessionManager) {
	t.manager = m
}

// =============================================================================
// sessions_history - 获取会话历史
// =============================================================================

type SessionsHistoryTool struct {
	manager SessionManager
}

type SessionsHistoryParams struct {
	SessionKey   string `json:"sessionKey"`
	Limit        int    `json:"limit,omitempty"`
	IncludeTools bool   `json:"includeTools,omitempty"`
}

func NewSessionsHistoryTool() *SessionsHistoryTool {
	return &SessionsHistoryTool{}
}

func (t *SessionsHistoryTool) Name() string {
	return ToolSessionsHistory
}

func (t *SessionsHistoryTool) Description() string {
	return "Fetch message history for a session."
}

func (t *SessionsHistoryTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"sessionKey": {
				"type": "string",
				"description": "Session key to fetch history for"
			},
			"limit": {
				"type": "number",
				"description": "Maximum messages to return"
			},
			"includeTools": {
				"type": "boolean",
				"description": "Include tool call/result messages"
			}
		},
		"required": ["sessionKey"]
	}`
	return json.RawMessage(schema)
}

func (t *SessionsHistoryTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params SessionsHistoryParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if params.SessionKey == "" {
		return &Result{Content: "sessionKey is required", IsError: true}, nil
	}

	if t.manager == nil {
		return &Result{Content: "Session manager not configured", IsError: true}, nil
	}

	messages, err := t.manager.GetHistory(params.SessionKey, params.Limit, params.IncludeTools)
	if err != nil {
		return &Result{Content: "Failed to get history: " + err.Error(), IsError: true}, nil
	}

	data, _ := json.MarshalIndent(messages, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *SessionsHistoryTool) SetManager(m SessionManager) {
	t.manager = m
}

// =============================================================================
// sessions_send - 发送消息到会话
// =============================================================================

type SessionsSendTool struct {
	manager SessionManager
}

type SessionsSendParams struct {
	SessionKey     string `json:"sessionKey,omitempty"`
	Label          string `json:"label,omitempty"`
	Message        string `json:"message"`
	AgentID        string `json:"agentId,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

func NewSessionsSendTool() *SessionsSendTool {
	return &SessionsSendTool{}
}

func (t *SessionsSendTool) Name() string {
	return ToolSessionsSend
}

func (t *SessionsSendTool) Description() string {
	return "Send a message into another session. Use sessionKey or label to identify the target."
}

func (t *SessionsSendTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"sessionKey": {
				"type": "string",
				"description": "Target session key"
			},
			"label": {
				"type": "string",
				"description": "Target session label (alternative to sessionKey)"
			},
			"message": {
				"type": "string",
				"description": "Message to send"
			},
			"agentId": {
				"type": "string",
				"description": "Agent ID for the target session"
			},
			"timeoutSeconds": {
				"type": "number",
				"description": "Timeout for the operation"
			}
		},
		"required": ["message"]
	}`
	return json.RawMessage(schema)
}

func (t *SessionsSendTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params SessionsSendParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if params.Message == "" {
		return &Result{Content: "message is required", IsError: true}, nil
	}

	if params.SessionKey == "" && params.Label == "" {
		return &Result{Content: "sessionKey or label is required", IsError: true}, nil
	}

	if t.manager == nil {
		return &Result{Content: "Session manager not configured", IsError: true}, nil
	}

	response, err := t.manager.SendToSession(params.SessionKey, params.Label, params.Message, params.TimeoutSeconds)
	if err != nil {
		return &Result{Content: "Send failed: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: response}, nil
}

func (t *SessionsSendTool) SetManager(m SessionManager) {
	t.manager = m
}

// =============================================================================
// sessions_spawn - 生成子会话
// =============================================================================

type SessionsSpawnTool struct {
	manager SessionManager
}

type SessionsSpawnParams struct {
	Task              string `json:"task"`
	Label             string `json:"label,omitempty"`
	Model             string `json:"model,omitempty"`
	AgentID           string `json:"agentId,omitempty"`
	Thinking          string `json:"thinking,omitempty"`
	TimeoutSeconds    int    `json:"timeoutSeconds,omitempty"`
	RunTimeoutSeconds int    `json:"runTimeoutSeconds,omitempty"`
	Cleanup           string `json:"cleanup,omitempty"` // "delete" or "keep"
}

func NewSessionsSpawnTool() *SessionsSpawnTool {
	return &SessionsSpawnTool{}
}

func (t *SessionsSpawnTool) Name() string {
	return ToolSessionsSpawn
}

func (t *SessionsSpawnTool) Description() string {
	return "Spawn a background sub-agent run in an isolated session and announce the result back to the requester chat."
}

func (t *SessionsSpawnTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"task": {
				"type": "string",
				"description": "Task description for the sub-agent"
			},
			"label": {
				"type": "string",
				"description": "Label for the spawned session"
			},
			"model": {
				"type": "string",
				"description": "Model to use for the sub-agent"
			},
			"agentId": {
				"type": "string",
				"description": "Agent ID to spawn"
			},
			"thinking": {
				"type": "string",
				"description": "Thinking mode (off, low, medium, high)"
			},
			"timeoutSeconds": {
				"type": "number",
				"description": "Timeout for spawning"
			},
			"runTimeoutSeconds": {
				"type": "number",
				"description": "Timeout for the sub-agent run"
			},
			"cleanup": {
				"type": "string",
				"enum": ["delete", "keep"],
				"description": "What to do with the session after completion"
			}
		},
		"required": ["task"]
	}`
	return json.RawMessage(schema)
}

func (t *SessionsSpawnTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params SessionsSpawnParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if params.Task == "" {
		return &Result{Content: "task is required", IsError: true}, nil
	}

	if t.manager == nil {
		return &Result{Content: "Session manager not configured", IsError: true}, nil
	}

	sessionKey, err := t.manager.SpawnSession(params.Task, params.Label, params.Model, params.AgentID, params.TimeoutSeconds)
	if err != nil {
		return &Result{Content: "Spawn failed: " + err.Error(), IsError: true}, nil
	}

	result := map[string]interface{}{
		"sessionKey": sessionKey,
		"status":     "spawned",
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *SessionsSpawnTool) SetManager(m SessionManager) {
	t.manager = m
}

// =============================================================================
// session_status - 会话状态
// =============================================================================

type SessionStatusTool struct {
	manager SessionManager
}

type SessionStatusParams struct {
	SessionKey string `json:"sessionKey,omitempty"`
	Model      string `json:"model,omitempty"` // 可选：设置会话模型覆盖
}

func NewSessionStatusTool() *SessionStatusTool {
	return &SessionStatusTool{}
}

func (t *SessionStatusTool) Name() string {
	return ToolSessionStatus
}

func (t *SessionStatusTool) Description() string {
	return "Show a /status-equivalent session status card (usage + time + cost when available). Optional: set per-session model override."
}

func (t *SessionStatusTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"sessionKey": {
				"type": "string",
				"description": "Session key (defaults to current session)"
			},
			"model": {
				"type": "string",
				"description": "Set model override for session (use 'default' to reset)"
			}
		}
	}`
	return json.RawMessage(schema)
}

func (t *SessionStatusTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params SessionStatusParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if t.manager == nil {
		return &Result{Content: "Session manager not configured", IsError: true}, nil
	}

	// 如果提供了 model 参数，设置模型覆盖
	if params.Model != "" {
		err := t.manager.SetSessionModel(params.SessionKey, params.Model)
		if err != nil {
			return &Result{Content: "Failed to set model: " + err.Error(), IsError: true}, nil
		}
	}

	status, err := t.manager.GetSessionStatus(params.SessionKey)
	if err != nil {
		return &Result{Content: "Failed to get status: " + err.Error(), IsError: true}, nil
	}

	data, _ := json.MarshalIndent(status, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *SessionStatusTool) SetManager(m SessionManager) {
	t.manager = m
}

// =============================================================================
// agents_list - 列出可用的 Agents
// =============================================================================

type AgentsListTool struct {
	// 获取 Agent 列表的函数
	ListFunc func() []string
}

func NewAgentsListTool() *AgentsListTool {
	return &AgentsListTool{
		ListFunc: func() []string {
			return []string{"main"} // 默认只有 main agent
		},
	}
}

func (t *AgentsListTool) Name() string {
	return ToolAgentsList
}

func (t *AgentsListTool) Description() string {
	return "List agent ids you can target with sessions_spawn (based on allowlists)."
}

func (t *AgentsListTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type": "object", "properties": {}}`)
}

func (t *AgentsListTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	agents := t.ListFunc()
	
	result := map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	}
	
	data, _ := json.MarshalIndent(result, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *AgentsListTool) SetListFunc(fn func() []string) {
	t.ListFunc = fn
}

// =============================================================================
// 便捷函数：批量注册所有会话工具
// =============================================================================

func RegisterSessionTools(registry *Registry, manager SessionManager) {
	listTool := NewSessionsListTool()
	listTool.SetManager(manager)
	registry.Register(listTool)

	historyTool := NewSessionsHistoryTool()
	historyTool.SetManager(manager)
	registry.Register(historyTool)

	sendTool := NewSessionsSendTool()
	sendTool.SetManager(manager)
	registry.Register(sendTool)

	spawnTool := NewSessionsSpawnTool()
	spawnTool.SetManager(manager)
	registry.Register(spawnTool)

	statusTool := NewSessionStatusTool()
	statusTool.SetManager(manager)
	registry.Register(statusTool)

	agentsTool := NewAgentsListTool()
	registry.Register(agentsTool)
}
