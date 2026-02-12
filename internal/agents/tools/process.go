package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
)

// ProcessTool 管理后台进程和 PTY 会话
type ProcessTool struct {
	workdir  string
	sessions map[string]*ProcessSession
	mu       sync.RWMutex
	counter  int
}

// ProcessSession 表示一个后台进程会话
type ProcessSession struct {
	ID        string    `json:"sessionId"`
	Command   string    `json:"command"`
	PID       int       `json:"pid"`
	PTY       bool      `json:"pty"`
	StartedAt time.Time `json:"startedAt"`
	Status    string    `json:"status"` // running, exited, killed

	cmd    *exec.Cmd
	ptmx   *os.File
	output []byte
	mu     sync.Mutex
	done   chan struct{}
}

// ProcessParams process 工具参数
type ProcessParams struct {
	Action    string `json:"action"` // list, poll, log, write, send-keys, kill
	SessionID string `json:"sessionId,omitempty"`
	Data      string `json:"data,omitempty"`      // for write action
	Literal   string `json:"literal,omitempty"`   // for send-keys
	Keys      []string `json:"keys,omitempty"`    // for send-keys (key tokens)
	Offset    int    `json:"offset,omitempty"`    // for log
	Limit     int    `json:"limit,omitempty"`     // for log
	EOF       bool   `json:"eof,omitempty"`       // close stdin after write
}

// NewProcessTool 创建 process 工具
func NewProcessTool(workdir string) *ProcessTool {
	return &ProcessTool{
		workdir:  workdir,
		sessions: make(map[string]*ProcessSession),
	}
}

func (t *ProcessTool) Name() string {
	return ToolProcess
}

func (t *ProcessTool) Description() string {
	return "Manage running exec sessions: list, poll, log, write, send-keys, kill."
}

func (t *ProcessTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["list", "poll", "log", "write", "send-keys", "kill"],
				"description": "Process action"
			},
			"sessionId": {
				"type": "string",
				"description": "Session id for actions other than list"
			},
			"data": {
				"type": "string",
				"description": "Data to write for write action"
			},
			"literal": {
				"type": "string",
				"description": "Literal string for send-keys"
			},
			"keys": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Key tokens to send for send-keys"
			},
			"offset": {
				"type": "number",
				"description": "Log offset"
			},
			"limit": {
				"type": "number",
				"description": "Log length limit"
			},
			"eof": {
				"type": "boolean",
				"description": "Close stdin after write"
			}
		},
		"required": ["action"]
	}`
	return json.RawMessage(schema)
}

func (t *ProcessTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params ProcessParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	switch params.Action {
	case "list":
		return t.list()
	case "poll":
		return t.poll(params.SessionID)
	case "log":
		return t.log(params.SessionID, params.Offset, params.Limit)
	case "write":
		return t.write(params.SessionID, params.Data, params.EOF)
	case "send-keys":
		return t.sendKeys(params.SessionID, params.Literal, params.Keys)
	case "kill":
		return t.kill(params.SessionID)
	default:
		return &Result{Content: "Unknown action: " + params.Action, IsError: true}, nil
	}
}

// StartSession 启动一个新的后台会话 (由 exec 工具调用)
func (t *ProcessTool) StartSession(command, workdir string, usePTY bool, env map[string]string) (string, error) {
	t.mu.Lock()
	t.counter++
	sessionID := fmt.Sprintf("session-%d", t.counter)
	t.mu.Unlock()

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = workdir

	// 设置环境变量
	if len(env) > 0 {
		cmdEnv := os.Environ()
		for k, v := range env {
			cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = cmdEnv
	}

	session := &ProcessSession{
		ID:        sessionID,
		Command:   command,
		PTY:       usePTY,
		StartedAt: time.Now(),
		Status:    "running",
		cmd:       cmd,
		done:      make(chan struct{}),
	}

	if usePTY {
		// 使用 PTY
		ptmx, err := pty.Start(cmd)
		if err != nil {
			return "", fmt.Errorf("pty start: %w", err)
		}
		session.ptmx = ptmx
		session.PID = cmd.Process.Pid

		// 后台读取输出
		go func() {
			defer close(session.done)
			buf := make([]byte, 4096)
			for {
				n, err := ptmx.Read(buf)
				if n > 0 {
					session.mu.Lock()
					session.output = append(session.output, buf[:n]...)
					// 限制输出缓冲区大小 (保留最后 1MB)
					if len(session.output) > 1024*1024 {
						session.output = session.output[len(session.output)-1024*1024:]
					}
					session.mu.Unlock()
				}
				if err != nil {
					break
				}
			}
			session.mu.Lock()
			session.Status = "exited"
			session.mu.Unlock()
		}()
	} else {
		// 非 PTY 模式
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("start: %w", err)
		}
		session.PID = cmd.Process.Pid

		// 后台读取输出
		go func() {
			defer close(session.done)
			// 读取 stdout 和 stderr
			go func() {
				buf := make([]byte, 4096)
				for {
					n, err := stdout.Read(buf)
					if n > 0 {
						session.mu.Lock()
						session.output = append(session.output, buf[:n]...)
						session.mu.Unlock()
					}
					if err != nil {
						break
					}
				}
			}()
			buf := make([]byte, 4096)
			for {
				n, err := stderr.Read(buf)
				if n > 0 {
					session.mu.Lock()
					session.output = append(session.output, buf[:n]...)
					session.mu.Unlock()
				}
				if err != nil {
					break
				}
			}
			cmd.Wait()
			session.mu.Lock()
			session.Status = "exited"
			session.mu.Unlock()
		}()
	}

	t.mu.Lock()
	t.sessions[sessionID] = session
	t.mu.Unlock()

	return sessionID, nil
}

func (t *ProcessTool) list() (*Result, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	sessions := make([]map[string]interface{}, 0, len(t.sessions))
	for _, s := range t.sessions {
		s.mu.Lock()
		sessions = append(sessions, map[string]interface{}{
			"sessionId": s.ID,
			"command":   s.Command,
			"pid":       s.PID,
			"pty":       s.PTY,
			"status":    s.Status,
			"startedAt": s.StartedAt.Format(time.RFC3339),
			"outputLen": len(s.output),
		})
		s.mu.Unlock()
	}

	data, _ := json.MarshalIndent(sessions, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *ProcessTool) poll(sessionID string) (*Result, error) {
	t.mu.RLock()
	session, ok := t.sessions[sessionID]
	t.mu.RUnlock()

	if !ok {
		return &Result{Content: "Session not found: " + sessionID, IsError: true}, nil
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	result := map[string]interface{}{
		"sessionId": session.ID,
		"status":    session.Status,
		"outputLen": len(session.output),
	}

	data, _ := json.Marshal(result)
	return &Result{Content: string(data)}, nil
}

func (t *ProcessTool) log(sessionID string, offset, limit int) (*Result, error) {
	t.mu.RLock()
	session, ok := t.sessions[sessionID]
	t.mu.RUnlock()

	if !ok {
		return &Result{Content: "Session not found: " + sessionID, IsError: true}, nil
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	output := session.output

	// 应用 offset
	if offset > 0 && offset < len(output) {
		output = output[offset:]
	}

	// 应用 limit
	if limit > 0 && limit < len(output) {
		output = output[:limit]
	}

	return &Result{Content: string(output)}, nil
}

func (t *ProcessTool) write(sessionID, data string, eof bool) (*Result, error) {
	t.mu.RLock()
	session, ok := t.sessions[sessionID]
	t.mu.RUnlock()

	if !ok {
		return &Result{Content: "Session not found: " + sessionID, IsError: true}, nil
	}

	if session.ptmx == nil {
		return &Result{Content: "Session does not have PTY", IsError: true}, nil
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status != "running" {
		return &Result{Content: "Session is not running", IsError: true}, nil
	}

	_, err := io.WriteString(session.ptmx, data)
	if err != nil {
		return &Result{Content: "Write failed: " + err.Error(), IsError: true}, nil
	}

	if eof {
		session.ptmx.Close()
	}

	return &Result{Content: fmt.Sprintf("Wrote %d bytes", len(data))}, nil
}

func (t *ProcessTool) sendKeys(sessionID, literal string, keys []string) (*Result, error) {
	t.mu.RLock()
	session, ok := t.sessions[sessionID]
	t.mu.RUnlock()

	if !ok {
		return &Result{Content: "Session not found: " + sessionID, IsError: true}, nil
	}

	if session.ptmx == nil {
		return &Result{Content: "Session does not have PTY", IsError: true}, nil
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status != "running" {
		return &Result{Content: "Session is not running", IsError: true}, nil
	}

	// 写入 literal 字符串
	if literal != "" {
		_, err := io.WriteString(session.ptmx, literal)
		if err != nil {
			return &Result{Content: "Send literal failed: " + err.Error(), IsError: true}, nil
		}
	}

	// 处理特殊键
	for _, key := range keys {
		var keyBytes []byte
		switch key {
		case "Enter", "Return":
			keyBytes = []byte{'\r'}
		case "Tab":
			keyBytes = []byte{'\t'}
		case "Escape", "Esc":
			keyBytes = []byte{0x1b}
		case "Backspace":
			keyBytes = []byte{0x7f}
		case "Delete":
			keyBytes = []byte{0x1b, '[', '3', '~'}
		case "Up":
			keyBytes = []byte{0x1b, '[', 'A'}
		case "Down":
			keyBytes = []byte{0x1b, '[', 'B'}
		case "Right":
			keyBytes = []byte{0x1b, '[', 'C'}
		case "Left":
			keyBytes = []byte{0x1b, '[', 'D'}
		case "Home":
			keyBytes = []byte{0x1b, '[', 'H'}
		case "End":
			keyBytes = []byte{0x1b, '[', 'F'}
		case "Ctrl-C":
			keyBytes = []byte{0x03}
		case "Ctrl-D":
			keyBytes = []byte{0x04}
		case "Ctrl-Z":
			keyBytes = []byte{0x1a}
		case "Ctrl-L":
			keyBytes = []byte{0x0c}
		default:
			// 尝试作为普通字符发送
			keyBytes = []byte(key)
		}
		_, err := session.ptmx.Write(keyBytes)
		if err != nil {
			return &Result{Content: "Send key failed: " + err.Error(), IsError: true}, nil
		}
	}

	return &Result{Content: "Keys sent"}, nil
}

func (t *ProcessTool) kill(sessionID string) (*Result, error) {
	t.mu.RLock()
	session, ok := t.sessions[sessionID]
	t.mu.RUnlock()

	if !ok {
		return &Result{Content: "Session not found: " + sessionID, IsError: true}, nil
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status != "running" {
		return &Result{Content: "Session already " + session.Status}, nil
	}

	if session.cmd.Process != nil {
		session.cmd.Process.Kill()
	}
	if session.ptmx != nil {
		session.ptmx.Close()
	}
	session.Status = "killed"

	return &Result{Content: "Session killed"}, nil
}

// Cleanup 清理已结束的会话
func (t *ProcessTool) Cleanup(maxAge time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for id, session := range t.sessions {
		session.mu.Lock()
		if session.Status != "running" && now.Sub(session.StartedAt) > maxAge {
			delete(t.sessions, id)
		}
		session.mu.Unlock()
	}
}
