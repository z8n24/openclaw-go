package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditTool_SimpleReplace(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	tool := NewEditTool(tmpDir)
	params := EditParams{
		Path:    "test.txt",
		OldText: "world",
		NewText: "universe",
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
	if string(content) != "hello universe" {
		t.Errorf("Content mismatch: %s", string(content))
	}
}

func TestEditTool_MultiLineReplace(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "multi.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3"), 0644)

	tool := NewEditTool(tmpDir)
	params := EditParams{
		Path:    "multi.txt",
		OldText: "line2",
		NewText: "replaced\nwith\nmultiple",
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
	expected := "line1\nreplaced\nwith\nmultiple\nline3"
	if string(content) != expected {
		t.Errorf("Content mismatch:\nGot: %q\nExpected: %q", string(content), expected)
	}
}

func TestEditTool_TextNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	tool := NewEditTool(tmpDir)
	params := EditParams{
		Path:    "test.txt",
		OldText: "nonexistent",
		NewText: "replacement",
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for text not found")
	}
	if !strings.Contains(result.Content, "not found") {
		t.Errorf("Expected 'not found' error, got: %s", result.Content)
	}
}

func TestEditTool_MultipleMatches(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello hello hello"), 0644)

	tool := NewEditTool(tmpDir)
	params := EditParams{
		Path:    "test.txt",
		OldText: "hello",
		NewText: "hi",
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for multiple matches")
	}
	if !strings.Contains(result.Content, "3 matches") {
		t.Errorf("Expected match count error, got: %s", result.Content)
	}
}

func TestEditTool_FileNotFound(t *testing.T) {
	tool := NewEditTool(t.TempDir())
	params := EditParams{
		Path:    "nonexistent.txt",
		OldText: "old",
		NewText: "new",
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for nonexistent file")
	}
}

func TestEditTool_EmptyPath(t *testing.T) {
	tool := NewEditTool(t.TempDir())
	params := EditParams{
		Path:    "",
		OldText: "old",
		NewText: "new",
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for empty path")
	}
}

func TestEditTool_EmptyOldText(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("content"), 0644)

	tool := NewEditTool(tmpDir)
	params := EditParams{
		Path:    "test.txt",
		OldText: "",
		NewText: "new",
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for empty oldText")
	}
}

func TestEditTool_DeleteText(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	tool := NewEditTool(tmpDir)
	params := EditParams{
		Path:    "test.txt",
		OldText: " world",
		NewText: "",
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
	if string(content) != "hello" {
		t.Errorf("Content mismatch: %s", string(content))
	}
}

func TestEditTool_AliasParameters(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("old text"), 0644)

	tool := NewEditTool(tmpDir)
	// Using old_string/new_string aliases
	params := map[string]string{
		"path":       "test.txt",
		"old_string": "old",
		"new_string": "new",
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
	if string(content) != "new text" {
		t.Errorf("Content mismatch: %s", string(content))
	}
}

func TestEditTool_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "abs.txt")
	os.WriteFile(testFile, []byte("absolute"), 0644)

	tool := NewEditTool("/different/path")
	params := EditParams{
		Path:    testFile,
		OldText: "absolute",
		NewText: "relative",
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
	if string(content) != "relative" {
		t.Errorf("Content mismatch: %s", string(content))
	}
}

func TestEditTool_WhitespaceMatching(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "ws.txt")
	os.WriteFile(testFile, []byte("hello  world"), 0644)  // two spaces

	tool := NewEditTool(tmpDir)
	
	// This should fail - only one space
	params := EditParams{
		Path:    "ws.txt",
		OldText: "hello world",  // one space
		NewText: "hi",
	}
	args, _ := json.Marshal(params)

	result, _ := tool.Execute(context.Background(), args)
	if !result.IsError {
		t.Error("Should fail when whitespace doesn't match exactly")
	}

	// This should succeed - two spaces
	params2 := EditParams{
		Path:    "ws.txt",
		OldText: "hello  world",  // two spaces
		NewText: "hi there",
	}
	args2, _ := json.Marshal(params2)

	result2, _ := tool.Execute(context.Background(), args2)
	if result2.IsError {
		t.Errorf("Should succeed with exact whitespace: %s", result2.Content)
	}
}

func TestEditTool_InvalidParams(t *testing.T) {
	tool := NewEditTool(t.TempDir())
	result, err := tool.Execute(context.Background(), []byte("invalid"))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for invalid params")
	}
}
