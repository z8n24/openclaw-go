package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TTSTool 文字转语音工具
type TTSTool struct {
	workdir string
	// TTS 函数回调 (由外部 TTS 服务注入)
	SynthesizeFunc func(ctx context.Context, text, voice, channel string) (string, error)
	defaultVoice   string
}

// TTSParams tts 工具参数
type TTSParams struct {
	Text    string `json:"text"`
	Voice   string `json:"voice,omitempty"`
	Channel string `json:"channel,omitempty"` // 用于选择输出格式
}

// NewTTSTool 创建 tts 工具
func NewTTSTool(workdir string) *TTSTool {
	return &TTSTool{
		workdir:      workdir,
		defaultVoice: "nova", // 默认声音
	}
}

func (t *TTSTool) Name() string {
	return ToolTTS
}

func (t *TTSTool) Description() string {
	return "Convert text to speech and return a MEDIA: path. Use when the user requests audio or TTS is enabled."
}

func (t *TTSTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"text": {
				"type": "string",
				"description": "Text to convert to speech"
			},
			"voice": {
				"type": "string",
				"description": "Voice name/id to use"
			},
			"channel": {
				"type": "string",
				"description": "Optional channel id to pick output format (e.g. telegram)"
			}
		},
		"required": ["text"]
	}`
	return json.RawMessage(schema)
}

func (t *TTSTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params TTSParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if params.Text == "" {
		return &Result{Content: "Text is required", IsError: true}, nil
	}

	voice := params.Voice
	if voice == "" {
		voice = t.defaultVoice
	}

	// 检查 TTS 函数是否已注入
	if t.SynthesizeFunc == nil {
		return &Result{Content: "TTS not configured", IsError: true}, nil
	}

	// 调用 TTS 服务
	audioPath, err := t.SynthesizeFunc(ctx, params.Text, voice, params.Channel)
	if err != nil {
		return &Result{Content: "TTS failed: " + err.Error(), IsError: true}, nil
	}

	// 返回 MEDIA 格式
	return &Result{
		Content: fmt.Sprintf("MEDIA: %s", audioPath),
		Media: []MediaItem{
			{
				Type:     "audio",
				Path:     audioPath,
				MimeType: "audio/mpeg", // 默认 MP3
			},
		},
	}, nil
}

// SetSynthesizeFunc 设置 TTS 合成函数
func (t *TTSTool) SetSynthesizeFunc(fn func(ctx context.Context, text, voice, channel string) (string, error)) {
	t.SynthesizeFunc = fn
}

// SetDefaultVoice 设置默认声音
func (t *TTSTool) SetDefaultVoice(voice string) {
	t.defaultVoice = voice
}

// SimpleTTSProvider 简单的 TTS 提供者 (使用系统 say 命令或 espeak)
type SimpleTTSProvider struct {
	outputDir string
}

// NewSimpleTTSProvider 创建简单 TTS 提供者
func NewSimpleTTSProvider(outputDir string) *SimpleTTSProvider {
	return &SimpleTTSProvider{outputDir: outputDir}
}

// Synthesize 使用系统命令合成语音
func (p *SimpleTTSProvider) Synthesize(ctx context.Context, text, voice, channel string) (string, error) {
	// 创建输出目录
	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	// 生成输出文件名
	filename := fmt.Sprintf("tts_%d.aiff", time.Now().UnixNano())
	outputPath := filepath.Join(p.outputDir, filename)

	// 尝试使用 macOS say 命令
	// say -v voice -o output.aiff "text"
	// 注意：这只是一个简单的示例，实际应该使用更好的 TTS 服务

	return outputPath, fmt.Errorf("TTS synthesis not implemented - please configure an external TTS service")
}
