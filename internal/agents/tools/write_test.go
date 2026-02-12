package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTool_CreateFile(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteTool(tmpDir)

	params := WriteParams{
		Path:    "test.txt",
		Content: "hello world",
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Created") {
		t.Errorf("Expected 'Created' message, got: %s", result.Content)
	}

	// Verify file content
	content, _ := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if string(content) != "hello world" {
		t.Errorf("File content mismatch: %s", string(content))
	}
}

func TestWriteTool_OverwriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "existing.txt")
	os.WriteFile(testFile, []byte("old content"), 0644)

	tool := NewWriteTool(tmpDir)
	params := WriteParams{
		Path:    "existing.txt",
		Content: "new content",
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Updated") {
		t.Errorf("Expected 'Updated' message, got: %s", result.Content)
	}

	// Verify file content
	content, _ := os.ReadFile(testFile)
	if string(content) != "new content" {
		t.Errorf("File content mismatch: %s", string(content))
	}
}

func TestWriteTool_CreateDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteTool(tmpDir)

	params := WriteParams{
		Path:    "nested/dir/file.txt",
		Content: "nested content",
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}

	// Verify file exists
	content, err := os.ReadFile(filepath.Join(tmpDir, "nested/dir/file.txt"))
	if err != nil {
		t.Fatalf("File should exist: %v", err)
	}
	if string(content) != "nested content" {
		t.Errorf("Content mismatch: %s", string(content))
	}
}

func TestWriteTool_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteTool("/different/path")

	testFile := filepath.Join(tmpDir, "abs.txt")
	params := WriteParams{
		Path:    testFile,
		Content: "absolute path content",
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}

	content, _ := os.ReadFile(testFile)
	if string(content) != "absolute path content" {
		t.Errorf("Content mismatch: %s", string(content))
	}
}

func TestWriteTool_EmptyPath(t *testing.T) {
	tool := NewWriteTool(t.TempDir())
	params := WriteParams{Path: "", Content: "test"}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for empty path")
	}
}

func TestWriteTool_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteTool(tmpDir)
	params := WriteParams{Path: "empty.txt", Content: ""}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	// Empty content should be allowed
	if result.IsError {
		t.Errorf("Should allow empty content: %s", result.Content)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "empty.txt"))
	if len(content) != 0 {
		t.Error("File should be empty")
	}
}

func TestWriteTool_InvalidParams(t *testing.T) {
	tool := NewWriteTool(t.TempDir())
	result, err := tool.Execute(context.Background(), []byte("invalid"))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for invalid params")
	}
}

func TestWriteTool_ByteCount(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteTool(tmpDir)
	
	content := "12345"
	params := WriteParams{Path: "count.txt", Content: content}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(result.Content, "5 bytes") {
		t.Errorf("Expected byte count in message, got: %s", result.Content)
	}
}
