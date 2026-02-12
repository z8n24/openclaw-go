package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/user/openclaw-go/internal/agents"
)

const (
	DefaultBaseURL = "https://api.anthropic.com"
	APIVersion     = "2023-06-01"
)

// Client Anthropic API 客户端
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient 创建 Anthropic 客户端
func NewClient(apiKey string) *Client {
	apiKey = GetAPIKey(apiKey)
	return &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{},
	}
}

func (c *Client) ID() string {
	return "anthropic"
}

func (c *Client) Name() string {
	return "Anthropic"
}

func (c *Client) ListModels() []agents.ModelInfo {
	return []agents.ModelInfo{
		{ID: "claude-opus-4-5-20250514", Name: "Claude Opus 4.5", Provider: "anthropic", ContextWindow: 200000, MaxOutput: 32000, SupportsTools: true, SupportsVision: true},
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Provider: "anthropic", ContextWindow: 200000, MaxOutput: 64000, SupportsTools: true, SupportsVision: true},
		{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Provider: "anthropic", ContextWindow: 200000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
	}
}

// API 请求/响应结构

type apiRequest struct {
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens"`
	System      string        `json:"system,omitempty"`
	Messages    []apiMessage  `json:"messages"`
	Tools       []apiTool     `json:"tools,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	Thinking    *apiThinking  `json:"thinking,omitempty"`
}

type apiThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

type apiMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string 或 []apiContent
}

type apiContent struct {
	Type      string    `json:"type"`
	Text      string    `json:"text,omitempty"`
	ID        string    `json:"id,omitempty"`
	Name      string    `json:"name,omitempty"`
	Input     any       `json:"input,omitempty"`
	ToolUseID string    `json:"tool_use_id,omitempty"`
	Content   string    `json:"content,omitempty"`
	Thinking  string    `json:"thinking,omitempty"`
	Source    *apiImage `json:"source,omitempty"`
}

type apiImage struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type apiTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

type apiResponse struct {
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	Role         string       `json:"role"`
	Content      []apiContent `json:"content"`
	Model        string       `json:"model"`
	StopReason   string       `json:"stop_reason"`
	StopSequence string       `json:"stop_sequence,omitempty"`
	Usage        apiUsage     `json:"usage"`
}

type apiUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// Chat 非流式对话
func (c *Client) Chat(ctx context.Context, req *agents.ChatRequest) (*agents.ChatResponse, error) {
	apiReq := c.buildRequest(req, false)
	
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
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
	
	var apiResp apiResponse
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
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
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
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer
	
	var currentToolCall *agents.ToolCall
	var toolInputBuf strings.Builder
	
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
		
		var event streamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		
		switch event.Type {
		case "message_start":
			ch <- agents.StreamEvent{Type: agents.StreamEventStart}
			
		case "content_block_start":
			if event.ContentBlock != nil {
				switch event.ContentBlock.Type {
				case "tool_use":
					currentToolCall = &agents.ToolCall{
						ID:   event.ContentBlock.ID,
						Name: event.ContentBlock.Name,
					}
					toolInputBuf.Reset()
				case "thinking":
					// thinking block started
				}
			}
			
		case "content_block_delta":
			if event.Delta != nil {
				switch event.Delta.Type {
				case "text_delta":
					ch <- agents.StreamEvent{
						Type:    agents.StreamEventDelta,
						Content: event.Delta.Text,
					}
				case "input_json_delta":
					toolInputBuf.WriteString(event.Delta.PartialJSON)
				case "thinking_delta":
					ch <- agents.StreamEvent{
						Type:     agents.StreamEventThinking,
						Thinking: event.Delta.Thinking,
					}
				}
			}
			
		case "content_block_stop":
			if currentToolCall != nil {
				// 解析完整的 tool input
				var args interface{}
				if err := json.Unmarshal([]byte(toolInputBuf.String()), &args); err == nil {
					currentToolCall.Arguments = args
				} else {
					currentToolCall.Arguments = toolInputBuf.String()
				}
				ch <- agents.StreamEvent{
					Type:     agents.StreamEventToolCall,
					ToolCall: currentToolCall,
				}
				currentToolCall = nil
			}
			
		case "message_delta":
			if event.Usage != nil {
				ch <- agents.StreamEvent{
					Type: agents.StreamEventUsage,
					Usage: &agents.Usage{
						InputTokens:  event.Usage.InputTokens,
						OutputTokens: event.Usage.OutputTokens,
					},
				}
			}
			
		case "message_stop":
			ch <- agents.StreamEvent{Type: agents.StreamEventDone, Done: true}
		}
	}
}

type streamEvent struct {
	Type         string        `json:"type"`
	Message      *apiResponse  `json:"message,omitempty"`
	Index        int           `json:"index,omitempty"`
	ContentBlock *apiContent   `json:"content_block,omitempty"`
	Delta        *streamDelta  `json:"delta,omitempty"`
	Usage        *apiUsage     `json:"usage,omitempty"`
}

type streamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
}

func (c *Client) buildRequest(req *agents.ChatRequest, stream bool) *apiRequest {
	apiReq := &apiRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		System:    req.System,
		Stream:    stream,
	}
	
	if apiReq.MaxTokens == 0 {
		apiReq.MaxTokens = 8192
	}
	
	if req.Temperature != nil {
		apiReq.Temperature = req.Temperature
	}
	
	if req.Thinking != nil && req.Thinking.Type == "enabled" {
		apiReq.Thinking = &apiThinking{
			Type:         "enabled",
			BudgetTokens: req.Thinking.BudgetTokens,
		}
	}
	
	// 转换消息
	for _, msg := range req.Messages {
		apiMsg := apiMessage{Role: msg.Role}
		
		switch content := msg.Content.(type) {
		case string:
			apiMsg.Content = content
		case []agents.ContentBlock:
			var blocks []apiContent
			for _, b := range content {
				switch b.Type {
				case "text":
					blocks = append(blocks, apiContent{Type: "text", Text: b.Text})
				case "image":
					if b.Image != nil {
						blocks = append(blocks, apiContent{
							Type: "image",
							Source: &apiImage{
								Type:      b.Image.Type,
								MediaType: b.Image.MediaType,
								Data:      b.Image.Data,
							},
						})
					}
				case "tool_use":
					if b.ToolUse != nil {
						blocks = append(blocks, apiContent{
							Type:  "tool_use",
							ID:    b.ToolUse.ID,
							Name:  b.ToolUse.Name,
							Input: b.ToolUse.Arguments,
						})
					}
				case "tool_result":
					if b.ToolResult != nil {
						blocks = append(blocks, apiContent{
							Type:      "tool_result",
							ToolUseID: b.ToolResult.ToolCallID,
							Content:   b.ToolResult.Content,
						})
					}
				}
			}
			apiMsg.Content = blocks
		}
		
		apiReq.Messages = append(apiReq.Messages, apiMsg)
	}
	
	// 转换工具
	for _, tool := range req.Tools {
		apiReq.Tools = append(apiReq.Tools, apiTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.Parameters,
		})
	}
	
	return apiReq
}

func (c *Client) parseResponse(resp *apiResponse) *agents.ChatResponse {
	result := &agents.ChatResponse{
		ID:         resp.ID,
		Model:      resp.Model,
		StopReason: resp.StopReason,
		Usage: agents.Usage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			CacheRead:    resp.Usage.CacheReadInputTokens,
			CacheWrite:   resp.Usage.CacheCreationInputTokens,
		},
	}
	
	var textParts []string
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "thinking":
			result.Thinking = block.Thinking
		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, agents.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}
	result.Content = strings.Join(textParts, "")
	
	return result
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", APIVersion)
}
