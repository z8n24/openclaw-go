package deepseek

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
	DefaultBaseURL = "https://api.deepseek.com"
)

// Client DeepSeek API 客户端 (OpenAI 兼容)
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient 创建 DeepSeek 客户端
func NewClient(apiKey string) *Client {
	apiKey = GetAPIKey(apiKey)
	return &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{},
	}
}

func (c *Client) ID() string {
	return "deepseek"
}

func (c *Client) Name() string {
	return "DeepSeek"
}

func (c *Client) ListModels() []agents.ModelInfo {
	return []agents.ModelInfo{
		{ID: "deepseek-chat", Name: "DeepSeek Chat", Provider: "deepseek", ContextWindow: 64000, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
		{ID: "deepseek-coder", Name: "DeepSeek Coder", Provider: "deepseek", ContextWindow: 64000, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
		{ID: "deepseek-reasoner", Name: "DeepSeek R1", Provider: "deepseek", ContextWindow: 64000, MaxOutput: 8192, SupportsTools: false, SupportsVision: false},
	}
}

// OpenAI 兼容的请求结构
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Tools       []chatTool    `json:"tools,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"` // string 或 []contentPart
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
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
	
	// 用于收集 tool calls
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
		}
	}
	
	ch <- agents.StreamEvent{Type: agents.StreamEventDone, Done: true}
}

func (c *Client) buildRequest(req *agents.ChatRequest, stream bool) *chatRequest {
	apiReq := &chatRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		Stream:    stream,
	}
	
	if apiReq.MaxTokens == 0 {
		apiReq.MaxTokens = 8192
	} else if apiReq.MaxTokens > 8192 {
		// DeepSeek 最大支持 8192 tokens
		apiReq.MaxTokens = 8192
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
			// 处理 content blocks
			var textParts []string
			var toolResults []chatMessage
			
			for _, b := range content {
				switch b.Type {
				case "text":
					textParts = append(textParts, b.Text)
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
			
			if len(textParts) > 0 {
				apiMsg.Content = strings.Join(textParts, "")
			}
			
			// 如果只有 tool results，直接添加它们
			if len(toolResults) > 0 && len(textParts) == 0 && len(apiMsg.ToolCalls) == 0 {
				for _, tr := range toolResults {
					apiReq.Messages = append(apiReq.Messages, tr)
				}
				continue
			}
		}
		
		apiReq.Messages = append(apiReq.Messages, apiMsg)
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
}

// GetAPIKey 获取 DeepSeek API Key
func GetAPIKey(explicit string) string {
	if explicit != "" {
		return explicit
	}
	
	// 环境变量
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		return key
	}
	
	return ""
}
