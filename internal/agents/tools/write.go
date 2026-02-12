package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteTool 写入文件的工具
type WriteTool struct {
	workdir string
}

// WriteParams write 工具参数
type WriteParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// NewWriteTool 创建 write 工具
func NewWriteTool(workdir string) *WriteTool {
	return &WriteTool{
		workdir: workdir,
	}
}

func (t *WriteTool) Name() string {
	return ToolWrite
}

func (t *WriteTool) Description() string {
	return "Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Automatically creates parent directories."
}

func (t *WriteTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the file to write (relative or absolute)"
			},
			"content": {
				"type": "string",
				"description": "Content to write to the file"
			}
		},
		"required": ["path", "content"]
	}`
	return json.RawMessage(schema)
}

func (t *WriteTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params WriteParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if params.Path == "" {
		return &Result{Content: "Path is required", IsError: true}, nil
	}

	// 解析路径
	path := params.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workdir, path)
	}

	// 创建父目录
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &Result{Content: fmt.Sprintf("Failed to create directory: %v", err), IsError: true}, nil
	}

	// 检查文件是否存在 (用于报告)
	_, existsErr := os.Stat(path)
	fileExists := existsErr == nil

	// 写入文件
	if err := os.WriteFile(path, []byte(params.Content), 0644); err != nil {
		return &Result{Content: fmt.Sprintf("Failed to write file: %v", err), IsError: true}, nil
	}

	// 构建结果消息
	action := "Created"
	if fileExists {
		action = "Updated"
	}
	
	return &Result{
		Content: fmt.Sprintf("%s %s (%d bytes)", action, path, len(params.Content)),
	}, nil
}
