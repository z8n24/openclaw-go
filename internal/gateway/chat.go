package gateway

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/z8n24/openclaw-go/internal/agents"
	"github.com/z8n24/openclaw-go/internal/agents/anthropic"
	"github.com/z8n24/openclaw-go/internal/agents/deepseek"
)

// ChatRequest 聊天请求
type ChatRequest struct {
	Message  string `json:"message"`
	Model    string `json:"model,omitempty"`
	Provider string `json:"provider,omitempty"`
	Stream   bool   `json:"stream,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Content string `json:"content"`
	Model   string `json:"model"`
	Error   string `json:"error,omitempty"`
}

// handleChat 处理聊天请求
func (s *Server) handleChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ChatResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, ChatResponse{Error: "Message is required"})
		return
	}

	// 选择 provider
	provider := req.Provider
	if provider == "" {
		// 根据 model 自动选择
		if strings.Contains(req.Model, "deepseek") {
			provider = "deepseek"
		} else if strings.Contains(req.Model, "claude") {
			provider = "anthropic"
		} else {
			// 默认检测哪个 API key 可用
			if deepseek.GetAPIKey("") != "" {
				provider = "deepseek"
			} else if anthropic.GetAPIKey("") != "" {
				provider = "anthropic"
			} else {
				c.JSON(http.StatusBadRequest, ChatResponse{Error: "No API key configured (set DEEPSEEK_API_KEY or ANTHROPIC_API_KEY)"})
				return
			}
		}
	}

	// 创建 provider
	var agentProvider agents.Provider
	var model string

	switch provider {
	case "deepseek":
		if deepseek.GetAPIKey("") == "" {
			c.JSON(http.StatusBadRequest, ChatResponse{Error: "DEEPSEEK_API_KEY not set"})
			return
		}
		agentProvider = deepseek.NewClient("")
		model = req.Model
		if model == "" {
			model = "deepseek-chat"
		}
	case "anthropic":
		if anthropic.GetAPIKey("") == "" {
			c.JSON(http.StatusBadRequest, ChatResponse{Error: "ANTHROPIC_API_KEY not set"})
			return
		}
		agentProvider = anthropic.NewClient("")
		model = req.Model
		if model == "" {
			model = "claude-sonnet-4-20250514"
		}
	default:
		c.JSON(http.StatusBadRequest, ChatResponse{Error: "Unknown provider: " + provider})
		return
	}

	// 构建请求
	chatReq := &agents.ChatRequest{
		Model: model,
		Messages: []agents.Message{
			{Role: "user", Content: req.Message},
		},
	}

	log.Info().
		Str("provider", provider).
		Str("model", model).
		Int("msgLen", len(req.Message)).
		Msg("Processing chat request")

	// 流式响应
	if req.Stream {
		s.handleStreamChat(c, agentProvider, chatReq, model)
		return
	}

	// 非流式响应
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	resp, err := agentProvider.Chat(ctx, chatReq)
	if err != nil {
		log.Error().Err(err).Msg("Chat failed")
		c.JSON(http.StatusInternalServerError, ChatResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ChatResponse{
		Content: resp.Content,
		Model:   model,
	})
}

// handleStreamChat 处理流式聊天
func (s *Server) handleStreamChat(c *gin.Context, provider agents.Provider, req *agents.ChatRequest, model string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	ch, err := provider.ChatStream(ctx, req)
	if err != nil {
		c.SSEvent("error", err.Error())
		return
	}

	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-ch:
			if !ok {
				return false
			}
			if event.Error != nil {
				c.SSEvent("error", event.Error.Error())
				return false
			}
			if event.Done {
				c.SSEvent("done", "")
				return false
			}
			if event.Content != "" {
				c.SSEvent("delta", event.Content)
			}
			return true
		case <-ctx.Done():
			return false
		}
	})
}

// handleModels 返回可用模型列表
func (s *Server) handleModels(c *gin.Context) {
	models := []map[string]any{}

	// DeepSeek models
	if deepseek.GetAPIKey("") != "" {
		models = append(models, map[string]any{
			"id":       "deepseek-chat",
			"name":     "DeepSeek Chat",
			"provider": "deepseek",
		})
		models = append(models, map[string]any{
			"id":       "deepseek-coder",
			"name":     "DeepSeek Coder",
			"provider": "deepseek",
		})
	}

	// Anthropic models
	if anthropic.GetAPIKey("") != "" {
		models = append(models, map[string]any{
			"id":       "claude-sonnet-4-20250514",
			"name":     "Claude 4 Sonnet",
			"provider": "anthropic",
		})
		models = append(models, map[string]any{
			"id":       "claude-opus-4-5",
			"name":     "Claude 4.5 Opus",
			"provider": "anthropic",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"models": models,
	})
}
