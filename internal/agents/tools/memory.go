package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MemorySearchTool 语义搜索工具 (简化版本，使用关键词匹配)
type MemorySearchTool struct {
	workspace string
}

type MemorySearchParams struct {
	Query      string `json:"query"`
	MaxResults int    `json:"maxResults,omitempty"`
	MinScore   float64 `json:"minScore,omitempty"`
}

type MemorySearchResult struct {
	Path    string  `json:"path"`
	Line    int     `json:"line"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

func NewMemorySearchTool(workspace string) *MemorySearchTool {
	return &MemorySearchTool{workspace: workspace}
}

func (t *MemorySearchTool) Name() string {
	return "memory_search"
}

func (t *MemorySearchTool) Description() string {
	return "Search MEMORY.md and memory/*.md files for relevant content. Use before answering questions about prior work, decisions, or preferences."
}

func (t *MemorySearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query"},
			"maxResults": {"type": "number", "description": "Maximum results to return (default 10)"},
			"minScore": {"type": "number", "description": "Minimum relevance score (0-1)"}
		},
		"required": ["query"]
	}`)
}

func (t *MemorySearchTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params MemorySearchParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if params.Query == "" {
		return &Result{Content: "Query is required", IsError: true}, nil
	}

	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}

	minScore := params.MinScore
	if minScore <= 0 {
		minScore = 0.1
	}

	// 搜索的文件列表
	searchPaths := []string{
		filepath.Join(t.workspace, "MEMORY.md"),
	}

	// 添加 memory/*.md
	memoryDir := filepath.Join(t.workspace, "memory")
	if entries, err := os.ReadDir(memoryDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				searchPaths = append(searchPaths, filepath.Join(memoryDir, entry.Name()))
			}
		}
	}

	// 搜索关键词
	keywords := extractKeywords(params.Query)
	var results []MemorySearchResult

	for _, path := range searchPaths {
		fileResults := searchFile(path, keywords, t.workspace)
		results = append(results, fileResults...)
	}

	// 按分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 过滤低分结果
	var filtered []MemorySearchResult
	for _, r := range results {
		if r.Score >= minScore {
			filtered = append(filtered, r)
		}
	}

	// 限制结果数量
	if len(filtered) > maxResults {
		filtered = filtered[:maxResults]
	}

	if len(filtered) == 0 {
		return &Result{Content: "No matching results found"}, nil
	}

	// 格式化输出
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d results:\n\n", len(filtered)))
	
	for i, r := range filtered {
		relPath, _ := filepath.Rel(t.workspace, r.Path)
		sb.WriteString(fmt.Sprintf("### Result %d (score: %.2f)\n", i+1, r.Score))
		sb.WriteString(fmt.Sprintf("File: %s, Line: %d\n", relPath, r.Line))
		sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", r.Content))
	}

	return &Result{Content: sb.String()}, nil
}

// extractKeywords 提取搜索关键词
func extractKeywords(query string) []string {
	// 简单分词
	words := strings.Fields(strings.ToLower(query))
	
	// 过滤停用词
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true,
		"of": true, "in": true, "to": true, "for": true, "with": true,
		"on": true, "at": true, "by": true, "from": true, "as": true,
		"and": true, "or": true, "but": true, "not": true,
		"what": true, "when": true, "where": true, "who": true, "how": true, "why": true,
	}
	
	var keywords []string
	for _, word := range words {
		if len(word) > 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}
	
	return keywords
}

// searchFile 在文件中搜索关键词
func searchFile(path string, keywords []string, workspace string) []MemorySearchResult {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var results []MemorySearchResult
	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	// 收集上下文窗口
	var lines []string
	
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lines = append(lines, line)
	}

	// 搜索每一行
	for i, line := range lines {
		lowerLine := strings.ToLower(line)
		score := 0.0
		matchCount := 0
		
		for _, keyword := range keywords {
			if strings.Contains(lowerLine, keyword) {
				matchCount++
				score += 1.0 / float64(len(keywords))
			}
		}
		
		if matchCount > 0 {
			// 获取上下文 (前后各2行)
			start := i - 2
			if start < 0 {
				start = 0
			}
			end := i + 3
			if end > len(lines) {
				end = len(lines)
			}
			
			context := strings.Join(lines[start:end], "\n")
			
			results = append(results, MemorySearchResult{
				Path:    path,
				Line:    i + 1,
				Content: context,
				Score:   score,
			})
		}
	}

	return results
}

// MemoryGetTool 读取记忆文件片段
type MemoryGetTool struct {
	workspace string
}

type MemoryGetParams struct {
	Path  string `json:"path"`
	From  int    `json:"from,omitempty"`
	Lines int    `json:"lines,omitempty"`
}

func NewMemoryGetTool(workspace string) *MemoryGetTool {
	return &MemoryGetTool{workspace: workspace}
}

func (t *MemoryGetTool) Name() string {
	return "memory_get"
}

func (t *MemoryGetTool) Description() string {
	return "Read a snippet from MEMORY.md or memory/*.md files. Use after memory_search to get specific content."
}

func (t *MemoryGetTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path to the memory file (relative to workspace)"},
			"from": {"type": "number", "description": "Starting line number (1-indexed)"},
			"lines": {"type": "number", "description": "Number of lines to read (default 50)"}
		},
		"required": ["path"]
	}`)
}

func (t *MemoryGetTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params MemoryGetParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if params.Path == "" {
		return &Result{Content: "Path is required", IsError: true}, nil
	}

	// 安全检查：只允许读取 MEMORY.md 和 memory/ 下的文件
	allowed := false
	if params.Path == "MEMORY.md" || strings.HasPrefix(params.Path, "memory/") {
		allowed = true
	}
	if !allowed {
		return &Result{Content: "Can only read MEMORY.md or memory/*.md files", IsError: true}, nil
	}

	fullPath := filepath.Join(t.workspace, params.Path)
	
	file, err := os.Open(fullPath)
	if err != nil {
		return &Result{Content: "Failed to open file: " + err.Error(), IsError: true}, nil
	}
	defer file.Close()

	from := params.From
	if from < 1 {
		from = 1
	}

	lines := params.Lines
	if lines <= 0 {
		lines = 50
	}

	scanner := bufio.NewScanner(file)
	var content []string
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum >= from && lineNum < from+lines {
			content = append(content, scanner.Text())
		}
		if lineNum >= from+lines {
			break
		}
	}

	if len(content) == 0 {
		return &Result{Content: "No content found at specified location"}, nil
	}

	return &Result{Content: strings.Join(content, "\n")}, nil
}
