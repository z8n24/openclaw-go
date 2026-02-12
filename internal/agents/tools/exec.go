package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ExecTool 执行 shell 命令的工具
type ExecTool struct {
	workdir     string
	allowlist   []string // 允许的命令前缀
	timeout     time.Duration
	yieldMs     time.Duration // 默认 yield 时间
	processTool *ProcessTool  // 用于后台进程
}

// ExecParams exec 工具参数
type ExecParams struct {
	Command    string            `json:"command"`
	Workdir    string            `json:"workdir,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Timeout    int               `json:"timeout,omitempty"` // 秒
	Background bool              `json:"background,omitempty"`
	PTY        bool              `json:"pty,omitempty"`
	YieldMs    int               `json:"yieldMs,omitempty"` // 等待多久后转入后台
}

// NewExecTool 创建 exec 工具
func NewExecTool(workdir string) *ExecTool {
	return &ExecTool{
		workdir: workdir,
		timeout: 30 * time.Second,
		yieldMs: 10 * time.Second,
	}
}

// SetProcessTool 设置 process 工具引用 (用于后台进程)
func (t *ExecTool) SetProcessTool(pt *ProcessTool) {
	t.processTool = pt
}

func (t *ExecTool) Name() string {
	return ToolExec
}

func (t *ExecTool) Description() string {
	return "Execute shell commands with background continuation. Use yieldMs/background to continue later via process tool. Use pty=true for TTY-required commands."
}

func (t *ExecTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "Shell command to execute"
			},
			"workdir": {
				"type": "string",
				"description": "Working directory (defaults to workspace)"
			},
			"env": {
				"type": "object",
				"description": "Environment variables"
			},
			"timeout": {
				"type": "number",
				"description": "Timeout in seconds (optional, kills process on expiry)"
			},
			"background": {
				"type": "boolean",
				"description": "Run in background immediately"
			},
			"pty": {
				"type": "boolean",
				"description": "Run in pseudo-terminal (for TTY-required CLIs)"
			},
			"yieldMs": {
				"type": "number",
				"description": "Milliseconds to wait before backgrounding (default 10000)"
			}
		},
		"required": ["command"]
	}`
	return json.RawMessage(schema)
}

func (t *ExecTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params ExecParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if params.Command == "" {
		return &Result{Content: "Command is required", IsError: true}, nil
	}

	// 确定工作目录
	workdir := t.workdir
	if params.Workdir != "" {
		workdir = params.Workdir
	}

	// 如果需要后台或 PTY，使用 ProcessTool
	if params.Background || params.PTY {
		return t.executeBackground(ctx, params, workdir)
	}

	// 普通同步执行
	return t.executeSync(ctx, params, workdir)
}

// executeSync 同步执行命令
func (t *ExecTool) executeSync(ctx context.Context, params ExecParams, workdir string) (*Result, error) {
	// 确定超时
	timeout := t.timeout
	if params.Timeout > 0 {
		timeout = time.Duration(params.Timeout) * time.Second
	}

	// yield 时间
	yieldMs := t.yieldMs
	if params.YieldMs > 0 {
		yieldMs = time.Duration(params.YieldMs) * time.Millisecond
	}

	// 创建带超时的 context
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 创建命令
	cmd := exec.CommandContext(execCtx, "sh", "-c", params.Command)
	cmd.Dir = workdir

	// 设置环境变量
	if len(params.Env) > 0 {
		env := os.Environ()
		for k, v := range params.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	// 使用 channel 来处理超时后转后台
	type execResult struct {
		output []byte
		err    error
	}
	resultChan := make(chan execResult, 1)

	go func() {
		output, err := cmd.CombinedOutput()
		resultChan <- execResult{output, err}
	}()

	// 等待完成或超时
	select {
	case result := <-resultChan:
		return t.formatSyncResult(result.output, result.err, execCtx, timeout, cmd)

	case <-time.After(yieldMs):
		// 超时，转入后台
		if t.processTool != nil {
			sessionID, err := t.processTool.StartSession(params.Command, workdir, false, params.Env)
			if err != nil {
				return &Result{
					Content: fmt.Sprintf("Command still running, failed to background: %v", err),
					IsError: true,
				}, nil
			}
			return &Result{
				Content: fmt.Sprintf("Command still running after %v, backgrounded as session: %s\nUse process tool to check status.", yieldMs, sessionID),
			}, nil
		}
		// 没有 processTool，等待完成
		result := <-resultChan
		return t.formatSyncResult(result.output, result.err, execCtx, timeout, cmd)
	}
}

// executeBackground 后台执行命令
func (t *ExecTool) executeBackground(ctx context.Context, params ExecParams, workdir string) (*Result, error) {
	if t.processTool == nil {
		return &Result{
			Content: "Background execution not available (process tool not configured)",
			IsError: true,
		}, nil
	}

	sessionID, err := t.processTool.StartSession(params.Command, workdir, params.PTY, params.Env)
	if err != nil {
		return &Result{
			Content: fmt.Sprintf("Failed to start background session: %v", err),
			IsError: true,
		}, nil
	}

	mode := "background"
	if params.PTY {
		mode = "PTY"
	}

	return &Result{
		Content: fmt.Sprintf("Started %s session: %s\nPID: check with process tool\nUse process(action=\"poll\", sessionId=\"%s\") to check status.\nUse process(action=\"log\", sessionId=\"%s\") to get output.", mode, sessionID, sessionID, sessionID),
	}, nil
}

// formatSyncResult 格式化同步执行结果
func (t *ExecTool) formatSyncResult(output []byte, err error, execCtx context.Context, timeout time.Duration, cmd *exec.Cmd) (*Result, error) {
	var result strings.Builder
	if len(output) > 0 {
		result.Write(output)
	}

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.WriteString("\n[Command timed out after ")
			result.WriteString(timeout.String())
			result.WriteString("]")
		} else {
			result.WriteString("\n[Exit error: ")
			result.WriteString(err.Error())
			result.WriteString("]")
		}
		return &Result{Content: result.String(), IsError: true}, nil
	}

	// 添加退出状态
	if cmd.ProcessState != nil {
		exitCode := cmd.ProcessState.ExitCode()
		if exitCode != 0 {
			result.WriteString(fmt.Sprintf("\n[Exit code: %d]", exitCode))
		}
	}

	return &Result{Content: result.String()}, nil
}
