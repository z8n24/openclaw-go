package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// BrowserTool 浏览器控制工具
type BrowserTool struct {
	allocCtx context.Context
	cancel   context.CancelFunc
	ctx      context.Context
	mu       sync.Mutex
	started  bool
}

type BrowserParams struct {
	Action    string            `json:"action"` // start, stop, navigate, snapshot, screenshot, click, type, etc.
	URL       string            `json:"url,omitempty"`
	Selector  string            `json:"selector,omitempty"`
	Text      string            `json:"text,omitempty"`
	Ref       string            `json:"ref,omitempty"`
	FullPage  bool              `json:"fullPage,omitempty"`
	TimeoutMs int               `json:"timeoutMs,omitempty"`
	Request   *BrowserActParams `json:"request,omitempty"`
}

type BrowserActParams struct {
	Kind     string `json:"kind"` // click, type, press, hover, scroll, wait
	Ref      string `json:"ref,omitempty"`
	Selector string `json:"selector,omitempty"`
	Text     string `json:"text,omitempty"`
	Key      string `json:"key,omitempty"`
	TimeMs   int    `json:"timeMs,omitempty"`
}

var globalBrowser *BrowserTool
var browserOnce sync.Once

func GetBrowserTool() *BrowserTool {
	browserOnce.Do(func() {
		globalBrowser = &BrowserTool{}
	})
	return globalBrowser
}

func NewBrowserTool() *BrowserTool {
	return GetBrowserTool()
}

func (t *BrowserTool) Name() string {
	return "browser"
}

func (t *BrowserTool) Description() string {
	return "Control web browser: start/stop, navigate, take screenshots, interact with elements."
}

func (t *BrowserTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["status", "start", "stop", "navigate", "snapshot", "screenshot", "act"],
				"description": "Browser action to perform"
			},
			"url": {"type": "string", "description": "URL to navigate to"},
			"selector": {"type": "string", "description": "CSS selector for element"},
			"fullPage": {"type": "boolean", "description": "Take full page screenshot"},
			"timeoutMs": {"type": "number", "description": "Timeout in milliseconds"},
			"request": {
				"type": "object",
				"properties": {
					"kind": {"type": "string", "enum": ["click", "type", "press", "hover", "scroll", "wait"]},
					"selector": {"type": "string"},
					"text": {"type": "string"},
					"key": {"type": "string"},
					"timeMs": {"type": "number"}
				}
			}
		},
		"required": ["action"]
	}`)
}

func (t *BrowserTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params BrowserParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	switch params.Action {
	case "status":
		return t.status()
	case "start":
		return t.start()
	case "stop":
		return t.stop()
	case "navigate":
		return t.navigate(ctx, params.URL, params.TimeoutMs)
	case "snapshot":
		return t.snapshot(ctx)
	case "screenshot":
		return t.screenshot(ctx, params.FullPage)
	case "act":
		if params.Request == nil {
			return &Result{Content: "Request is required for act action", IsError: true}, nil
		}
		return t.act(ctx, params.Request)
	default:
		return &Result{Content: "Unknown action: " + params.Action, IsError: true}, nil
	}
}

func (t *BrowserTool) status() (*Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.started {
		return &Result{Content: "Browser is running"}, nil
	}
	return &Result{Content: "Browser is not running"}, nil
}

func (t *BrowserTool) start() (*Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.started {
		return &Result{Content: "Browser already running"}, nil
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1920, 1080),
	)

	t.allocCtx, t.cancel = chromedp.NewExecAllocator(context.Background(), opts...)
	t.ctx, _ = chromedp.NewContext(t.allocCtx)

	// 启动浏览器
	if err := chromedp.Run(t.ctx); err != nil {
		return &Result{Content: "Failed to start browser: " + err.Error(), IsError: true}, nil
	}

	t.started = true
	return &Result{Content: "Browser started"}, nil
}

func (t *BrowserTool) stop() (*Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.started {
		return &Result{Content: "Browser not running"}, nil
	}

	if t.cancel != nil {
		t.cancel()
	}
	t.started = false
	return &Result{Content: "Browser stopped"}, nil
}

func (t *BrowserTool) ensureStarted() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.started {
		return nil
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1920, 1080),
	)

	t.allocCtx, t.cancel = chromedp.NewExecAllocator(context.Background(), opts...)
	t.ctx, _ = chromedp.NewContext(t.allocCtx)

	if err := chromedp.Run(t.ctx); err != nil {
		return err
	}

	t.started = true
	return nil
}

func (t *BrowserTool) navigate(ctx context.Context, url string, timeoutMs int) (*Result, error) {
	if url == "" {
		return &Result{Content: "URL is required", IsError: true}, nil
	}

	if err := t.ensureStarted(); err != nil {
		return &Result{Content: "Failed to start browser: " + err.Error(), IsError: true}, nil
	}

	timeout := 30 * time.Second
	if timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
	}

	navCtx, cancel := context.WithTimeout(t.ctx, timeout)
	defer cancel()

	var title string
	err := chromedp.Run(navCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.Title(&title),
	)
	if err != nil {
		return &Result{Content: "Navigation failed: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Navigated to: %s\nTitle: %s", url, title)}, nil
}

func (t *BrowserTool) snapshot(ctx context.Context) (*Result, error) {
	if err := t.ensureStarted(); err != nil {
		return &Result{Content: "Failed to start browser: " + err.Error(), IsError: true}, nil
	}

	var html string
	var url string
	var title string

	err := chromedp.Run(t.ctx,
		chromedp.Location(&url),
		chromedp.Title(&title),
		chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return err
			}
			html, err = dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
			return err
		}),
	)
	if err != nil {
		return &Result{Content: "Snapshot failed: " + err.Error(), IsError: true}, nil
	}

	// 简化 HTML，只保留主要内容
	content := extractTextFromHTML(html)
	if len(content) > 30000 {
		content = content[:30000] + "\n...[truncated]"
	}

	result := fmt.Sprintf("URL: %s\nTitle: %s\n\n%s", url, title, content)
	return &Result{Content: result}, nil
}

func (t *BrowserTool) screenshot(ctx context.Context, fullPage bool) (*Result, error) {
	if err := t.ensureStarted(); err != nil {
		return &Result{Content: "Failed to start browser: " + err.Error(), IsError: true}, nil
	}

	var buf []byte
	var url string

	actions := []chromedp.Action{
		chromedp.Location(&url),
	}

	if fullPage {
		actions = append(actions, chromedp.FullScreenshot(&buf, 90))
	} else {
		actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			buf, err = page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatPng).Do(ctx)
			return err
		}))
	}

	if err := chromedp.Run(t.ctx, actions...); err != nil {
		return &Result{Content: "Screenshot failed: " + err.Error(), IsError: true}, nil
	}

	// 返回 base64 编码的图片
	b64 := base64.StdEncoding.EncodeToString(buf)
	return &Result{
		Content: fmt.Sprintf("Screenshot taken (%d bytes)\nURL: %s\n[Image data: data:image/png;base64,%s...]", len(buf), url, b64[:100]),
		Media: []MediaItem{
			{Type: "image", MimeType: "image/png"},
		},
	}, nil
}

func (t *BrowserTool) act(ctx context.Context, req *BrowserActParams) (*Result, error) {
	if err := t.ensureStarted(); err != nil {
		return &Result{Content: "Failed to start browser: " + err.Error(), IsError: true}, nil
	}

	selector := req.Selector
	if selector == "" && req.Ref != "" {
		// 如果只有 ref，假设是某种选择器格式
		selector = req.Ref
	}

	var action chromedp.Action

	switch req.Kind {
	case "click":
		if selector == "" {
			return &Result{Content: "Selector is required for click", IsError: true}, nil
		}
		action = chromedp.Click(selector, chromedp.ByQuery)

	case "type":
		if selector == "" {
			return &Result{Content: "Selector is required for type", IsError: true}, nil
		}
		action = chromedp.SendKeys(selector, req.Text, chromedp.ByQuery)

	case "press":
		key := req.Key
		if key == "" {
			key = "Enter"
		}
		action = chromedp.KeyEvent(key)

	case "hover":
		if selector == "" {
			return &Result{Content: "Selector is required for hover", IsError: true}, nil
		}
		action = chromedp.MouseClickXY(0, 0) // TODO: proper hover

	case "scroll":
		action = chromedp.Evaluate(`window.scrollBy(0, 500)`, nil)

	case "wait":
		waitTime := req.TimeMs
		if waitTime <= 0 {
			waitTime = 1000
		}
		action = chromedp.Sleep(time.Duration(waitTime) * time.Millisecond)

	default:
		return &Result{Content: "Unknown action kind: " + req.Kind, IsError: true}, nil
	}

	timeout := 10 * time.Second
	actCtx, cancel := context.WithTimeout(t.ctx, timeout)
	defer cancel()

	if err := chromedp.Run(actCtx, action); err != nil {
		return &Result{Content: fmt.Sprintf("Action %s failed: %s", req.Kind, err.Error()), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Action %s completed", req.Kind)}, nil
}

// extractTextFromHTML 从 HTML 提取文本
func extractTextFromHTML(html string) string {
	// 简单实现：移除标签
	// 实际应该用 goquery
	re := regexp.MustCompile(`<script[^>]*>[\s\S]*?</script>`)
	html = re.ReplaceAllString(html, "")
	re = regexp.MustCompile(`<style[^>]*>[\s\S]*?</style>`)
	html = re.ReplaceAllString(html, "")
	re = regexp.MustCompile(`<[^>]+>`)
	text := re.ReplaceAllString(html, " ")
	re = regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}
