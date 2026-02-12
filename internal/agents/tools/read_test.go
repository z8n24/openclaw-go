package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTool_SimpleFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3"), 0644)

	tool := NewReadTool(tmpDir)
	params := ReadParams{Path: "test.txt"}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "line1") {
		t.Errorf("Expected content, got: %s", result.Content)
	}
}

func TestReadTool_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "abs.txt")
	os.WriteFile(testFile, []byte("absolute content"), 0644)

	tool := NewReadTool("/different/path")
	params := ReadParams{Path: testFile}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "absolute content") {
		t.Errorf("Expected content, got: %s", result.Content)
	}
}

func TestReadTool_Offset(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "lines.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\nline5"), 0644)

	tool := NewReadTool(tmpDir)
	params := ReadParams{Path: "lines.txt", Offset: 3}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if strings.Contains(result.Content, "line1") {
		t.Error("Should not contain line1 with offset 3")
	}
	if !strings.Contains(result.Content, "line3") {
		t.Error("Should contain line3 with offset 3")
	}
}

func TestReadTool_Limit(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "lines.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\nline5"), 0644)

	tool := NewReadTool(tmpDir)
	params := ReadParams{Path: "lines.txt", Limit: 2}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(result.Content, "line1") {
		t.Error("Should contain line1")
	}
	if !strings.Contains(result.Content, "line2") {
		t.Error("Should contain line2")
	}
	if strings.Contains(result.Content, "line3") {
		t.Error("Should not contain line3 with limit 2")
	}
}

func TestReadTool_OffsetAndLimit(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "lines.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\nline5"), 0644)

	tool := NewReadTool(tmpDir)
	params := ReadParams{Path: "lines.txt", Offset: 2, Limit: 2}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if strings.Contains(result.Content, "line1") {
		t.Error("Should not contain line1")
	}
	if !strings.Contains(result.Content, "line2") {
		t.Error("Should contain line2")
	}
	if !strings.Contains(result.Content, "line3") {
		t.Error("Should contain line3")
	}
	if strings.Contains(result.Content, "line4") {
		t.Error("Should not contain line4")
	}
}

func TestReadTool_FileNotFound(t *testing.T) {
	tool := NewReadTool(t.TempDir())
	params := ReadParams{Path: "nonexistent.txt"}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for nonexistent file")
	}
}

func TestReadTool_EmptyPath(t *testing.T) {
	tool := NewReadTool(t.TempDir())
	params := ReadParams{Path: ""}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for empty path")
	}
}

func TestReadTool_InvalidParams(t *testing.T) {
	tool := NewReadTool(t.TempDir())

	result, err := tool.Execute(context.Background(), []byte("invalid"))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for invalid params")
	}
}

func TestReadTool_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewReadTool(tmpDir)

	params := ReadParams{Path: tmpDir}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	// Reading a directory should fail
	if !result.IsError {
		t.Error("Expected error when reading directory")
	}
}

func TestReadTool_BinaryFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "binary.bin")
	os.WriteFile(testFile, []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}, 0644)

	tool := NewReadTool(tmpDir)
	params := ReadParams{Path: "binary.bin"}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	// Binary files should be read (though content might look weird)
	// Just verify it doesn't crash and returns something
	_ = result // used
}

func TestReadTool_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")
	
	// Create a file with many lines
	var content strings.Builder
	for i := 0; i < 3000; i++ {
		content.WriteString("This is a test line\n")
	}
	os.WriteFile(testFile, []byte(content.String()), 0644)

	tool := NewReadTool(tmpDir)
	params := ReadParams{Path: "large.txt"}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	// Should be truncated
	if len(result.Content) > 60000 { // Allow some overhead for messages
		t.Error("Large file should be truncated")
	}
}
