package tools

import (
	"context"
	"encoding/json"
)

// CanvasTool Canvas UI 控制工具
type CanvasTool struct {
	// 回调函数 (由 Gateway 注入)
	PresentFunc  func(ctx context.Context, url string, opts CanvasOptions) error
	HideFunc     func(ctx context.Context, target string) error
	NavigateFunc func(ctx context.Context, url, target string) error
	EvalFunc     func(ctx context.Context, js, target string) (string, error)
	SnapshotFunc func(ctx context.Context, target string, opts SnapshotOptions) ([]byte, error)
}

// CanvasOptions 呈现选项
type CanvasOptions struct {
	Target   string `json:"target,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	MaxWidth int    `json:"maxWidth,omitempty"`
	X        int    `json:"x,omitempty"`
	Y        int    `json:"y,omitempty"`
}

// SnapshotOptions 截图选项
type SnapshotOptions struct {
	Format  string `json:"outputFormat,omitempty"` // png, jpg, jpeg
	Quality int    `json:"quality,omitempty"`
	DelayMs int    `json:"delayMs,omitempty"`
}

// CanvasParams canvas 工具参数
type CanvasParams struct {
	Action       string `json:"action"` // present, hide, navigate, eval, snapshot, a2ui_push, a2ui_reset
	URL          string `json:"url,omitempty"`
	Target       string `json:"target,omitempty"`
	JavaScript   string `json:"javaScript,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	MaxWidth     int    `json:"maxWidth,omitempty"`
	X            int    `json:"x,omitempty"`
	Y            int    `json:"y,omitempty"`
	OutputFormat string `json:"outputFormat,omitempty"`
	Quality      int    `json:"quality,omitempty"`
	DelayMs      int    `json:"delayMs,omitempty"`
	TimeoutMs    int    `json:"timeoutMs,omitempty"`
	Node         string `json:"node,omitempty"`
	JSONL        string `json:"jsonl,omitempty"`
	JSONLPath    string `json:"jsonlPath,omitempty"`
}

// NewCanvasTool 创建 canvas 工具
func NewCanvasTool() *CanvasTool {
	return &CanvasTool{}
}

func (t *CanvasTool) Name() string {
	return ToolCanvas
}

func (t *CanvasTool) Description() string {
	return "Control node canvases (present/hide/navigate/eval/snapshot/A2UI). Use snapshot to capture the rendered UI."
}

func (t *CanvasTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["present", "hide", "navigate", "eval", "snapshot", "a2ui_push", "a2ui_reset"],
				"description": "Canvas action"
			},
			"url": {
				"type": "string",
				"description": "URL for present/navigate"
			},
			"target": {
				"type": "string",
				"description": "Target canvas id"
			},
			"javaScript": {
				"type": "string",
				"description": "JavaScript to evaluate"
			},
			"width": {
				"type": "number",
				"description": "Canvas width"
			},
			"height": {
				"type": "number",
				"description": "Canvas height"
			},
			"maxWidth": {
				"type": "number",
				"description": "Maximum width"
			},
			"x": {
				"type": "number",
				"description": "X position"
			},
			"y": {
				"type": "number",
				"description": "Y position"
			},
			"outputFormat": {
				"type": "string",
				"enum": ["png", "jpg", "jpeg"],
				"description": "Screenshot format"
			},
			"quality": {
				"type": "number",
				"description": "JPEG quality (0-100)"
			},
			"delayMs": {
				"type": "number",
				"description": "Delay before action"
			},
			"timeoutMs": {
				"type": "number",
				"description": "Operation timeout"
			},
			"node": {
				"type": "string",
				"description": "Target node id/name"
			},
			"jsonl": {
				"type": "string",
				"description": "JSONL data for A2UI"
			},
			"jsonlPath": {
				"type": "string",
				"description": "Path to JSONL file for A2UI"
			}
		},
		"required": ["action"]
	}`
	return json.RawMessage(schema)
}

func (t *CanvasTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params CanvasParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	switch params.Action {
	case "present":
		return t.present(ctx, &params)
	case "hide":
		return t.hide(ctx, &params)
	case "navigate":
		return t.navigate(ctx, &params)
	case "eval":
		return t.eval(ctx, &params)
	case "snapshot":
		return t.snapshot(ctx, &params)
	case "a2ui_push":
		return t.a2uiPush(ctx, &params)
	case "a2ui_reset":
		return t.a2uiReset(ctx, &params)
	default:
		return &Result{Content: "Unknown action: " + params.Action, IsError: true}, nil
	}
}

func (t *CanvasTool) present(ctx context.Context, params *CanvasParams) (*Result, error) {
	if params.URL == "" {
		return &Result{Content: "URL is required for present action", IsError: true}, nil
	}

	if t.PresentFunc == nil {
		return &Result{Content: "Canvas present not configured", IsError: true}, nil
	}

	opts := CanvasOptions{
		Target:   params.Target,
		Width:    params.Width,
		Height:   params.Height,
		MaxWidth: params.MaxWidth,
		X:        params.X,
		Y:        params.Y,
	}

	if err := t.PresentFunc(ctx, params.URL, opts); err != nil {
		return &Result{Content: "Present failed: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: "Canvas presented"}, nil
}

func (t *CanvasTool) hide(ctx context.Context, params *CanvasParams) (*Result, error) {
	if t.HideFunc == nil {
		return &Result{Content: "Canvas hide not configured", IsError: true}, nil
	}

	if err := t.HideFunc(ctx, params.Target); err != nil {
		return &Result{Content: "Hide failed: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: "Canvas hidden"}, nil
}

func (t *CanvasTool) navigate(ctx context.Context, params *CanvasParams) (*Result, error) {
	if params.URL == "" {
		return &Result{Content: "URL is required for navigate action", IsError: true}, nil
	}

	if t.NavigateFunc == nil {
		return &Result{Content: "Canvas navigate not configured", IsError: true}, nil
	}

	if err := t.NavigateFunc(ctx, params.URL, params.Target); err != nil {
		return &Result{Content: "Navigate failed: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: "Canvas navigated to " + params.URL}, nil
}

func (t *CanvasTool) eval(ctx context.Context, params *CanvasParams) (*Result, error) {
	if params.JavaScript == "" {
		return &Result{Content: "JavaScript is required for eval action", IsError: true}, nil
	}

	if t.EvalFunc == nil {
		return &Result{Content: "Canvas eval not configured", IsError: true}, nil
	}

	result, err := t.EvalFunc(ctx, params.JavaScript, params.Target)
	if err != nil {
		return &Result{Content: "Eval failed: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: result}, nil
}

func (t *CanvasTool) snapshot(ctx context.Context, params *CanvasParams) (*Result, error) {
	if t.SnapshotFunc == nil {
		return &Result{Content: "Canvas snapshot not configured", IsError: true}, nil
	}

	opts := SnapshotOptions{
		Format:  params.OutputFormat,
		Quality: params.Quality,
		DelayMs: params.DelayMs,
	}

	data, err := t.SnapshotFunc(ctx, params.Target, opts)
	if err != nil {
		return &Result{Content: "Snapshot failed: " + err.Error(), IsError: true}, nil
	}

	// 返回图片数据
	mimeType := "image/png"
	if opts.Format == "jpg" || opts.Format == "jpeg" {
		mimeType = "image/jpeg"
	}

	_ = data // TODO: 实际返回图片数据
	return &Result{
		Content: "Screenshot captured",
		Media: []MediaItem{
			{
				Type:     "image",
				MimeType: mimeType,
			},
		},
	}, nil
}

func (t *CanvasTool) a2uiPush(ctx context.Context, params *CanvasParams) (*Result, error) {
	// A2UI push 实现
	return &Result{Content: "A2UI push not yet implemented"}, nil
}

func (t *CanvasTool) a2uiReset(ctx context.Context, params *CanvasParams) (*Result, error) {
	// A2UI reset 实现
	return &Result{Content: "A2UI reset not yet implemented"}, nil
}

// 设置回调函数
func (t *CanvasTool) SetPresentFunc(fn func(ctx context.Context, url string, opts CanvasOptions) error) {
	t.PresentFunc = fn
}

func (t *CanvasTool) SetHideFunc(fn func(ctx context.Context, target string) error) {
	t.HideFunc = fn
}

func (t *CanvasTool) SetNavigateFunc(fn func(ctx context.Context, url, target string) error) {
	t.NavigateFunc = fn
}

func (t *CanvasTool) SetEvalFunc(fn func(ctx context.Context, js, target string) (string, error)) {
	t.EvalFunc = fn
}

func (t *CanvasTool) SetSnapshotFunc(fn func(ctx context.Context, target string, opts SnapshotOptions) ([]byte, error)) {
	t.SnapshotFunc = fn
}
