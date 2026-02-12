package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkReadTool_SmallFile(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "small.txt")
	os.WriteFile(testFile, []byte("Hello, World!"), 0644)
	
	tool := NewReadTool(tmpDir)
	params := ReadParams{Path: "small.txt"}
	args, _ := json.Marshal(params)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool.Execute(ctx, args)
	}
}

func BenchmarkReadTool_LargeFile(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")
	
	// 创建 100KB 文件
	content := make([]byte, 100*1024)
	for i := range content {
		content[i] = byte('a' + (i % 26))
	}
	os.WriteFile(testFile, content, 0644)
	
	tool := NewReadTool(tmpDir)
	params := ReadParams{Path: "large.txt"}
	args, _ := json.Marshal(params)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool.Execute(ctx, args)
	}
}

func BenchmarkWriteTool(b *testing.B) {
	tmpDir := b.TempDir()
	tool := NewWriteTool(tmpDir)
	params := WriteParams{Path: "test.txt", Content: "Hello, World!"}
	args, _ := json.Marshal(params)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool.Execute(ctx, args)
	}
}

func BenchmarkEditTool(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "edit.txt")
	
	tool := NewEditTool(tmpDir)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 重置文件
		os.WriteFile(testFile, []byte("old content here"), 0644)
		
		params := EditParams{
			Path:    "edit.txt",
			OldText: "old",
			NewText: "new",
		}
		args, _ := json.Marshal(params)
		tool.Execute(ctx, args)
	}
}

func BenchmarkExecTool_SimpleCommand(b *testing.B) {
	tool := NewExecTool(os.TempDir())
	params := ExecParams{Command: "echo hello"}
	args, _ := json.Marshal(params)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool.Execute(ctx, args)
	}
}

func BenchmarkWebFetchTool_Parse(b *testing.B) {
	// 只测试解析，不做网络请求
	_ = NewWebFetchTool() // 确保初始化可用
	
	html := `<!DOCTYPE html>
	<html><head><title>Test</title></head>
	<body><main><h1>Hello</h1><p>World</p></main></body>
	</html>`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cleanText(html)
	}
}

func BenchmarkToolRegistry_Get(b *testing.B) {
	registry := NewRegistry()
	registry.Register(NewReadTool("/tmp"))
	registry.Register(NewWriteTool("/tmp"))
	registry.Register(NewEditTool("/tmp"))
	registry.Register(NewExecTool("/tmp"))
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Get("read")
		registry.Get("write")
		registry.Get("edit")
		registry.Get("exec")
	}
}

func BenchmarkToolRegistry_Execute(b *testing.B) {
	tmpDir := b.TempDir()
	registry := NewRegistry()
	registry.Register(NewReadTool(tmpDir))
	
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	args := json.RawMessage(`{"path":"test.txt"}`)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Execute(ctx, "read", args)
	}
}
