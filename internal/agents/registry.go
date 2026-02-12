package agents

import (
	"context"
	"fmt"
	"strings"
)

// DefaultRegistry 全局默认提供商注册表
var DefaultRegistry = NewProviderRegistry()

// RegisterProvider 注册提供商到默认注册表
func RegisterProvider(provider Provider) {
	DefaultRegistry.Register(provider)
}

// GetProvider 从默认注册表获取提供商
func GetProvider(id string) (Provider, bool) {
	return DefaultRegistry.Get(id)
}

// ListProviders 列出默认注册表中的所有提供商
func ListProviders() []Provider {
	return DefaultRegistry.List()
}

// ResolveProviderAndModel 根据模型 ID 解析提供商
// 支持格式: "provider/model" 或 "model" (会遍历所有提供商查找)
func ResolveProviderAndModel(modelID string) (Provider, string, error) {
	providerID, model := ResolveModel(modelID)
	
	if providerID != "" {
		// 明确指定了提供商
		provider, ok := GetProvider(providerID)
		if !ok {
			return nil, "", fmt.Errorf("provider not found: %s", providerID)
		}
		return provider, model, nil
	}
	
	// 遍历所有提供商查找模型
	for _, provider := range ListProviders() {
		for _, m := range provider.ListModels() {
			if m.ID == model {
				return provider, model, nil
			}
		}
	}
	
	return nil, "", fmt.Errorf("model not found: %s", modelID)
}

// ListAllModels 列出所有提供商的所有模型
func ListAllModels() []ModelInfo {
	var models []ModelInfo
	for _, provider := range ListProviders() {
		models = append(models, provider.ListModels()...)
	}
	return models
}

// FindModel 查找模型信息
func FindModel(modelID string) (*ModelInfo, error) {
	providerID, model := ResolveModel(modelID)
	
	if providerID != "" {
		provider, ok := GetProvider(providerID)
		if !ok {
			return nil, fmt.Errorf("provider not found: %s", providerID)
		}
		for _, m := range provider.ListModels() {
			if m.ID == model {
				return &m, nil
			}
		}
		return nil, fmt.Errorf("model not found: %s/%s", providerID, model)
	}
	
	// 遍历查找
	for _, provider := range ListProviders() {
		for _, m := range provider.ListModels() {
			if m.ID == model {
				return &m, nil
			}
		}
	}
	
	return nil, fmt.Errorf("model not found: %s", modelID)
}

// ModelAliases 模型别名映射
var ModelAliases = map[string]string{
	// Anthropic
	"claude":        "anthropic/claude-sonnet-4-20250514",
	"sonnet":        "anthropic/claude-sonnet-4-20250514",
	"opus":          "anthropic/claude-opus-4-5",
	"haiku":         "anthropic/claude-3-5-haiku-latest",
	
	// OpenAI
	"gpt4":          "openai/gpt-4o",
	"gpt4o":         "openai/gpt-4o",
	"gpt4-mini":     "openai/gpt-4o-mini",
	"o1":            "openai/o1",
	"o1-mini":       "openai/o1-mini",
	"o3-mini":       "openai/o3-mini",
	
	// Google
	"gemini":        "google/gemini-2.0-flash-exp",
	"gemini-pro":    "google/gemini-1.5-pro",
	"gemini-flash":  "google/gemini-1.5-flash",
	
	// DeepSeek
	"deepseek":      "deepseek/deepseek-chat",
	"deepseek-r1":   "deepseek/deepseek-reasoner",
	
	// OpenRouter shortcuts
	"llama":         "openrouter/meta-llama/llama-3.3-70b-instruct",
	"mistral":       "openrouter/mistralai/mistral-large-2411",
	"qwen":          "openrouter/qwen/qwen-2.5-72b-instruct",
}

// ResolveAlias 解析模型别名
func ResolveAlias(alias string) string {
	if resolved, ok := ModelAliases[strings.ToLower(alias)]; ok {
		return resolved
	}
	return alias
}

// QuickChat 便捷函数：使用指定模型进行对话
func QuickChat(ctx context.Context, modelID string, messages []Message, options ...ChatOption) (*ChatResponse, error) {
	modelID = ResolveAlias(modelID)
	provider, model, err := ResolveProviderAndModel(modelID)
	if err != nil {
		return nil, err
	}
	
	req := &ChatRequest{
		Model:    model,
		Messages: messages,
	}
	
	for _, opt := range options {
		opt(req)
	}
	
	return provider.Chat(ctx, req)
}

// ChatOption 对话选项
type ChatOption func(*ChatRequest)

// WithSystem 设置系统提示
func WithSystem(system string) ChatOption {
	return func(req *ChatRequest) {
		req.System = system
	}
}

// WithTools 设置工具
func WithTools(tools []Tool) ChatOption {
	return func(req *ChatRequest) {
		req.Tools = tools
	}
}

// WithMaxTokens 设置最大输出 token
func WithMaxTokens(n int) ChatOption {
	return func(req *ChatRequest) {
		req.MaxTokens = n
	}
}

// WithTemperature 设置温度
func WithTemperature(t float64) ChatOption {
	return func(req *ChatRequest) {
		req.Temperature = &t
	}
}
