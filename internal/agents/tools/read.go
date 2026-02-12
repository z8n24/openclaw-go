package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadTool 读取文件的工具
type ReadTool struct {
	workdir  string
	maxBytes int64
	maxLines int
}

// ReadParams read 工具参数
type ReadParams struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"` // 起始行 (1-indexed)
	Limit  int    `json:"limit,omitempty"`  // 最大行数
}

// NewReadTool 创建 read 工具
func NewReadTool(workdir string) *ReadTool {
	return &ReadTool{
		workdir:  workdir,
		maxBytes: 50 * 1024, // 50KB
		maxLines: 2000,
	}
}

func (t *ReadTool) Name() string {
	return ToolRead
}

func (t *ReadTool) Description() string {
	return "Read the contents of a file. Supports text files and images (jpg, png, gif, webp). For large files, use offset/limit."
}

func (t *ReadTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the file to read (relative or absolute)"
			},
			"offset": {
				"type": "number",
				"description": "Line number to start reading from (1-indexed)"
			},
			"limit": {
				"type": "number",
				"description": "Maximum number of lines to read"
			}
		},
		"required": ["path"]
	}`
	return json.RawMessage(schema)
}

func (t *ReadTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params ReadParams
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

	// 检查文件是否存在
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Result{Content: fmt.Sprintf("File not found: %s", params.Path), IsError: true}, nil
		}
		return &Result{Content: fmt.Sprintf("Error accessing file: %s", err), IsError: true}, nil
	}

	// 检查是否是目录
	if info.IsDir() {
		return &Result{Content: fmt.Sprintf("Path is a directory: %s", params.Path), IsError: true}, nil
	}

	// 检查是否是图片
	ext := strings.ToLower(filepath.Ext(path))
	if isImageExt(ext) {
		return t.readImage(path, ext)
	}

	// 读取文本文件
	return t.readText(path, params.Offset, params.Limit)
}

func (t *ReadTool) readText(path string, offset, limit int) (*Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error opening file: %s", err), IsError: true}, nil
	}
	defer file.Close()

	// 默认限制
	if limit <= 0 {
		limit = t.maxLines
	}
	if limit > t.maxLines {
		limit = t.maxLines
	}

	// offset 是 1-indexed
	if offset < 1 {
		offset = 1
	}

	scanner := bufio.NewScanner(file)
	var lines []string
	lineNum := 0
	totalBytes := int64(0)

	for scanner.Scan() {
		lineNum++

		// 跳过 offset 之前的行
		if lineNum < offset {
			continue
		}

		line := scanner.Text()
		lineBytes := int64(len(line))

		// 检查大小限制
		if totalBytes+lineBytes > t.maxBytes {
			lines = append(lines, fmt.Sprintf("[Truncated: exceeded %d bytes limit]", t.maxBytes))
			break
		}

		lines = append(lines, line)
		totalBytes += lineBytes

		// 检查行数限制
		if len(lines) >= limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return &Result{Content: fmt.Sprintf("Error reading file: %s", err), IsError: true}, nil
	}

	content := strings.Join(lines, "\n")
	return &Result{Content: content}, nil
}

func (t *ReadTool) readImage(path, ext string) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error reading image: %s", err), IsError: true}, nil
	}

	// 限制图片大小 (10MB)
	if len(data) > 10*1024*1024 {
		return &Result{Content: "Image too large (max 10MB)", IsError: true}, nil
	}

	mimeType := getMimeType(ext)

	return &Result{
		Content: fmt.Sprintf("[Image: %s, %d bytes]", filepath.Base(path), len(data)),
		Media: []MediaItem{
			{
				Type:     "image",
				Path:     path,
				MimeType: mimeType,
			},
		},
	}, nil
}

func isImageExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

func getMimeType(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}
