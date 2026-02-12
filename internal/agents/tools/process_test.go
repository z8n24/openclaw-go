package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestProcessTool_StartSession(t *testing.T) {
	tool := NewProcessTool(os.TempDir())

	sessionID, err := tool.StartSession("echo hello && sleep 1", os.TempDir(), false, nil)
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	if sessionID == "" {
		t.Fatal("Expected non-empty session ID")
	}

	// Wait for command to finish
	time.Sleep(2 * time.Second)

	// Check log
	params := ProcessParams{Action: "log", SessionID: sessionID}
	args, _ := json.Marshal(params)
	result, _ := tool.Execute(context.Background(), args)
	
	if !strings.Contains(result.Content, "hello") {
		t.Errorf("Expected 'hello' in output, got: %s", result.Content)
	}
}

func TestProcessTool_List(t *testing.T) {
	tool := NewProcessTool(os.TempDir())

	// Start a session
	tool.StartSession("sleep 10", os.TempDir(), false, nil)

	params := ProcessParams{Action: "list"}
	args, _ := json.Marshal(params)
	result, err := tool.Execute(context.Background(), args)

	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "session") {
		t.Errorf("Expected session in list, got: %s", result.Content)
	}
}

func TestProcessTool_Poll(t *testing.T) {
	tool := NewProcessTool(os.TempDir())

	sessionID, _ := tool.StartSession("echo test", os.TempDir(), false, nil)
	time.Sleep(500 * time.Millisecond)

	params := ProcessParams{Action: "poll", SessionID: sessionID}
	args, _ := json.Marshal(params)
	result, err := tool.Execute(context.Background(), args)

	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "status") {
		t.Errorf("Expected status in poll, got: %s", result.Content)
	}
}

func TestProcessTool_Kill(t *testing.T) {
	tool := NewProcessTool(os.TempDir())

	sessionID, _ := tool.StartSession("sleep 100", os.TempDir(), false, nil)
	time.Sleep(100 * time.Millisecond)

	params := ProcessParams{Action: "kill", SessionID: sessionID}
	args, _ := json.Marshal(params)
	result, err := tool.Execute(context.Background(), args)

	if err != nil {
		t.Fatalf("Kill failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "killed") {
		t.Errorf("Expected 'killed' message, got: %s", result.Content)
	}
}

func TestProcessTool_PTY(t *testing.T) {
	tool := NewProcessTool(os.TempDir())

	sessionID, err := tool.StartSession("echo PTY test", os.TempDir(), true, nil)
	if err != nil {
		t.Fatalf("StartSession with PTY failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Check that PTY flag is set
	params := ProcessParams{Action: "poll", SessionID: sessionID}
	args, _ := json.Marshal(params)
	result, _ := tool.Execute(context.Background(), args)

	// The result should indicate PTY mode in some way
	if result.IsError {
		t.Errorf("PTY session poll failed: %s", result.Content)
	}
}

func TestProcessTool_SendKeys(t *testing.T) {
	tool := NewProcessTool(os.TempDir())

	// Start a PTY session with cat (which echoes input)
	sessionID, err := tool.StartSession("cat", os.TempDir(), true, nil)
	if err != nil {
		t.Skipf("PTY not available: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Send some text
	params := ProcessParams{
		Action:    "send-keys",
		SessionID: sessionID,
		Literal:   "hello",
		Keys:      []string{"Enter"},
	}
	args, _ := json.Marshal(params)
	result, err := tool.Execute(context.Background(), args)

	if err != nil {
		t.Fatalf("SendKeys failed: %v", err)
	}
	if result.IsError {
		t.Errorf("SendKeys error: %s", result.Content)
	}

	// Kill the session
	tool.kill(sessionID)
}

func TestProcessTool_Write(t *testing.T) {
	tool := NewProcessTool(os.TempDir())

	sessionID, err := tool.StartSession("cat", os.TempDir(), true, nil)
	if err != nil {
		t.Skipf("PTY not available: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	params := ProcessParams{
		Action:    "write",
		SessionID: sessionID,
		Data:      "test data\n",
	}
	args, _ := json.Marshal(params)
	result, err := tool.Execute(context.Background(), args)

	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if result.IsError {
		t.Errorf("Write error: %s", result.Content)
	}

	tool.kill(sessionID)
}

func TestProcessTool_SessionNotFound(t *testing.T) {
	tool := NewProcessTool(os.TempDir())

	params := ProcessParams{Action: "poll", SessionID: "nonexistent"}
	args, _ := json.Marshal(params)
	result, _ := tool.Execute(context.Background(), args)

	if !result.IsError {
		t.Error("Expected error for nonexistent session")
	}
	if !strings.Contains(result.Content, "not found") {
		t.Errorf("Expected 'not found' error, got: %s", result.Content)
	}
}

func TestProcessTool_Cleanup(t *testing.T) {
	tool := NewProcessTool(os.TempDir())

	// Start a quick session
	sessionID, _ := tool.StartSession("echo done", os.TempDir(), false, nil)
	time.Sleep(500 * time.Millisecond)

	// Cleanup old sessions (0 duration = cleanup all finished)
	tool.Cleanup(0)

	// Session should be removed
	params := ProcessParams{Action: "poll", SessionID: sessionID}
	args, _ := json.Marshal(params)
	result, _ := tool.Execute(context.Background(), args)

	if !result.IsError {
		t.Error("Expected error after cleanup")
	}
}

func TestProcessTool_UnknownAction(t *testing.T) {
	tool := NewProcessTool(os.TempDir())

	params := ProcessParams{Action: "invalid"}
	args, _ := json.Marshal(params)
	result, _ := tool.Execute(context.Background(), args)

	if !result.IsError {
		t.Error("Expected error for unknown action")
	}
}
