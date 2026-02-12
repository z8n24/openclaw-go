package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditTool 编辑文件的工具
type EditTool struct {
	workdir string
}

// EditParams edit 工具参数
type EditParams struct {
	Path    string `json:"path"`
	OldText string `json:"oldText"`
	NewText string `json:"newText"`
	// 兼容别名
	OldString string `json:"old_string,omitempty"`
	NewString string `json:"new_string,omitempty"`
}

// NewEditTool 创建 edit 工具
func NewEditTool(workdir string) *EditTool {
	return &EditTool{
		workdir: workdir,
	}
}

func (t *EditTool) Name() string {
	return ToolEdit
}

func (t *EditTool) Description() string {
	return "Edit a file by replacing exact text. The oldText must match exactly (including whitespace). Use this for precise, surgical edits."
}

func (t *EditTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the file to edit (relative or absolute)"
			},
			"oldText": {
				"type": "string",
				"description": "Exact text to find and replace (must match exactly)"
			},
			"newText": {
				"type": "string",
				"description": "New text to replace the old text with"
			},
			"old_string": {
				"type": "string",
				"description": "Alias for oldText"
			},
			"new_string": {
				"type": "string",
				"description": "Alias for newText"
			}
		},
		"required": ["path", "oldText", "newText"]
	}`
	return json.RawMessage(schema)
}

func (t *EditTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params EditParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	// 处理别名
	if params.OldText == "" && params.OldString != "" {
		params.OldText = params.OldString
	}
	if params.NewText == "" && params.NewString != "" {
		params.NewText = params.NewString
	}

	if params.Path == "" {
		return &Result{Content: "Path is required", IsError: true}, nil
	}
	if params.OldText == "" {
		return &Result{Content: "oldText is required", IsError: true}, nil
	}

	// 解析路径
	path := params.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workdir, path)
	}

	// 读取文件
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Result{Content: fmt.Sprintf("File not found: %s", path), IsError: true}, nil
		}
		return &Result{Content: fmt.Sprintf("Failed to read file: %v", err), IsError: true}, nil
	}

	contentStr := string(content)

	// 检查 oldText 是否存在
	if !strings.Contains(contentStr, params.OldText) {
		// 提供有用的错误信息
		preview := params.OldText
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		return &Result{
			Content: fmt.Sprintf("Text not found in file.\nSearched for: %q\nFile: %s", preview, path),
			IsError: true,
		}, nil
	}

	// 统计匹配次数
	count := strings.Count(contentStr, params.OldText)
	if count > 1 {
		return &Result{
			Content: fmt.Sprintf("Found %d matches for the text. Please provide a more specific oldText that matches exactly once.", count),
			IsError: true,
		}, nil
	}

	// 执行替换
	newContent := strings.Replace(contentStr, params.OldText, params.NewText, 1)

	// 写回文件
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return &Result{Content: fmt.Sprintf("Failed to write file: %v", err), IsError: true}, nil
	}

	// 计算差异统计
	oldLines := strings.Count(params.OldText, "\n") + 1
	newLines := strings.Count(params.NewText, "\n") + 1
	lineDiff := newLines - oldLines

	var diffMsg string
	if lineDiff > 0 {
		diffMsg = fmt.Sprintf("+%d lines", lineDiff)
	} else if lineDiff < 0 {
		diffMsg = fmt.Sprintf("%d lines", lineDiff)
	} else {
		diffMsg = "same lines"
	}

	return &Result{
		Content: fmt.Sprintf("Edited %s: replaced %d chars with %d chars (%s)", 
			path, len(params.OldText), len(params.NewText), diffMsg),
	}, nil
}
