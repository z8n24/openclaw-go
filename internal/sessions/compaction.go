package sessions

import (
	"context"
	"fmt"
	"strings"

	"github.com/user/openclaw-go/internal/agents"
)

// Compactor 会话压缩器
type Compactor struct {
	provider agents.Provider
	model    string
}

// NewCompactor 创建压缩器
func NewCompactor(provider agents.Provider, model string) *Compactor {
	return &Compactor{
		provider: provider,
		model:    model,
	}
}

// CompactSession 压缩会话
func (c *Compactor) CompactSession(ctx context.Context, session *EnhancedSession, keepCount int) error {
	if !session.NeedsCompaction(keepCount * 2) {
		return nil // 不需要压缩
	}
	
	return session.Compact(func(messages []agents.Message) (string, error) {
		return c.GenerateSummary(ctx, messages)
	}, keepCount)
}

// GenerateSummary 使用 LLM 生成对话摘要
func (c *Compactor) GenerateSummary(ctx context.Context, messages []agents.Message) (string, error) {
	if c.provider == nil {
		// 如果没有 provider，使用简单摘要
		return c.simpleSummary(messages), nil
	}
	
	// 构建摘要请求
	conversationText := formatConversation(messages)
	
	req := &agents.ChatRequest{
		Model: c.model,
		System: `You are a conversation summarizer. Your task is to create a concise but comprehensive summary of the conversation that preserves:
1. Key decisions and conclusions
2. Important facts and information shared
3. Action items or tasks mentioned
4. User preferences or requirements expressed
5. Technical details that might be referenced later

Write the summary in a neutral, factual tone. Be concise but don't omit important details.`,
		Messages: []agents.Message{
			{
				Role:    "user",
				Content: fmt.Sprintf("Please summarize the following conversation:\n\n%s", conversationText),
			},
		},
		MaxTokens: 2000,
	}
	
	resp, err := c.provider.Chat(ctx, req)
	if err != nil {
		// 如果 LLM 失败，回退到简单摘要
		return c.simpleSummary(messages), nil
	}
	
	return resp.Content, nil
}

// simpleSummary 简单摘要 (不使用 LLM)
func (c *Compactor) simpleSummary(messages []agents.Message) string {
	var parts []string
	
	// 统计
	userMsgCount := 0
	assistantMsgCount := 0
	toolCalls := 0
	
	// 提取关键内容
	var userTopics []string
	
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			userMsgCount++
			// 提取用户消息的前 100 个字符作为主题
			if content, ok := msg.Content.(string); ok && len(content) > 0 {
				topic := content
				if len(topic) > 100 {
					topic = topic[:100] + "..."
				}
				// 只保留前几个主题
				if len(userTopics) < 5 {
					userTopics = append(userTopics, topic)
				}
			}
		case "assistant":
			assistantMsgCount++
			// 检查工具调用
			if blocks, ok := msg.Content.([]agents.ContentBlock); ok {
				for _, b := range blocks {
					if b.Type == "tool_use" {
						toolCalls++
					}
				}
			}
		}
	}
	
	// 构建摘要
	parts = append(parts, fmt.Sprintf("Conversation overview: %d user messages, %d assistant responses, %d tool calls.",
		userMsgCount, assistantMsgCount, toolCalls))
	
	if len(userTopics) > 0 {
		parts = append(parts, "\nKey topics discussed:")
		for i, topic := range userTopics {
			parts = append(parts, fmt.Sprintf("%d. %s", i+1, topic))
		}
	}
	
	return strings.Join(parts, "\n")
}

// formatConversation 格式化对话用于摘要
func formatConversation(messages []agents.Message) string {
	var parts []string
	
	for _, msg := range messages {
		var content string
		
		switch c := msg.Content.(type) {
		case string:
			content = c
		case []agents.ContentBlock:
			var textParts []string
			for _, b := range c {
				switch b.Type {
				case "text":
					textParts = append(textParts, b.Text)
				case "tool_use":
					if b.ToolUse != nil {
						textParts = append(textParts, fmt.Sprintf("[Tool: %s]", b.ToolUse.Name))
					}
				case "tool_result":
					if b.ToolResult != nil {
						result := b.ToolResult.Content
						if len(result) > 200 {
							result = result[:200] + "..."
						}
						textParts = append(textParts, fmt.Sprintf("[Result: %s]", result))
					}
				}
			}
			content = strings.Join(textParts, " ")
		}
		
		if content == "" {
			continue
		}
		
		// 截断过长的内容
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		
		role := msg.Role
		if role == "user" {
			role = "User"
		} else if role == "assistant" {
			role = "Assistant"
		}
		
		parts = append(parts, fmt.Sprintf("%s: %s", role, content))
	}
	
	return strings.Join(parts, "\n\n")
}

// AutoCompactionConfig 自动压缩配置
type AutoCompactionConfig struct {
	Enabled         bool   // 是否启用自动压缩
	Threshold       int    // 触发压缩的消息数
	KeepCount       int    // 保留的最近消息数
	Model           string // 用于生成摘要的模型
	UseSimpleSummary bool  // 使用简单摘要而非 LLM
}

// DefaultAutoCompactionConfig 默认自动压缩配置
var DefaultAutoCompactionConfig = AutoCompactionConfig{
	Enabled:          true,
	Threshold:        50,
	KeepCount:        10,
	Model:            "claude-3-haiku", // 使用便宜的模型
	UseSimpleSummary: false,
}

// CompactionResult 压缩结果
type CompactionResult struct {
	SessionKey      string `json:"sessionKey"`
	CompactedCount  int    `json:"compactedCount"`
	KeptCount       int    `json:"keptCount"`
	SummaryLength   int    `json:"summaryLength"`
	TokensSaved     int    `json:"tokensSaved,omitempty"`
}

// EstimateTokens 估算消息的 token 数量
func EstimateTokens(messages []agents.Message) int {
	// 简单估算: 平均每 4 个字符 1 个 token
	totalChars := 0
	
	for _, msg := range messages {
		switch c := msg.Content.(type) {
		case string:
			totalChars += len(c)
		case []agents.ContentBlock:
			for _, b := range c {
				if b.Type == "text" {
					totalChars += len(b.Text)
				}
			}
		}
	}
	
	return totalChars / 4
}
