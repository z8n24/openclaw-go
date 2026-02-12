package tools

import (
	"context"
	"encoding/json"
)

// Tool 是工具的抽象接口
type Tool interface {
	// Name 返回工具名称
	Name() string
	
	// Description 返回工具描述
	Description() string
	
	// Parameters 返回参数的 JSON Schema
	Parameters() json.RawMessage
	
	// Execute 执行工具
	Execute(ctx context.Context, args json.RawMessage) (*Result, error)
}

// Result 工具执行结果
type Result struct {
	Content string      `json:"content"`
	IsError bool        `json:"isError,omitempty"`
	Media   []MediaItem `json:"media,omitempty"`
}

// MediaItem 媒体项
type MediaItem struct {
	Type     string `json:"type"` // "image" | "file"
	Path     string `json:"path,omitempty"`
	URL      string `json:"url,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Caption  string `json:"caption,omitempty"`
}

// Registry 工具注册表
type Registry struct {
	tools map[string]Tool
}

// NewRegistry 创建工具注册表
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register 注册工具
func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get 获取工具
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List 列出所有工具
func (r *Registry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// ToSchemas 转换为 Agent 可用的工具定义
func (r *Registry) ToSchemas() []map[string]interface{} {
	schemas := make([]map[string]interface{}, 0, len(r.tools))
	for _, t := range r.tools {
		var params interface{}
		if err := json.Unmarshal(t.Parameters(), &params); err != nil {
			params = map[string]interface{}{"type": "object"}
		}
		
		schemas = append(schemas, map[string]interface{}{
			"name":        t.Name(),
			"description": t.Description(),
			"parameters":  params,
		})
	}
	return schemas
}

// Execute 执行指定工具
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (*Result, error) {
	tool, ok := r.Get(name)
	if !ok {
		return &Result{
			Content: "Tool not found: " + name,
			IsError: true,
		}, nil
	}
	return tool.Execute(ctx, args)
}

// 预定义工具名
const (
	ToolRead         = "read"
	ToolWrite        = "write"
	ToolEdit         = "edit"
	ToolExec         = "exec"
	ToolProcess      = "process"
	ToolBrowser      = "browser"
	ToolWebSearch    = "web_search"
	ToolWebFetch     = "web_fetch"
	ToolImage        = "image"
	ToolMemorySearch = "memory_search"
	ToolMemoryGet    = "memory_get"
	ToolCron         = "cron"
	ToolMessage      = "message"
	ToolTTS          = "tts"
	ToolNodes        = "nodes"
	ToolCanvas       = "canvas"
	ToolGateway      = "gateway"
	ToolAgentsList   = "agents_list"
	ToolSessionsList = "sessions_list"
	ToolSessionsHistory = "sessions_history"
	ToolSessionsSend = "sessions_send"
	ToolSessionsSpawn = "sessions_spawn"
	ToolSessionStatus = "session_status"
)

// ToolsConfig 工具配置
type ToolsConfig struct {
	Workdir       string
	ConfigPath    string
	CronScheduler interface{} // *cron.Scheduler
}

// RegisterAllTools 注册所有内置工具
func RegisterAllTools(registry *Registry, cfg ToolsConfig) {
	// 文件操作工具
	registry.Register(NewReadTool(cfg.Workdir))
	registry.Register(NewWriteTool(cfg.Workdir))
	registry.Register(NewEditTool(cfg.Workdir))
	
	// 命令执行
	registry.Register(NewExecTool(cfg.Workdir))
	registry.Register(NewProcessTool(cfg.Workdir))
	
	// Web 工具
	registry.Register(NewWebSearchTool())
	registry.Register(NewWebFetchTool())
	
	// 浏览器
	registry.Register(NewBrowserTool())
	
	// 记忆
	registry.Register(NewMemorySearchTool(cfg.Workdir))
	registry.Register(NewMemoryGetTool(cfg.Workdir))
	
	// 定时任务 (需要 scheduler，由调用者单独注册)
	// 使用: registry.Register(NewCronTool(scheduler))
	
	// 消息
	registry.Register(NewMessageTool())
	
	// 图像
	registry.Register(NewImageTool(cfg.Workdir))
	
	// TTS
	registry.Register(NewTTSTool(cfg.Workdir))
	
	// 节点
	registry.Register(NewNodesTool())
	
	// Canvas
	registry.Register(NewCanvasTool())
	
	// Gateway
	registry.Register(NewGatewayTool(cfg.ConfigPath))
	
	// Sessions (需要 SessionManager，由调用者单独注册)
	// 使用: RegisterSessionTools(registry, manager)
	
	// Agents list
	registry.Register(NewAgentsListTool())
}

// RegisterCoreTools 只注册核心工具 (无外部依赖)
func RegisterCoreTools(registry *Registry, workdir string) {
	// 文件操作
	registry.Register(NewReadTool(workdir))
	registry.Register(NewWriteTool(workdir))
	registry.Register(NewEditTool(workdir))
	
	// 命令执行
	registry.Register(NewExecTool(workdir))
	registry.Register(NewProcessTool(workdir))
	
	// Web
	registry.Register(NewWebSearchTool())
	registry.Register(NewWebFetchTool())
	
	// 浏览器
	registry.Register(NewBrowserTool())
	
	// 记忆
	registry.Register(NewMemorySearchTool(workdir))
	registry.Register(NewMemoryGetTool(workdir))
	
	// 图像
	registry.Register(NewImageTool(workdir))
}
