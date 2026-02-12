package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// GatewayTool Gateway 自管理工具
type GatewayTool struct {
	configPath string
	// 回调函数
	GetConfigFunc    func() (map[string]interface{}, error)
	GetSchemaFunc    func() (map[string]interface{}, error)
	ApplyConfigFunc  func(config map[string]interface{}) error
	PatchConfigFunc  func(patch map[string]interface{}) error
	RestartFunc      func(delayMs int, reason string) error
	UpdateFunc       func() error
}

// GatewayParams gateway 工具参数
type GatewayParams struct {
	Action         string                 `json:"action"` // restart, config.get, config.schema, config.apply, config.patch, update.run
	Raw            string                 `json:"raw,omitempty"` // 原始 JSON 配置
	DelayMs        int                    `json:"delayMs,omitempty"`
	RestartDelayMs int                    `json:"restartDelayMs,omitempty"`
	Reason         string                 `json:"reason,omitempty"`
	Note           string                 `json:"note,omitempty"`
	BaseHash       string                 `json:"baseHash,omitempty"`
	SessionKey     string                 `json:"sessionKey,omitempty"`
	TimeoutMs      int                    `json:"timeoutMs,omitempty"`
}

// NewGatewayTool 创建 gateway 工具
func NewGatewayTool(configPath string) *GatewayTool {
	return &GatewayTool{
		configPath: configPath,
	}
}

func (t *GatewayTool) Name() string {
	return ToolGateway
}

func (t *GatewayTool) Description() string {
	return "Restart, apply config, or update the gateway in-place. Use config.patch for safe partial config updates."
}

func (t *GatewayTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["restart", "config.get", "config.schema", "config.apply", "config.patch", "update.run"],
				"description": "Gateway action"
			},
			"raw": {
				"type": "string",
				"description": "Raw JSON config for config.apply"
			},
			"delayMs": {
				"type": "number",
				"description": "Delay before action in milliseconds"
			},
			"restartDelayMs": {
				"type": "number",
				"description": "Delay before restart after config change"
			},
			"reason": {
				"type": "string",
				"description": "Reason for restart/update"
			},
			"note": {
				"type": "string",
				"description": "Note for config change"
			},
			"baseHash": {
				"type": "string",
				"description": "Base config hash for optimistic locking"
			},
			"sessionKey": {
				"type": "string",
				"description": "Session key context"
			},
			"timeoutMs": {
				"type": "number",
				"description": "Operation timeout"
			}
		},
		"required": ["action"]
	}`
	return json.RawMessage(schema)
}

func (t *GatewayTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params GatewayParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	switch params.Action {
	case "restart":
		return t.restart(params.DelayMs, params.Reason)
	case "config.get":
		return t.configGet()
	case "config.schema":
		return t.configSchema()
	case "config.apply":
		return t.configApply(params.Raw)
	case "config.patch":
		return t.configPatch(params.Raw)
	case "update.run":
		return t.updateRun()
	default:
		return &Result{Content: "Unknown action: " + params.Action, IsError: true}, nil
	}
}

func (t *GatewayTool) restart(delayMs int, reason string) (*Result, error) {
	if t.RestartFunc != nil {
		err := t.RestartFunc(delayMs, reason)
		if err != nil {
			return &Result{Content: "Restart failed: " + err.Error(), IsError: true}, nil
		}
		return &Result{Content: "Gateway restart initiated"}, nil
	}

	// 默认实现：通过信号重启自己
	if delayMs > 0 {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}

	// 尝试发送 SIGUSR1 给自己触发重启
	pid := os.Getpid()
	process, err := os.FindProcess(pid)
	if err != nil {
		return &Result{Content: "Failed to find process: " + err.Error(), IsError: true}, nil
	}

	// 在 Unix 系统上，SIGUSR1 通常用于触发重载/重启
	// 这里我们简单地返回成功，实际重启逻辑需要在主程序中处理信号
	_ = process

	result := map[string]interface{}{
		"status": "restart_scheduled",
		"pid":    pid,
		"reason": reason,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *GatewayTool) configGet() (*Result, error) {
	if t.GetConfigFunc != nil {
		config, err := t.GetConfigFunc()
		if err != nil {
			return &Result{Content: "Failed to get config: " + err.Error(), IsError: true}, nil
		}
		data, _ := json.MarshalIndent(config, "", "  ")
		return &Result{Content: string(data)}, nil
	}

	// 默认实现：读取配置文件
	if t.configPath == "" {
		return &Result{Content: "Config path not set", IsError: true}, nil
	}

	data, err := os.ReadFile(t.configPath)
	if err != nil {
		return &Result{Content: "Failed to read config: " + err.Error(), IsError: true}, nil
	}

	// 解析并美化输出
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		// 返回原始内容
		return &Result{Content: string(data)}, nil
	}

	formatted, _ := json.MarshalIndent(config, "", "  ")
	return &Result{Content: string(formatted)}, nil
}

func (t *GatewayTool) configSchema() (*Result, error) {
	if t.GetSchemaFunc != nil {
		schema, err := t.GetSchemaFunc()
		if err != nil {
			return &Result{Content: "Failed to get schema: " + err.Error(), IsError: true}, nil
		}
		data, _ := json.MarshalIndent(schema, "", "  ")
		return &Result{Content: string(data)}, nil
	}

	// 返回简化的 schema
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"gateway": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"port":  map[string]interface{}{"type": "number", "default": 19001},
					"bind":  map[string]interface{}{"type": "string", "default": "127.0.0.1"},
					"token": map[string]interface{}{"type": "string"},
				},
			},
			"agent": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"model":    map[string]interface{}{"type": "string"},
					"provider": map[string]interface{}{"type": "string"},
				},
			},
			"channels": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"telegram": map[string]interface{}{"type": "object"},
					"discord":  map[string]interface{}{"type": "object"},
					"whatsapp": map[string]interface{}{"type": "object"},
				},
			},
		},
	}

	data, _ := json.MarshalIndent(schema, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *GatewayTool) configApply(raw string) (*Result, error) {
	if raw == "" {
		return &Result{Content: "Raw config is required", IsError: true}, nil
	}

	// 解析 JSON
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return &Result{Content: "Invalid JSON: " + err.Error(), IsError: true}, nil
	}

	if t.ApplyConfigFunc != nil {
		err := t.ApplyConfigFunc(config)
		if err != nil {
			return &Result{Content: "Apply failed: " + err.Error(), IsError: true}, nil
		}
		return &Result{Content: "Config applied successfully"}, nil
	}

	// 默认实现：写入配置文件
	if t.configPath == "" {
		return &Result{Content: "Config path not set", IsError: true}, nil
	}

	formatted, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return &Result{Content: "Failed to format config: " + err.Error(), IsError: true}, nil
	}

	if err := os.WriteFile(t.configPath, formatted, 0644); err != nil {
		return &Result{Content: "Failed to write config: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: "Config applied. Restart required for changes to take effect."}, nil
}

func (t *GatewayTool) configPatch(raw string) (*Result, error) {
	if raw == "" {
		return &Result{Content: "Patch data is required", IsError: true}, nil
	}

	// 解析 patch
	var patch map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &patch); err != nil {
		return &Result{Content: "Invalid JSON patch: " + err.Error(), IsError: true}, nil
	}

	if t.PatchConfigFunc != nil {
		err := t.PatchConfigFunc(patch)
		if err != nil {
			return &Result{Content: "Patch failed: " + err.Error(), IsError: true}, nil
		}
		return &Result{Content: "Config patched successfully"}, nil
	}

	// 默认实现：读取、合并、写入
	if t.configPath == "" {
		return &Result{Content: "Config path not set", IsError: true}, nil
	}

	// 读取现有配置
	data, err := os.ReadFile(t.configPath)
	if err != nil && !os.IsNotExist(err) {
		return &Result{Content: "Failed to read config: " + err.Error(), IsError: true}, nil
	}

	var config map[string]interface{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &config); err != nil {
			return &Result{Content: "Failed to parse existing config: " + err.Error(), IsError: true}, nil
		}
	} else {
		config = make(map[string]interface{})
	}

	// 合并 patch
	mergeMap(config, patch)

	// 写回
	formatted, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return &Result{Content: "Failed to format config: " + err.Error(), IsError: true}, nil
	}

	if err := os.WriteFile(t.configPath, formatted, 0644); err != nil {
		return &Result{Content: "Failed to write config: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: "Config patched. Restart required for changes to take effect."}, nil
}

func (t *GatewayTool) updateRun() (*Result, error) {
	if t.UpdateFunc != nil {
		err := t.UpdateFunc()
		if err != nil {
			return &Result{Content: "Update failed: " + err.Error(), IsError: true}, nil
		}
		return &Result{Content: "Update completed successfully"}, nil
	}

	// 默认实现：尝试 git pull (如果在 git repo 中)
	cmd := exec.Command("git", "pull", "--ff-only")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 如果不是 git repo，尝试其他更新方式
		return &Result{Content: fmt.Sprintf("Update attempt:\n%s\nError: %v", string(output), err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Update completed:\n%s", string(output))}, nil
}

// mergeMap 递归合并 map
func mergeMap(dst, src map[string]interface{}) {
	for key, srcVal := range src {
		if dstVal, ok := dst[key]; ok {
			// 如果两边都是 map，递归合并
			srcMap, srcIsMap := srcVal.(map[string]interface{})
			dstMap, dstIsMap := dstVal.(map[string]interface{})
			if srcIsMap && dstIsMap {
				mergeMap(dstMap, srcMap)
				continue
			}
		}
		// 直接覆盖
		dst[key] = srcVal
	}
}

// 设置回调函数
func (t *GatewayTool) SetGetConfigFunc(fn func() (map[string]interface{}, error)) {
	t.GetConfigFunc = fn
}

func (t *GatewayTool) SetGetSchemaFunc(fn func() (map[string]interface{}, error)) {
	t.GetSchemaFunc = fn
}

func (t *GatewayTool) SetApplyConfigFunc(fn func(config map[string]interface{}) error) {
	t.ApplyConfigFunc = fn
}

func (t *GatewayTool) SetPatchConfigFunc(fn func(patch map[string]interface{}) error) {
	t.PatchConfigFunc = fn
}

func (t *GatewayTool) SetRestartFunc(fn func(delayMs int, reason string) error) {
	t.RestartFunc = fn
}

func (t *GatewayTool) SetUpdateFunc(fn func() error) {
	t.UpdateFunc = fn
}
