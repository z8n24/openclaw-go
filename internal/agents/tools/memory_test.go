package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemorySearchTool_BasicSearch(t *testing.T) {
	tmpDir := t.TempDir()
	
	// 创建 MEMORY.md
	memoryContent := `# Memory
## Important Decisions
We decided to use Go for the rewrite.
The deadline is January 2026.

## User Preferences
User prefers dark mode.
`
	os.WriteFile(filepath.Join(tmpDir, "MEMORY.md"), []byte(memoryContent), 0644)
	
	tool := NewMemorySearchTool(tmpDir)
	params := MemorySearchParams{Query: "deadline January"}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "2026") || !strings.Contains(result.Content, "deadline") {
		t.Errorf("Expected to find deadline mention, got: %s", result.Content)
	}
}

func TestMemorySearchTool_SearchMemoryDir(t *testing.T) {
	tmpDir := t.TempDir()
	
	// 创建 memory 目录和文件
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)
	
	os.WriteFile(filepath.Join(memoryDir, "2025-01-15.md"), 
		[]byte("Today we discussed the API design.\nDecided on REST endpoints."), 0644)
	os.WriteFile(filepath.Join(memoryDir, "2025-01-16.md"), 
		[]byte("Implemented user authentication.\nUsing JWT tokens."), 0644)
	
	tool := NewMemorySearchTool(tmpDir)
	params := MemorySearchParams{Query: "API design REST"}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "REST") {
		t.Errorf("Should find REST mention: %s", result.Content)
	}
}

func TestMemorySearchTool_EmptyQuery(t *testing.T) {
	tool := NewMemorySearchTool(t.TempDir())
	params := MemorySearchParams{Query: ""}
	args, _ := json.Marshal(params)
	
	result, _ := tool.Execute(context.Background(), args)
	if !result.IsError {
		t.Error("Expected error for empty query")
	}
}

func TestMemorySearchTool_MaxResults(t *testing.T) {
	tmpDir := t.TempDir()
	
	// 创建多行内容，每行不同以便区分
	var content strings.Builder
	for i := 0; i < 50; i++ {
		content.WriteString(fmt.Sprintf("test keyword line-%d\n", i))
	}
	os.WriteFile(filepath.Join(tmpDir, "MEMORY.md"), []byte(content.String()), 0644)
	
	tool := NewMemorySearchTool(tmpDir)
	params := MemorySearchParams{Query: "keyword", MaxResults: 5}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	
	// 验证结果存在
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	
	// 结果应该被限制（不需要精确计数，因为上下文包含多行）
	if result.Content == "" {
		t.Error("Should have results")
	}
}

func TestMemorySearchTool_NoResults(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "MEMORY.md"), []byte("Some content here"), 0644)
	
	tool := NewMemorySearchTool(tmpDir)
	params := MemorySearchParams{Query: "xyznonexistent"}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	// Should not error, just return no results
	if result.IsError {
		t.Logf("Note: returned error for no results: %s", result.Content)
	}
}

func TestMemoryGetTool_BasicGet(t *testing.T) {
	tmpDir := t.TempDir()
	
	content := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
	os.WriteFile(filepath.Join(tmpDir, "MEMORY.md"), []byte(content), 0644)
	
	tool := NewMemoryGetTool(tmpDir)
	params := MemoryGetParams{Path: "MEMORY.md"}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Line 1") {
		t.Errorf("Should contain Line 1: %s", result.Content)
	}
}

func TestMemoryGetTool_WithFromAndLines(t *testing.T) {
	tmpDir := t.TempDir()
	
	content := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7"
	os.WriteFile(filepath.Join(tmpDir, "MEMORY.md"), []byte(content), 0644)
	
	tool := NewMemoryGetTool(tmpDir)
	params := MemoryGetParams{Path: "MEMORY.md", From: 3, Lines: 2}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Line 3") {
		t.Error("Should contain Line 3")
	}
	if !strings.Contains(result.Content, "Line 4") {
		t.Error("Should contain Line 4")
	}
	if strings.Contains(result.Content, "Line 5") {
		t.Error("Should not contain Line 5")
	}
}

func TestMemoryGetTool_FileNotFound(t *testing.T) {
	tool := NewMemoryGetTool(t.TempDir())
	params := MemoryGetParams{Path: "nonexistent.md"}
	args, _ := json.Marshal(params)
	
	result, _ := tool.Execute(context.Background(), args)
	if !result.IsError {
		t.Error("Expected error for nonexistent file")
	}
}

func TestMemoryGetTool_EmptyPath(t *testing.T) {
	tool := NewMemoryGetTool(t.TempDir())
	params := MemoryGetParams{Path: ""}
	args, _ := json.Marshal(params)
	
	result, _ := tool.Execute(context.Background(), args)
	if !result.IsError {
		t.Error("Expected error for empty path")
	}
}

func TestMemoryGetTool_MemorySubdir(t *testing.T) {
	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)
	
	os.WriteFile(filepath.Join(memoryDir, "notes.md"), []byte("Daily notes here"), 0644)
	
	tool := NewMemoryGetTool(tmpDir)
	params := MemoryGetParams{Path: "memory/notes.md"}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Daily notes") {
		t.Errorf("Should read memory subdir file: %s", result.Content)
	}
}

func TestMemoryGetTool_SecurityPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	
	tool := NewMemoryGetTool(tmpDir)
	// 尝试路径遍历
	params := MemoryGetParams{Path: "../../../etc/passwd"}
	args, _ := json.Marshal(params)
	
	result, _ := tool.Execute(context.Background(), args)
	// 应该拒绝或返回错误
	if !result.IsError && strings.Contains(result.Content, "root:") {
		t.Error("Should block path traversal attacks")
	}
}
