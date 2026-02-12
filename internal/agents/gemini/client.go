package gemini

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
	DefaultBaseURL = "https://generativelanguage.googleapis.com"
)

// Client Google Gemini API 客户端
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Config 客户端配置
type Config struct {
	APIKey  string
	BaseURL string
}

// NewClient 创建 Gemini 客户端
func NewClient(cfg Config) *Client {
	apiKey := GetAPIKey(cfg.APIKey)
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// NewClientSimple 简单创建客户端
func NewClientSimple(apiKey string) *Client {
	return NewClient(Config{APIKey: apiKey})
}

func (c *Client) ID() string {
	return "google"
}

func (c *Client) Name() string {
	return "Google Gemini"
}

func (c *Client) ListModels() []agents.ModelInfo {
	return []agents.ModelInfo{
		// Gemini 2.0
		{ID: "gemini-2.0-flash-exp", Name: "Gemini 2.0 Flash", Provider: "google", ContextWindow: 1000000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
		{ID: "gemini-2.0-flash-thinking-exp", Name: "Gemini 2.0 Flash Thinking", Provider: "google", ContextWindow: 1000000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
		
		// Gemini 1.5
		{ID: "gemini-1.5-pro", Name: "Gemini 1.5 Pro", Provider: "google", ContextWindow: 2000000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
		{ID: "gemini-1.5-pro-latest", Name: "Gemini 1.5 Pro Latest", Provider: "google", ContextWindow: 2000000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
		{ID: "gemini-1.5-flash", Name: "Gemini 1.5 Flash", Provider: "google", ContextWindow: 1000000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
		{ID: "gemini-1.5-flash-latest", Name: "Gemini 1.5 Flash Latest", Provider: "google", ContextWindow: 1000000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
		{ID: "gemini-1.5-flash-8b", Name: "Gemini 1.5 Flash 8B", Provider: "google", ContextWindow: 1000000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
		
		// Gemini 1.0
		{ID: "gemini-pro", Name: "Gemini Pro", Provider: "google", ContextWindow: 32760, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
		{ID: "gemini-pro-vision", Name: "Gemini Pro Vision", Provider: "google", ContextWindow: 16384, MaxOutput: 4096, SupportsTools: false, SupportsVision: true},
	}
}

// Gemini API 请求/响应结构
type generateRequest struct {
	Contents          []content        `json:"contents"`
	SystemInstruction *content         `json:"systemInstruction,omitempty"`
	Tools             []tool           `json:"tools,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text         string        `json:"text,omitempty"`
	InlineData   *inlineData   `json:"inlineData,omitempty"`
	FunctionCall *functionCall `json:"functionCall,omitempty"`
	FunctionResponse *functionResponse `json:"functionResponse,omitempty"`
}

type inlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64
}

type functionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type functionResponse struct {
	Name     string `json:"name"`
	Response any    `json:"response"`
}

type tool struct {
	FunctionDeclarations []functionDeclaration `json:"functionDeclarations,omitempty"`
}

type functionDeclaration struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters,omitempty"`
}

type generationConfig struct {
	Temperature      *float64 `json:"temperature,omitempty"`
	MaxOutputTokens  int      `json:"maxOutputTokens,omitempty"`
	StopSequences    []string `json:"stopSequences,omitempty"`
	CandidateCount   int      `json:"candidateCount,omitempty"`
	ResponseMimeType string   `json:"responseMimeType,omitempty"`
}

type generateResponse struct {
	Candidates     []candidate    `json:"candidates"`
	UsageMetadata  usageMetadata  `json:"usageMetadata"`
	PromptFeedback *promptFeedback `json:"promptFeedback,omitempty"`
}

type candidate struct {
	Content       content        `json:"content"`
	FinishReason  string         `json:"finishReason"`
	SafetyRatings []safetyRating `json:"safetyRatings,omitempty"`
	Index         int            `json:"index"`
}

type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type promptFeedback struct {
	BlockReason string `json:"blockReason,omitempty"`
}

type safetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

// 流式响应
type streamChunk struct {
	Candidates    []candidate   `json:"candidates,omitempty"`
	UsageMetadata usageMetadata `json:"usageMetadata,omitempty"`
}

// Chat 非流式对话
func (c *Client) Chat(ctx context.Context, req *agents.ChatRequest) (*agents.ChatResponse, error) {
	apiReq := c.buildRequest(req)

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.baseURL, req.Model, c.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return c.parseResponse(&apiResp, req.Model), nil
}

// ChatStream 流式对话
func (c *Client) ChatStream(ctx context.Context, req *agents.ChatRequest) (<-chan agents.StreamEvent, error) {
	apiReq := c.buildRequest(req)

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", c.baseURL, req.Model, c.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

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

	ch <- agents.StreamEvent{Type: agents.StreamEventStart}

	var lastUsage *usageMetadata

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
		if data == "" {
			continue
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// 保存 usage
		if chunk.UsageMetadata.TotalTokenCount > 0 {
			lastUsage = &chunk.UsageMetadata
		}

		if len(chunk.Candidates) == 0 {
			continue
		}

		candidate := chunk.Candidates[0]

		// 处理内容
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				ch <- agents.StreamEvent{
					Type:    agents.StreamEventDelta,
					Content: part.Text,
				}
			}
			if part.FunctionCall != nil {
				ch <- agents.StreamEvent{
					Type: agents.StreamEventToolCall,
					ToolCall: &agents.ToolCall{
						ID:        part.FunctionCall.Name, // Gemini 没有单独的 ID
						Name:      part.FunctionCall.Name,
						Arguments: part.FunctionCall.Args,
					},
				}
			}
		}
	}

	// 发送 usage
	if lastUsage != nil {
		ch <- agents.StreamEvent{
			Type: agents.StreamEventUsage,
			Usage: &agents.Usage{
				InputTokens:  lastUsage.PromptTokenCount,
				OutputTokens: lastUsage.CandidatesTokenCount,
			},
		}
	}

	ch <- agents.StreamEvent{Type: agents.StreamEventDone, Done: true}
}

func (c *Client) buildRequest(req *agents.ChatRequest) *generateRequest {
	apiReq := &generateRequest{
		GenerationConfig: &generationConfig{},
	}

	if req.MaxTokens > 0 {
		apiReq.GenerationConfig.MaxOutputTokens = req.MaxTokens
	} else {
		apiReq.GenerationConfig.MaxOutputTokens = 8192
	}

	if req.Temperature != nil {
		apiReq.GenerationConfig.Temperature = req.Temperature
	}

	if len(req.StopSeqs) > 0 {
		apiReq.GenerationConfig.StopSequences = req.StopSeqs
	}

	// System instruction
	if req.System != "" {
		apiReq.SystemInstruction = &content{
			Parts: []part{{Text: req.System}},
		}
	}

	// 转换消息
	for _, msg := range req.Messages {
		geminiContent := content{
			Role: c.mapRole(msg.Role),
		}

		switch contentVal := msg.Content.(type) {
		case string:
			geminiContent.Parts = append(geminiContent.Parts, part{Text: contentVal})
		case []agents.ContentBlock:
			for _, b := range contentVal {
				switch b.Type {
				case "text":
					geminiContent.Parts = append(geminiContent.Parts, part{Text: b.Text})
				case "image":
					if b.Image != nil {
						geminiContent.Parts = append(geminiContent.Parts, part{
							InlineData: &inlineData{
								MimeType: b.Image.MediaType,
								Data:     b.Image.Data,
							},
						})
					}
				case "tool_use":
					if b.ToolUse != nil {
						args, _ := b.ToolUse.Arguments.(map[string]any)
						geminiContent.Parts = append(geminiContent.Parts, part{
							FunctionCall: &functionCall{
								Name: b.ToolUse.Name,
								Args: args,
							},
						})
					}
				case "tool_result":
					if b.ToolResult != nil {
						// Gemini 的 function response 需要单独的 content
						apiReq.Contents = append(apiReq.Contents, content{
							Role: "function",
							Parts: []part{{
								FunctionResponse: &functionResponse{
									Name: b.ToolResult.ToolCallID,
									Response: map[string]any{
										"result": b.ToolResult.Content,
									},
								},
							}},
						})
						continue
					}
				}
			}
		}

		if len(geminiContent.Parts) > 0 {
			apiReq.Contents = append(apiReq.Contents, geminiContent)
		}
	}

	// 转换工具
	if len(req.Tools) > 0 {
		var funcDecls []functionDeclaration
		for _, tool := range req.Tools {
			funcDecls = append(funcDecls, functionDeclaration{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			})
		}
		apiReq.Tools = []tool{{FunctionDeclarations: funcDecls}}
	}

	return apiReq
}

func (c *Client) mapRole(role string) string {
	switch role {
	case "user":
		return "user"
	case "assistant":
		return "model"
	case "system":
		return "user" // Gemini 不直接支持 system role 在 contents 中
	default:
		return role
	}
}

func (c *Client) parseResponse(resp *generateResponse, model string) *agents.ChatResponse {
	result := &agents.ChatResponse{
		Model: model,
		Usage: agents.Usage{
			InputTokens:  resp.UsageMetadata.PromptTokenCount,
			OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
		},
	}

	if len(resp.Candidates) > 0 {
		candidate := resp.Candidates[0]
		result.StopReason = candidate.FinishReason

		var textParts []string
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}
			if part.FunctionCall != nil {
				result.ToolCalls = append(result.ToolCalls, agents.ToolCall{
					ID:        part.FunctionCall.Name,
					Name:      part.FunctionCall.Name,
					Arguments: part.FunctionCall.Args,
				})
			}
		}
		result.Content = strings.Join(textParts, "")
	}

	return result
}

// GetAPIKey 获取 Google API Key
func GetAPIKey(explicit string) string {
	if explicit != "" {
		return explicit
	}

	// 尝试多个环境变量
	envVars := []string{"GOOGLE_API_KEY", "GEMINI_API_KEY", "GOOGLE_GENERATIVE_AI_API_KEY"}
	for _, env := range envVars {
		if key := os.Getenv(env); key != "" {
			return key
		}
	}

	return ""
}
