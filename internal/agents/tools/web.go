package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
)

// ==================== Web Search ====================

// WebSearchTool 网页搜索工具 (使用 Brave Search API)
type WebSearchTool struct {
	apiKey     string
	httpClient *http.Client
}

type WebSearchParams struct {
	Query      string `json:"query"`
	Count      int    `json:"count,omitempty"`
	Country    string `json:"country,omitempty"`
	SearchLang string `json:"search_lang,omitempty"`
	Freshness  string `json:"freshness,omitempty"`
}

type BraveSearchResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}

func NewWebSearchTool() *WebSearchTool {
	apiKey := os.Getenv("BRAVE_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("BRAVE_SEARCH_API_KEY")
	}
	return &WebSearchTool{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *WebSearchTool) Name() string {
	return "web_search"
}

func (t *WebSearchTool) Description() string {
	return "Search the web using Brave Search API. Returns titles, URLs, and snippets."
}

func (t *WebSearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query string"},
			"count": {"type": "number", "description": "Number of results (1-10)"},
			"country": {"type": "string", "description": "2-letter country code (e.g., US, DE)"},
			"search_lang": {"type": "string", "description": "ISO language code (e.g., en, zh)"},
			"freshness": {"type": "string", "description": "Filter by time: pd (24h), pw (week), pm (month), py (year)"}
		},
		"required": ["query"]
	}`)
}

func (t *WebSearchTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params WebSearchParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if params.Query == "" {
		return &Result{Content: "Query is required", IsError: true}, nil
	}

	if t.apiKey == "" {
		return &Result{Content: "BRAVE_API_KEY not set", IsError: true}, nil
	}

	count := params.Count
	if count <= 0 || count > 10 {
		count = 5
	}

	// 构建请求 URL
	u, _ := url.Parse("https://api.search.brave.com/res/v1/web/search")
	q := u.Query()
	q.Set("q", params.Query)
	q.Set("count", fmt.Sprintf("%d", count))
	if params.Country != "" {
		q.Set("country", params.Country)
	}
	if params.SearchLang != "" {
		q.Set("search_lang", params.SearchLang)
	}
	if params.Freshness != "" {
		q.Set("freshness", params.Freshness)
	}
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return &Result{Content: "Search failed: " + err.Error(), IsError: true}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &Result{Content: fmt.Sprintf("Search API error %d: %s", resp.StatusCode, string(body)), IsError: true}, nil
	}

	var searchResp BraveSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return &Result{Content: "Failed to parse search results: " + err.Error(), IsError: true}, nil
	}

	// 格式化结果
	var sb strings.Builder
	for i, result := range searchResp.Web.Results {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, result.Title))
		sb.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
		sb.WriteString(fmt.Sprintf("   %s\n\n", result.Description))
	}

	if sb.Len() == 0 {
		return &Result{Content: "No results found"}, nil
	}

	return &Result{Content: sb.String()}, nil
}

// ==================== Web Fetch ====================

// WebFetchTool 网页抓取工具
type WebFetchTool struct {
	httpClient *http.Client
	converter  *md.Converter
}

type WebFetchParams struct {
	URL         string `json:"url"`
	ExtractMode string `json:"extractMode,omitempty"` // "markdown" or "text"
	MaxChars    int    `json:"maxChars,omitempty"`
}

func NewWebFetchTool() *WebFetchTool {
	converter := md.NewConverter("", true, nil)
	return &WebFetchTool{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		converter:  converter,
	}
}

func (t *WebFetchTool) Name() string {
	return "web_fetch"
}

func (t *WebFetchTool) Description() string {
	return "Fetch and extract readable content from a URL (HTML → markdown/text)."
}

func (t *WebFetchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "HTTP or HTTPS URL to fetch"},
			"extractMode": {"type": "string", "description": "Extraction mode: markdown or text", "enum": ["markdown", "text"]},
			"maxChars": {"type": "number", "description": "Maximum characters to return"}
		},
		"required": ["url"]
	}`)
}

func (t *WebFetchTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params WebFetchParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if params.URL == "" {
		return &Result{Content: "URL is required", IsError: true}, nil
	}

	// 验证 URL
	parsedURL, err := url.Parse(params.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return &Result{Content: "Invalid URL: must be http or https", IsError: true}, nil
	}

	mode := params.ExtractMode
	if mode == "" {
		mode = "markdown"
	}

	maxChars := params.MaxChars
	if maxChars <= 0 {
		maxChars = 50000
	}

	// 发起请求
	req, _ := http.NewRequestWithContext(ctx, "GET", params.URL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; OpenClaw/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return &Result{Content: "Fetch failed: " + err.Error(), IsError: true}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &Result{Content: fmt.Sprintf("HTTP error %d", resp.StatusCode), IsError: true}, nil
	}

	// 解析 HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return &Result{Content: "Failed to parse HTML: " + err.Error(), IsError: true}, nil
	}

	// 移除不需要的元素
	doc.Find("script, style, nav, footer, header, aside, .ad, .ads, .advertisement").Remove()

	// 提取主要内容
	var content string
	
	// 尝试找到主要内容区域
	mainSelectors := []string{"main", "article", "#content", ".content", "#main", ".main", "body"}
	for _, selector := range mainSelectors {
		selection := doc.Find(selector).First()
		if selection.Length() > 0 {
			html, _ := selection.Html()
			if mode == "markdown" {
				content, _ = t.converter.ConvertString(html)
			} else {
				content = selection.Text()
			}
			break
		}
	}

	// 清理内容
	content = cleanText(content)

	// 截断
	if len(content) > maxChars {
		content = content[:maxChars] + "\n\n[Truncated...]"
	}

	if content == "" {
		return &Result{Content: "No readable content found"}, nil
	}

	return &Result{Content: content}, nil
}

// cleanText 清理文本
func cleanText(s string) string {
	// 合并多个空行
	re := regexp.MustCompile(`\n{3,}`)
	s = re.ReplaceAllString(s, "\n\n")
	
	// 去除行首尾空白
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
