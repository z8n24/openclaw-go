package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ImageTool 图片视觉分析工具
type ImageTool struct {
	workdir string
	// 视觉分析函数回调 (由 Provider 注入)
	AnalyzeFunc func(ctx context.Context, imageData []byte, mimeType, prompt, model string) (string, error)
	maxSizeMB   int
}

// ImageParams image 工具参数
type ImageParams struct {
	Image      string `json:"image"`                // 图片路径或 URL
	Prompt     string `json:"prompt,omitempty"`     // 分析提示
	Model      string `json:"model,omitempty"`      // 使用的模型
	MaxBytesMB int    `json:"maxBytesMb,omitempty"` // 最大文件大小 (MB)
}

// NewImageTool 创建 image 工具
func NewImageTool(workdir string) *ImageTool {
	return &ImageTool{
		workdir:   workdir,
		maxSizeMB: 20, // 默认 20MB 限制
	}
}

func (t *ImageTool) Name() string {
	return ToolImage
}

func (t *ImageTool) Description() string {
	return "Analyze an image with a vision model. Only use this tool when the image was NOT already provided in the user's message."
}

func (t *ImageTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"image": {
				"type": "string",
				"description": "Path to image file or URL"
			},
			"prompt": {
				"type": "string",
				"description": "Analysis prompt (what to look for)"
			},
			"model": {
				"type": "string",
				"description": "Vision model to use (optional)"
			},
			"maxBytesMb": {
				"type": "number",
				"description": "Maximum file size in MB"
			}
		},
		"required": ["image"]
	}`
	return json.RawMessage(schema)
}

func (t *ImageTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params ImageParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if params.Image == "" {
		return &Result{Content: "Image path or URL is required", IsError: true}, nil
	}

	// 设置默认 prompt
	prompt := params.Prompt
	if prompt == "" {
		prompt = "Describe this image in detail."
	}

	// 获取图片数据
	imageData, mimeType, err := t.loadImage(ctx, params.Image, params.MaxBytesMB)
	if err != nil {
		return &Result{Content: "Failed to load image: " + err.Error(), IsError: true}, nil
	}

	// 检查分析函数是否已注入
	if t.AnalyzeFunc == nil {
		return &Result{Content: "Vision analysis not configured", IsError: true}, nil
	}

	// 分析图片
	result, err := t.AnalyzeFunc(ctx, imageData, mimeType, prompt, params.Model)
	if err != nil {
		return &Result{Content: "Analysis failed: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: result}, nil
}

func (t *ImageTool) loadImage(ctx context.Context, source string, maxMB int) ([]byte, string, error) {
	if maxMB <= 0 {
		maxMB = t.maxSizeMB
	}
	maxBytes := int64(maxMB) * 1024 * 1024

	// 检查是否是 URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return t.loadImageFromURL(ctx, source, maxBytes)
	}

	// 检查是否是 data URI
	if strings.HasPrefix(source, "data:") {
		return t.loadImageFromDataURI(source)
	}

	// 当作文件路径处理
	return t.loadImageFromFile(source, maxBytes)
}

func (t *ImageTool) loadImageFromFile(path string, maxBytes int64) ([]byte, string, error) {
	// 扩展路径
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	// 如果是相对路径，相对于工作目录
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workdir, path)
	}

	// 检查文件存在
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}

	// 检查文件大小
	if info.Size() > maxBytes {
		return nil, "", fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), maxBytes)
	}

	// 读取文件
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	// 检测 MIME 类型
	mimeType := t.detectMimeType(path, data)

	return data, mimeType, nil
}

func (t *ImageTool) loadImageFromURL(ctx context.Context, url string, maxBytes int64) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}

	client := &http.Client{
		Timeout: 30 * 1000 * 1000 * 1000, // 30 seconds
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// 检查 Content-Length
	if resp.ContentLength > maxBytes {
		return nil, "", fmt.Errorf("file too large: %d bytes (max %d)", resp.ContentLength, maxBytes)
	}

	// 读取数据，限制大小
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", err
	}

	if int64(len(data)) > maxBytes {
		return nil, "", fmt.Errorf("file too large: exceeds %d bytes", maxBytes)
	}

	// 获取 MIME 类型
	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = t.detectMimeType("", data)
	}

	return data, mimeType, nil
}

func (t *ImageTool) loadImageFromDataURI(dataURI string) ([]byte, string, error) {
	// 格式: data:image/png;base64,iVBORw0KGgo...
	if !strings.HasPrefix(dataURI, "data:") {
		return nil, "", fmt.Errorf("invalid data URI")
	}

	// 找到逗号分隔符
	commaIdx := strings.Index(dataURI, ",")
	if commaIdx == -1 {
		return nil, "", fmt.Errorf("invalid data URI: no comma found")
	}

	// 解析头部
	header := dataURI[5:commaIdx] // 跳过 "data:"
	data := dataURI[commaIdx+1:]

	// 提取 MIME 类型
	mimeType := "application/octet-stream"
	isBase64 := false

	parts := strings.Split(header, ";")
	for i, part := range parts {
		if i == 0 {
			mimeType = part
		} else if part == "base64" {
			isBase64 = true
		}
	}

	// 解码数据
	var decoded []byte
	var err error

	if isBase64 {
		decoded, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, "", fmt.Errorf("base64 decode: %w", err)
		}
	} else {
		decoded = []byte(data)
	}

	return decoded, mimeType, nil
}

func (t *ImageTool) detectMimeType(path string, data []byte) string {
	// 根据文件扩展名
	if path != "" {
		ext := strings.ToLower(filepath.Ext(path))
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
		case ".svg":
			return "image/svg+xml"
		}
	}

	// 根据文件头 (magic bytes)
	if len(data) >= 8 {
		// PNG: 89 50 4E 47 0D 0A 1A 0A
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
			return "image/png"
		}
		// JPEG: FF D8 FF
		if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			return "image/jpeg"
		}
		// GIF: GIF87a or GIF89a
		if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
			return "image/gif"
		}
		// WebP: RIFF....WEBP
		if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
			len(data) >= 12 && data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "image/webp"
		}
		// BMP: BM
		if data[0] == 0x42 && data[1] == 0x4D {
			return "image/bmp"
		}
	}

	return "application/octet-stream"
}

// SetAnalyzeFunc 设置视觉分析函数
func (t *ImageTool) SetAnalyzeFunc(fn func(ctx context.Context, imageData []byte, mimeType, prompt, model string) (string, error)) {
	t.AnalyzeFunc = fn
}
