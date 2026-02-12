package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/z8n24/openclaw-go/internal/agents"
)

// Session 表示一个对话会话
type Session struct {
	Key           string    `json:"key"`
	Kind          string    `json:"kind"`
	Label         string    `json:"label,omitempty"`
	Channel       string    `json:"channel,omitempty"`
	ChatID        string    `json:"chatId,omitempty"`
	Model         string    `json:"model,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	
	messages []agents.Message
	mu       sync.RWMutex
}

// Manager 管理所有会话
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager 创建会话管理器
func NewManager() *Manager {
	m := &Manager{
		sessions: make(map[string]*Session),
	}
	m.sessions["main"] = &Session{
		Key:       "main",
		Kind:      "main",
		Label:     "Main Session",
		CreatedAt: time.Now(),
	}
	return m
}

// Get 获取会话
func (m *Manager) Get(key string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[key]
	return s, ok
}

// GetOrCreate 获取或创建会话
func (m *Manager) GetOrCreate(key, kind, label string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if s, ok := m.sessions[key]; ok {
		return s
	}
	
	s := &Session{
		Key:       key,
		Kind:      kind,
		Label:     label,
		CreatedAt: time.Now(),
	}
	m.sessions[key] = s
	return s
}

// AddMessage 添加消息到会话
func (s *Session) AddMessage(msg agents.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
	s.LastMessageAt = time.Now()
}

// GetMessages 获取会话消息
func (s *Session) GetMessages() []agents.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := make([]agents.Message, len(s.messages))
	copy(msgs, s.messages)
	return msgs
}

// ClearMessages 清空消息
func (s *Session) ClearMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
}

// ToolRegistry 工具注册表
type ToolRegistry struct {
	tools   map[string]ToolExecutor
	schemas map[string]agents.Tool
}

// ToolExecutor 工具执行器
type ToolExecutor func(ctx context.Context, args json.RawMessage) (string, error)

// NewToolRegistry 创建工具注册表
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:   make(map[string]ToolExecutor),
		schemas: make(map[string]agents.Tool),
	}
}

// Register 注册工具
func (r *ToolRegistry) Register(name string, executor ToolExecutor) {
	r.tools[name] = executor
}

// RegisterWithSchema 注册工具带 schema
func (r *ToolRegistry) RegisterWithSchema(tool agents.Tool, executor ToolExecutor) {
	r.tools[tool.Name] = executor
	r.schemas[tool.Name] = tool
}

// Execute 执行工具
func (r *ToolRegistry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	executor, ok := r.tools[name]
	if !ok {
		return fmt.Sprintf("Tool not found: %s", name), nil
	}
	return executor(ctx, args)
}

// GetSchemas 获取所有工具的 schema
func (r *ToolRegistry) GetSchemas() []agents.Tool {
	schemas := make([]agents.Tool, 0, len(r.schemas))
	for _, s := range r.schemas {
		schemas = append(schemas, s)
	}
	return schemas
}

// AgentLoop 运行 agent 循环
type AgentLoop struct {
	provider agents.Provider
	tools    *ToolRegistry
	session  *Session
	system   string
	model    string
}

// NewAgentLoop 创建 agent 循环
func NewAgentLoop(provider agents.Provider, tools *ToolRegistry, session *Session, system, model string) *AgentLoop {
	return &AgentLoop{
		provider: provider,
		tools:    tools,
		session:  session,
		system:   system,
		model:    model,
	}
}

// Run 运行 agent 循环
func (l *AgentLoop) Run(ctx context.Context, userMessage string, onDelta func(string)) (*agents.ChatResponse, error) {
	// 添加用户消息
	l.session.AddMessage(agents.Message{
		Role:    "user",
		Content: userMessage,
	})
	
	// 获取工具定义
	toolDefs := []agents.Tool{
		{
			Name:        "read",
			Description: "Read the contents of a file. Supports text files and images.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":   map[string]interface{}{"type": "string", "description": "Path to the file to read"},
					"offset": map[string]interface{}{"type": "number", "description": "Line number to start from (1-indexed)"},
					"limit":  map[string]interface{}{"type": "number", "description": "Maximum lines to read"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "write",
			Description: "Write content to a file. Creates parent directories automatically.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string", "description": "Path to write to"},
					"content": map[string]interface{}{"type": "string", "description": "Content to write"},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "edit",
			Description: "Edit a file by replacing exact text. oldText must match exactly.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string", "description": "Path to edit"},
					"oldText": map[string]interface{}{"type": "string", "description": "Exact text to find"},
					"newText": map[string]interface{}{"type": "string", "description": "Text to replace with"},
				},
				"required": []string{"path", "oldText", "newText"},
			},
		},
		{
			Name:        "exec",
			Description: "Execute shell commands. Returns stdout/stderr.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string", "description": "Shell command to execute"},
					"workdir": map[string]interface{}{"type": "string", "description": "Working directory"},
					"timeout": map[string]interface{}{"type": "number", "description": "Timeout in seconds"},
				},
				"required": []string{"command"},
			},
		},
	}
	
	maxIterations := 20
	for i := 0; i < maxIterations; i++ {
		// 构建请求
		req := &agents.ChatRequest{
			Model:     l.model,
			System:    l.system,
			Messages:  l.session.GetMessages(),
			Tools:     toolDefs,
			MaxTokens: 16384,
		}
		
		// 流式调用
		stream, err := l.provider.ChatStream(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("chat stream: %w", err)
		}
		
		var contentBuf string
		var toolCalls []agents.ToolCall
		
		for event := range stream {
			switch event.Type {
			case agents.StreamEventDelta:
				contentBuf += event.Content
				if onDelta != nil {
					onDelta(event.Content)
				}
			case agents.StreamEventToolCall:
				toolCalls = append(toolCalls, *event.ToolCall)
				// 显示工具调用
				if onDelta != nil {
					argsStr, _ := json.Marshal(event.ToolCall.Arguments)
					onDelta(fmt.Sprintf("\n[Tool: %s(%s)]\n", event.ToolCall.Name, string(argsStr)))
				}
			case agents.StreamEventError:
				return nil, event.Error
			}
		}
		
		// 添加 assistant 消息
		if contentBuf != "" || len(toolCalls) > 0 {
			// 构建 content blocks
			var contentBlocks []agents.ContentBlock
			if contentBuf != "" {
				contentBlocks = append(contentBlocks, agents.ContentBlock{
					Type: "text",
					Text: contentBuf,
				})
			}
			for _, tc := range toolCalls {
				contentBlocks = append(contentBlocks, agents.ContentBlock{
					Type:    "tool_use",
					ToolUse: &tc,
				})
			}
			
			l.session.AddMessage(agents.Message{
				Role:    "assistant",
				Content: contentBlocks,
			})
		}
		
		// 如果没有 tool calls，完成
		if len(toolCalls) == 0 {
			return &agents.ChatResponse{Content: contentBuf}, nil
		}
		
		// 执行 tools 并收集结果
		var toolResults []agents.ContentBlock
		for _, tc := range toolCalls {
			log.Debug().Str("tool", tc.Name).Interface("args", tc.Arguments).Msg("Executing tool")
			
			argsBytes, _ := json.Marshal(tc.Arguments)
			result, err := l.tools.Execute(ctx, tc.Name, argsBytes)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}
			
			// 显示工具结果
			if onDelta != nil {
				// 截断长结果
				displayResult := result
				if len(displayResult) > 500 {
					displayResult = displayResult[:500] + "...[truncated]"
				}
				onDelta(fmt.Sprintf("[Result: %s]\n", displayResult))
			}
			
			toolResults = append(toolResults, agents.ContentBlock{
				Type: "tool_result",
				ToolResult: &agents.ToolResult{
					ToolCallID: tc.ID,
					Content:    result,
				},
			})
		}
		
		// 添加 tool results 作为 user 消息
		l.session.AddMessage(agents.Message{
			Role:    "user",
			Content: toolResults,
		})
	}
	
	return nil, fmt.Errorf("max iterations reached")
}
