package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/user/openclaw-go/internal/agents"
)

const (
	DefaultBaseURL = "https://api.openai.com"
)

// Client OpenAI API 客户端
type Client struct {
	apiKey     string
	baseURL    string
	orgID      string
	httpClient *http.Client
}

// Config 客户端配置
type Config struct {
	APIKey  string
	BaseURL string
	OrgID   string
}

// NewClient 创建 OpenAI 客户端
func NewClient(cfg Config) *Client {
	apiKey := GetAPIKey(cfg.APIKey)
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		orgID:      cfg.OrgID,
		httpClient: &http.Client{},
	}
}

// NewClientSimple 简单创建客户端
func NewClientSimple(apiKey string) *Client {
	return NewClient(Config{APIKey: apiKey})
}

func (c *Client) ID() string {
	return "openai"
}

func (c *Client) Name() string {
	return "OpenAI"
}

func (c *Client) ListModels() []agents.ModelInfo {
	return []agents.ModelInfo{
		// GPT-4o family
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", ContextWindow: 128000, MaxOutput: 16384, SupportsTools: true, SupportsVision: true},
		{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openai", ContextWindow: 128000, MaxOutput: 16384, SupportsTools: true, SupportsVision: true},
		{ID: "gpt-4o-2024-11-20", Name: "GPT-4o (Nov 2024)", Provider: "openai", ContextWindow: 128000, MaxOutput: 16384, SupportsTools: true, SupportsVision: true},
		
		// o1/o3 reasoning models
		{ID: "o1", Name: "o1", Provider: "openai", ContextWindow: 200000, MaxOutput: 100000, SupportsTools: true, SupportsVision: true},
		{ID: "o1-mini", Name: "o1 Mini", Provider: "openai", ContextWindow: 128000, MaxOutput: 65536, SupportsTools: true, SupportsVision: false},
		{ID: "o1-preview", Name: "o1 Preview", Provider: "openai", ContextWindow: 128000, MaxOutput: 32768, SupportsTools: false, SupportsVision: false},
		{ID: "o3-mini", Name: "o3 Mini", Provider: "openai", ContextWindow: 200000, MaxOutput: 100000, SupportsTools: true, SupportsVision: false},
		
		// GPT-4 Turbo
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Provider: "openai", ContextWindow: 128000, MaxOutput: 4096, SupportsTools: true, SupportsVision: true},
		{ID: "gpt-4-turbo-preview", Name: "GPT-4 Turbo Preview", Provider: "openai", ContextWindow: 128000, MaxOutput: 4096, SupportsTools: true, SupportsVision: false},
		
		// GPT-4
		{ID: "gpt-4", Name: "GPT-4", Provider: "openai", ContextWindow: 8192, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
		{ID: "gpt-4-32k", Name: "GPT-4 32K", Provider: "openai", ContextWindow: 32768, MaxOutput: 32768, SupportsTools: true, SupportsVision: false},
		
		// GPT-3.5
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Provider: "openai", ContextWindow: 16385, MaxOutput: 4096, SupportsTools: true, SupportsVision: false},
	}
}

// OpenAI API 请求/响应结构
type chatRequest struct {
	Model            string        `json:"model"`
	Messages         []chatMessage `json:"messages"`
	Tools            []chatTool    `json:"tools,omitempty"`
	MaxTokens        int           `json:"max_tokens,omitempty"`
	MaxCompletionTokens int        `json:"max_completion_tokens,omitempty"` // 用于 o1/o3 系列
	Temperature      *float64      `json:"temperature,omitempty"`
	Stream           bool          `json:"stream,omitempty"`
	StreamOptions    *streamOpts   `json:"stream_options,omitempty"`
	ReasoningEffort  string        `json:"reasoning_effort,omitempty"` // o1/o3: low, medium, high
}

type streamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // auto, low, high
}

type chatTool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
	Strict      bool   `json:"strict,omitempty"`
}

type toolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		// o1/o3 reasoning tokens
		CompletionTokensDetails *struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details,omitempty"`
	} `json:"usage"`
}

type streamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string     `json:"role,omitempty"`
			Content   string     `json:"content,omitempty"`
			ToolCalls []toolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
}

// Chat 非流式对话
func (c *Client) Chat(ctx context.Context, req *agents.ChatRequest) (*agents.ChatResponse, error) {
	apiReq := c.buildRequest(req, false)

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return c.parseResponse(&apiResp), nil
}

// ChatStream 流式对话
func (c *Client) ChatStream(ctx context.Context, req *agents.ChatRequest) (<-chan agents.StreamEvent, error) {
	apiReq := c.buildRequest(req, true)

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan agents.StreamEvent, 100)
	go c.readStream(ctx, resp.Body, ch)

	return ch, nil
}

func (c *Client) readStream(ctx context.Context, body io.ReadCloser, ch chan<- agents.StreamEvent) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	toolCalls := make(map[int]*agents.ToolCall)
	toolArgsBuffers := make(map[int]*strings.Builder)

	ch <- agents.StreamEvent{Type: agents.StreamEventStart}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// 处理 usage (最后一个 chunk)
		if chunk.Usage != nil {
			ch <- agents.StreamEvent{
				Type: agents.StreamEventUsage,
				Usage: &agents.Usage{
					InputTokens:  chunk.Usage.PromptTokens,
					OutputTokens: chunk.Usage.CompletionTokens,
				},
			}
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		// 处理文本内容
		if delta.Content != "" {
			ch <- agents.StreamEvent{
				Type:    agents.StreamEventDelta,
				Content: delta.Content,
			}
		}

		// 处理 tool calls
		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			if _, ok := toolCalls[idx]; !ok {
				toolCalls[idx] = &agents.ToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
				toolArgsBuffers[idx] = &strings.Builder{}
			}
			if tc.Function.Name != "" {
				toolCalls[idx].Name = tc.Function.Name
			}
			if tc.ID != "" {
				toolCalls[idx].ID = tc.ID
			}
			toolArgsBuffers[idx].WriteString(tc.Function.Arguments)
		}

		// 完成时发送 tool calls
		if chunk.Choices[0].FinishReason == "tool_calls" || chunk.Choices[0].FinishReason == "stop" {
			for idx, tc := range toolCalls {
				argsStr := toolArgsBuffers[idx].String()
				var args interface{}
				if err := json.Unmarshal([]byte(argsStr), &args); err == nil {
					tc.Arguments = args
				} else {
					tc.Arguments = argsStr
				}
				ch <- agents.StreamEvent{
					Type:     agents.StreamEventToolCall,
					ToolCall: tc,
				}
			}
			// 清空以防后续还有
			toolCalls = make(map[int]*agents.ToolCall)
			toolArgsBuffers = make(map[int]*strings.Builder)
		}
	}

	ch <- agents.StreamEvent{Type: agents.StreamEventDone, Done: true}
}

func (c *Client) buildRequest(req *agents.ChatRequest, stream bool) *chatRequest {
	apiReq := &chatRequest{
		Model:  req.Model,
		Stream: stream,
	}

	// 判断是否是 o1/o3 系列模型
	isReasoningModel := strings.HasPrefix(req.Model, "o1") || strings.HasPrefix(req.Model, "o3")

	if isReasoningModel {
		// o1/o3 使用 max_completion_tokens
		if req.MaxTokens > 0 {
			apiReq.MaxCompletionTokens = req.MaxTokens
		} else {
			apiReq.MaxCompletionTokens = 16384
		}
		// 设置 reasoning effort (默认 medium)
		if req.Thinking != nil && req.Thinking.Type == "enabled" {
			if req.Thinking.BudgetTokens > 50000 {
				apiReq.ReasoningEffort = "high"
			} else if req.Thinking.BudgetTokens > 10000 {
				apiReq.ReasoningEffort = "medium"
			} else {
				apiReq.ReasoningEffort = "low"
			}
		}
	} else {
		if req.MaxTokens > 0 {
			apiReq.MaxTokens = req.MaxTokens
		} else {
			apiReq.MaxTokens = 16384
		}
		if req.Temperature != nil {
			apiReq.Temperature = req.Temperature
		}
	}

	if stream {
		apiReq.StreamOptions = &streamOpts{IncludeUsage: true}
	}

	// 添加 system 消息 (o1/o3 不支持 system role，需要放到 user 消息中)
	if req.System != "" {
		if isReasoningModel {
			// o1/o3: 将 system 放到 developer 消息中 (如果支持) 或 user 消息
			apiReq.Messages = append(apiReq.Messages, chatMessage{
				Role:    "user", // 或 "developer" 如果 API 支持
				Content: "[System Instructions]\n" + req.System,
			})
		} else {
			apiReq.Messages = append(apiReq.Messages, chatMessage{
				Role:    "system",
				Content: req.System,
			})
		}
	}

	// 转换消息
	for _, msg := range req.Messages {
		apiMsg := chatMessage{Role: msg.Role}

		switch content := msg.Content.(type) {
		case string:
			apiMsg.Content = content
		case []agents.ContentBlock:
			apiMsg = c.convertContentBlocks(content, msg.Role, apiReq)
		}

		if apiMsg.Content != nil || len(apiMsg.ToolCalls) > 0 {
			apiReq.Messages = append(apiReq.Messages, apiMsg)
		}
	}

	// 转换工具 (o1-preview 不支持工具)
	if !strings.HasPrefix(req.Model, "o1-preview") {
		for _, tool := range req.Tools {
			apiReq.Tools = append(apiReq.Tools, chatTool{
				Type: "function",
				Function: toolFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			})
		}
	}

	return apiReq
}

func (c *Client) convertContentBlocks(blocks []agents.ContentBlock, role string, apiReq *chatRequest) chatMessage {
	apiMsg := chatMessage{Role: role}

	var parts []contentPart
	var toolResults []chatMessage

	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, contentPart{Type: "text", Text: b.Text})
		case "image":
			if b.Image != nil {
				var url string
				if b.Image.Type == "base64" {
					url = fmt.Sprintf("data:%s;base64,%s", b.Image.MediaType, b.Image.Data)
				} else {
					url = b.Image.URL
				}
				parts = append(parts, contentPart{
					Type:     "image_url",
					ImageURL: &imageURL{URL: url, Detail: "auto"},
				})
			}
		case "tool_use":
			if b.ToolUse != nil {
				argsStr, _ := json.Marshal(b.ToolUse.Arguments)
				apiMsg.ToolCalls = append(apiMsg.ToolCalls, toolCall{
					ID:   b.ToolUse.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      b.ToolUse.Name,
						Arguments: string(argsStr),
					},
				})
			}
		case "tool_result":
			if b.ToolResult != nil {
				toolResults = append(toolResults, chatMessage{
					Role:       "tool",
					Content:    b.ToolResult.Content,
					ToolCallID: b.ToolResult.ToolCallID,
				})
			}
		}
	}

	// 设置内容
	if len(parts) == 1 && parts[0].Type == "text" {
		apiMsg.Content = parts[0].Text
	} else if len(parts) > 0 {
		apiMsg.Content = parts
	}

	// 添加 tool results 作为单独消息
	if len(toolResults) > 0 && apiMsg.Content == nil && len(apiMsg.ToolCalls) == 0 {
		for _, tr := range toolResults {
			apiReq.Messages = append(apiReq.Messages, tr)
		}
		return chatMessage{} // 返回空消息，不添加
	}

	return apiMsg
}

func (c *Client) parseResponse(resp *chatResponse) *agents.ChatResponse {
	result := &agents.ChatResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Usage: agents.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		result.StopReason = choice.FinishReason

		if content, ok := choice.Message.Content.(string); ok {
			result.Content = content
		}

		for _, tc := range choice.Message.ToolCalls {
			var args interface{}
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			result.ToolCalls = append(result.ToolCalls, agents.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}

	return result
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if c.orgID != "" {
		req.Header.Set("OpenAI-Organization", c.orgID)
	}
}

// GetAPIKey 获取 OpenAI API Key
func GetAPIKey(explicit string) string {
	if explicit != "" {
		return explicit
	}

	// 环境变量
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}

	return ""
}
