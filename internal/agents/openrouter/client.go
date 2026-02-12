package openrouter

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

	"github.com/z8n24/openclaw-go/internal/agents"
)

const (
	DefaultBaseURL = "https://openrouter.ai/api"
)

// Client OpenRouter API 客户端
type Client struct {
	apiKey     string
	baseURL    string
	siteURL    string  // 用于排行榜显示
	siteName   string  // 用于排行榜显示
	httpClient *http.Client
}

// Config 客户端配置
type Config struct {
	APIKey   string
	BaseURL  string
	SiteURL  string
	SiteName string
}

// NewClient 创建 OpenRouter 客户端
func NewClient(cfg Config) *Client {
	apiKey := GetAPIKey(cfg.APIKey)
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		siteURL:    cfg.SiteURL,
		siteName:   cfg.SiteName,
		httpClient: &http.Client{},
	}
}

// NewClientSimple 简单创建客户端
func NewClientSimple(apiKey string) *Client {
	return NewClient(Config{APIKey: apiKey})
}

func (c *Client) ID() string {
	return "openrouter"
}

func (c *Client) Name() string {
	return "OpenRouter"
}

func (c *Client) ListModels() []agents.ModelInfo {
	// OpenRouter 支持众多模型，这里列出常用的
	return []agents.ModelInfo{
		// Anthropic
		{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", Provider: "openrouter", ContextWindow: 200000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
		{ID: "anthropic/claude-3-opus", Name: "Claude 3 Opus", Provider: "openrouter", ContextWindow: 200000, MaxOutput: 4096, SupportsTools: true, SupportsVision: true},
		{ID: "anthropic/claude-3-haiku", Name: "Claude 3 Haiku", Provider: "openrouter", ContextWindow: 200000, MaxOutput: 4096, SupportsTools: true, SupportsVision: true},
		
		// OpenAI
		{ID: "openai/gpt-4o", Name: "GPT-4o", Provider: "openrouter", ContextWindow: 128000, MaxOutput: 16384, SupportsTools: true, SupportsVision: true},
		{ID: "openai/gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openrouter", ContextWindow: 128000, MaxOutput: 16384, SupportsTools: true, SupportsVision: true},
		{ID: "openai/o1", Name: "o1", Provider: "openrouter", ContextWindow: 200000, MaxOutput: 100000, SupportsTools: true, SupportsVision: true},
		{ID: "openai/o1-mini", Name: "o1 Mini", Provider: "openrouter", ContextWindow: 128000, MaxOutput: 65536, SupportsTools: true, SupportsVision: false},
		
		// Google
		{ID: "google/gemini-2.0-flash-exp", Name: "Gemini 2.0 Flash", Provider: "openrouter", ContextWindow: 1000000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
		{ID: "google/gemini-pro-1.5", Name: "Gemini 1.5 Pro", Provider: "openrouter", ContextWindow: 2000000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
		
		// Meta
		{ID: "meta-llama/llama-3.3-70b-instruct", Name: "Llama 3.3 70B", Provider: "openrouter", ContextWindow: 131072, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
		{ID: "meta-llama/llama-3.1-405b-instruct", Name: "Llama 3.1 405B", Provider: "openrouter", ContextWindow: 131072, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
		
		// Mistral
		{ID: "mistralai/mistral-large-2411", Name: "Mistral Large", Provider: "openrouter", ContextWindow: 131072, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
		{ID: "mistralai/codestral-2501", Name: "Codestral", Provider: "openrouter", ContextWindow: 256000, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
		
		// DeepSeek
		{ID: "deepseek/deepseek-chat", Name: "DeepSeek Chat", Provider: "openrouter", ContextWindow: 64000, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
		{ID: "deepseek/deepseek-r1", Name: "DeepSeek R1", Provider: "openrouter", ContextWindow: 64000, MaxOutput: 8192, SupportsTools: false, SupportsVision: false},
		
		// Qwen
		{ID: "qwen/qwen-2.5-72b-instruct", Name: "Qwen 2.5 72B", Provider: "openrouter", ContextWindow: 131072, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
		{ID: "qwen/qwq-32b-preview", Name: "QwQ 32B", Provider: "openrouter", ContextWindow: 32768, MaxOutput: 8192, SupportsTools: false, SupportsVision: false},
		
		// Free models
		{ID: "meta-llama/llama-3.2-3b-instruct:free", Name: "Llama 3.2 3B (Free)", Provider: "openrouter", ContextWindow: 131072, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
		{ID: "google/gemma-2-9b-it:free", Name: "Gemma 2 9B (Free)", Provider: "openrouter", ContextWindow: 8192, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
	}
}

// OpenAI 兼容的请求结构
type chatRequest struct {
	Model         string        `json:"model"`
	Messages      []chatMessage `json:"messages"`
	Tools         []chatTool    `json:"tools,omitempty"`
	MaxTokens     int           `json:"max_tokens,omitempty"`
	Temperature   *float64      `json:"temperature,omitempty"`
	Stream        bool          `json:"stream,omitempty"`
	
	// OpenRouter specific
	Transforms    []string      `json:"transforms,omitempty"`
	RouteType     string        `json:"route,omitempty"` // fallback
	Provider      *providerPrefs `json:"provider,omitempty"`
}

type providerPrefs struct {
	Order         []string `json:"order,omitempty"`
	AllowFallback bool     `json:"allow_fallback,omitempty"`
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
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
	} `json:"usage"`
}

type streamChunk struct {
	ID      string `json:"id"`
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

		if delta.Content != "" {
			ch <- agents.StreamEvent{
				Type:    agents.StreamEventDelta,
				Content: delta.Content,
			}
		}

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

	if req.MaxTokens > 0 {
		apiReq.MaxTokens = req.MaxTokens
	} else {
		apiReq.MaxTokens = 16384
	}

	if req.Temperature != nil {
		apiReq.Temperature = req.Temperature
	}

	// 添加 system 消息
	if req.System != "" {
		apiReq.Messages = append(apiReq.Messages, chatMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	// 转换消息
	for _, msg := range req.Messages {
		apiMsg := chatMessage{Role: msg.Role}

		switch content := msg.Content.(type) {
		case string:
			apiMsg.Content = content
		case []agents.ContentBlock:
			var parts []contentPart
			var toolResults []chatMessage

			for _, b := range content {
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

			if len(parts) == 1 && parts[0].Type == "text" {
				apiMsg.Content = parts[0].Text
			} else if len(parts) > 0 {
				apiMsg.Content = parts
			}

			if len(toolResults) > 0 && apiMsg.Content == nil && len(apiMsg.ToolCalls) == 0 {
				for _, tr := range toolResults {
					apiReq.Messages = append(apiReq.Messages, tr)
				}
				continue
			}
		}

		if apiMsg.Content != nil || len(apiMsg.ToolCalls) > 0 {
			apiReq.Messages = append(apiReq.Messages, apiMsg)
		}
	}

	// 转换工具
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

	return apiReq
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
	req.Header.Set("HTTP-Referer", c.siteURL)
	req.Header.Set("X-Title", c.siteName)
}

// GetAPIKey 获取 OpenRouter API Key
func GetAPIKey(explicit string) string {
	if explicit != "" {
		return explicit
	}

	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		return key
	}

	return ""
}
