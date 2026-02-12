package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExecTool_SimpleCommand(t *testing.T) {
	tool := NewExecTool(os.TempDir())

	params := ExecParams{Command: "echo hello"}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error result: %s", result.Content)
	}
	if !strings.Contains(result.Content, "hello") {
		t.Errorf("Expected output to contain 'hello', got: %s", result.Content)
	}
}

func TestExecTool_WorkingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewExecTool(tmpDir)

	params := ExecParams{Command: "pwd"}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(result.Content, tmpDir) {
		t.Errorf("Expected working dir %s, got: %s", tmpDir, result.Content)
	}
}

func TestExecTool_CustomWorkdir(t *testing.T) {
	tool := NewExecTool("/tmp")
	customDir := t.TempDir()

	params := ExecParams{
		Command: "pwd",
		Workdir: customDir,
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(result.Content, customDir) {
		t.Errorf("Expected custom workdir %s, got: %s", customDir, result.Content)
	}
}

func TestExecTool_EnvVariables(t *testing.T) {
	tool := NewExecTool(os.TempDir())

	params := ExecParams{
		Command: "echo $MY_VAR",
		Env:     map[string]string{"MY_VAR": "test_value"},
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(result.Content, "test_value") {
		t.Errorf("Expected env var value, got: %s", result.Content)
	}
}

func TestExecTool_Timeout(t *testing.T) {
	tool := NewExecTool(os.TempDir())

	params := ExecParams{
		Command: "sleep 10",
		Timeout: 1, // 1 second
	}
	args, _ := json.Marshal(params)

	start := time.Now()
	result, err := tool.Execute(context.Background(), args)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error result for timeout")
	}
	if !strings.Contains(result.Content, "timed out") {
		t.Errorf("Expected timeout message, got: %s", result.Content)
	}
	if elapsed > 3*time.Second {
		t.Errorf("Timeout didn't work, elapsed: %v", elapsed)
	}
}

func TestExecTool_ExitCode(t *testing.T) {
	tool := NewExecTool(os.TempDir())

	params := ExecParams{Command: "exit 1"}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error result for non-zero exit")
	}
}

func TestExecTool_EmptyCommand(t *testing.T) {
	tool := NewExecTool(os.TempDir())

	params := ExecParams{Command: ""}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for empty command")
	}
	if !strings.Contains(result.Content, "required") {
		t.Errorf("Expected 'required' message, got: %s", result.Content)
	}
}

func TestExecTool_InvalidParams(t *testing.T) {
	tool := NewExecTool(os.TempDir())

	result, err := tool.Execute(context.Background(), []byte("invalid json"))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for invalid params")
	}
}

func TestExecTool_Background(t *testing.T) {
	tool := NewExecTool(os.TempDir())
	processTool := NewProcessTool(os.TempDir())
	tool.SetProcessTool(processTool)

	params := ExecParams{
		Command:    "sleep 5",
		Background: true,
	}
	args, _ := json.Marshal(params)

	start := time.Now()
	result, err := tool.Execute(context.Background(), args)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "session") {
		t.Errorf("Expected session ID in result, got: %s", result.Content)
	}
	if elapsed > 2*time.Second {
		t.Errorf("Background should return immediately, took: %v", elapsed)
	}
}

func TestExecTool_PTY(t *testing.T) {
	tool := NewExecTool(os.TempDir())
	processTool := NewProcessTool(os.TempDir())
	tool.SetProcessTool(processTool)

	params := ExecParams{
		Command: "echo PTY test",
		PTY:     true,
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "PTY") {
		t.Errorf("Expected PTY mode message, got: %s", result.Content)
	}
}
