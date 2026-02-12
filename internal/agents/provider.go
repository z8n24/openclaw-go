package agents

import (
	"context"
)

// Provider 是 LLM 提供商的抽象接口
type Provider interface {
	// ID 返回提供商标识
	ID() string
	
	// Name 返回显示名称
	Name() string
	
	// ListModels 列出支持的模型
	ListModels() []ModelInfo
	
	// Chat 发送对话请求 (非流式)
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	
	// ChatStream 发送对话请求 (流式)
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
}

// ModelInfo 模型信息
type ModelInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	ContextWindow int    `json:"contextWindow"`
	MaxOutput     int    `json:"maxOutput,omitempty"`
	SupportsTools bool   `json:"supportsTools"`
	SupportsVision bool  `json:"supportsVision"`
}

// ChatRequest 对话请求
type ChatRequest struct {
	Model       string       `json:"model"`
	Messages    []Message    `json:"messages"`
	Tools       []Tool       `json:"tools,omitempty"`
	System      string       `json:"system,omitempty"`
	MaxTokens   int          `json:"maxTokens,omitempty"`
	Temperature *float64     `json:"temperature,omitempty"`
	StopSeqs    []string     `json:"stopSequences,omitempty"`
	Thinking    *ThinkingConfig `json:"thinking,omitempty"`
}

// ThinkingConfig 思考模式配置
type ThinkingConfig struct {
	Type        string `json:"type"` // "enabled" | "disabled"
	BudgetTokens int   `json:"budget_tokens,omitempty"`
}

// Message 消息
type Message struct {
	Role       string        `json:"role"` // "user" | "assistant" | "system"
	Content    interface{}   `json:"content"` // string 或 []ContentBlock
	ToolCalls  []ToolCall    `json:"toolCalls,omitempty"`
	ToolCallID string        `json:"toolCallId,omitempty"` // 用于 tool result
}

// ContentBlock 内容块
type ContentBlock struct {
	Type      string     `json:"type"` // "text" | "image" | "tool_use" | "tool_result" | "thinking"
	Text      string     `json:"text,omitempty"`
	Image     *ImageData `json:"image,omitempty"`
	ToolUse   *ToolCall  `json:"toolUse,omitempty"`
	ToolResult *ToolResult `json:"toolResult,omitempty"`
	Thinking  string     `json:"thinking,omitempty"`
}

// ImageData 图片数据
type ImageData struct {
	Type      string `json:"type"` // "base64" | "url"
	MediaType string `json:"mediaType,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// Tool 工具定义
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"` // JSON Schema
}

// ToolCall 工具调用
type ToolCall struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"` // 已解析的参数
}

// ToolResult 工具执行结果
type ToolResult struct {
	ToolCallID string `json:"toolCallId"`
	Content    string `json:"content"`
	IsError    bool   `json:"isError,omitempty"`
}

// ChatResponse 对话响应
type ChatResponse struct {
	ID           string           `json:"id"`
	Model        string           `json:"model"`
	Content      string           `json:"content"`
	ToolCalls    []ToolCall       `json:"toolCalls,omitempty"`
	StopReason   string           `json:"stopReason"` // "end_turn" | "tool_use" | "max_tokens"
	Usage        Usage            `json:"usage"`
	Thinking     string           `json:"thinking,omitempty"`
}

// Usage 使用量统计
type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	CacheRead    int `json:"cacheReadTokens,omitempty"`
	CacheWrite   int `json:"cacheWriteTokens,omitempty"`
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type     StreamEventType `json:"type"`
	Content  string          `json:"content,omitempty"`
	ToolCall *ToolCall       `json:"toolCall,omitempty"`
	Thinking string          `json:"thinking,omitempty"`
	Usage    *Usage          `json:"usage,omitempty"`
	Error    error           `json:"error,omitempty"`
	Done     bool            `json:"done,omitempty"`
}

// StreamEventType 流式事件类型
type StreamEventType string

const (
	StreamEventStart    StreamEventType = "start"
	StreamEventDelta    StreamEventType = "delta"
	StreamEventToolCall StreamEventType = "tool_call"
	StreamEventThinking StreamEventType = "thinking"
	StreamEventUsage    StreamEventType = "usage"
	StreamEventError    StreamEventType = "error"
	StreamEventDone     StreamEventType = "done"
)

// ProviderRegistry 提供商注册表
type ProviderRegistry struct {
	providers map[string]Provider
}

// NewProviderRegistry 创建提供商注册表
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Provider),
	}
}

// Register 注册提供商
func (r *ProviderRegistry) Register(provider Provider) {
	r.providers[provider.ID()] = provider
}

// Get 获取提供商
func (r *ProviderRegistry) Get(id string) (Provider, bool) {
	p, ok := r.providers[id]
	return p, ok
}

// List 列出所有提供商
func (r *ProviderRegistry) List() []Provider {
	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	return providers
}

// ResolveModel 解析模型 ID (provider/model)
func ResolveModel(modelID string) (provider, model string) {
	for i := 0; i < len(modelID); i++ {
		if modelID[i] == '/' {
			return modelID[:i], modelID[i+1:]
		}
	}
	return "", modelID
}
