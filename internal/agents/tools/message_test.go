package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMessageTool_Name(t *testing.T) {
	tool := NewMessageTool()
	if tool.Name() != ToolMessage {
		t.Errorf("Name should be %s, got %s", ToolMessage, tool.Name())
	}
}

func TestMessageTool_Description(t *testing.T) {
	tool := NewMessageTool()
	desc := tool.Description()
	
	if !strings.Contains(desc, "send") {
		t.Error("Description should mention send")
	}
	if !strings.Contains(desc, "broadcast") {
		t.Error("Description should mention broadcast")
	}
}

func TestMessageTool_Parameters(t *testing.T) {
	tool := NewMessageTool()
	params := tool.Parameters()
	
	var schema map[string]interface{}
	if err := json.Unmarshal(params, &schema); err != nil {
		t.Fatalf("Parameters should be valid JSON: %v", err)
	}
	
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema should have properties")
	}
	
	requiredProps := []string{"action", "channel", "target", "message"}
	for _, prop := range requiredProps {
		if _, ok := props[prop]; !ok {
			t.Errorf("Schema should have '%s' property", prop)
		}
	}
}

func TestMessageTool_NoSendFunc(t *testing.T) {
	tool := NewMessageTool()
	// SendFunc 未设置
	
	params := MessageParams{
		Action:  "send",
		Channel: "telegram",
		Target:  "12345",
		Message: "Hello",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error when SendFunc is not set")
	}
}

func TestMessageTool_SendAction(t *testing.T) {
	tool := NewMessageTool()
	
	var sentChannel, sentTarget, sentMessage string
	tool.SendFunc = func(ctx context.Context, channel, target, message string, opts MessageOptions) error {
		sentChannel = channel
		sentTarget = target
		sentMessage = message
		return nil
	}
	
	params := MessageParams{
		Action:  "send",
		Channel: "telegram",
		Target:  "12345",
		Message: "Hello World",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	
	if sentChannel != "telegram" {
		t.Errorf("Channel mismatch: %s", sentChannel)
	}
	if sentTarget != "12345" {
		t.Errorf("Target mismatch: %s", sentTarget)
	}
	if sentMessage != "Hello World" {
		t.Errorf("Message mismatch: %s", sentMessage)
	}
}

func TestMessageTool_BroadcastAction(t *testing.T) {
	tool := NewMessageTool()
	
	var sentTargets []string
	tool.SendFunc = func(ctx context.Context, channel, target, message string, opts MessageOptions) error {
		sentTargets = append(sentTargets, target)
		return nil
	}
	
	params := MessageParams{
		Action:  "broadcast",
		Channel: "telegram",
		Targets: []string{"111", "222", "333"},
		Message: "Broadcast message",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	
	if len(sentTargets) != 3 {
		t.Errorf("Expected 3 targets, got %d", len(sentTargets))
	}
}

func TestMessageTool_SendWithOptions(t *testing.T) {
	tool := NewMessageTool()
	
	var receivedOpts MessageOptions
	tool.SendFunc = func(ctx context.Context, channel, target, message string, opts MessageOptions) error {
		receivedOpts = opts
		return nil
	}
	
	params := MessageParams{
		Action:  "send",
		Channel: "telegram",
		Target:  "12345",
		Message: "Hello",
		Silent:  true,
		ReplyTo: "msg-999",
		AsVoice: true,
	}
	args, _ := json.Marshal(params)
	
	tool.Execute(context.Background(), args)
	
	if !receivedOpts.Silent {
		t.Error("Silent should be true")
	}
	if receivedOpts.ReplyTo != "msg-999" {
		t.Errorf("ReplyTo mismatch: %s", receivedOpts.ReplyTo)
	}
	if !receivedOpts.AsVoice {
		t.Error("AsVoice should be true")
	}
}

func TestMessageTool_EmptyMessage(t *testing.T) {
	tool := NewMessageTool()
	tool.SendFunc = func(ctx context.Context, channel, target, message string, opts MessageOptions) error {
		return nil
	}
	
	params := MessageParams{
		Action:  "send",
		Channel: "telegram",
		Target:  "12345",
		Message: "",
	}
	args, _ := json.Marshal(params)
	
	result, _ := tool.Execute(context.Background(), args)
	if !result.IsError {
		t.Error("Expected error for empty message")
	}
}

func TestMessageTool_EmptyTarget(t *testing.T) {
	tool := NewMessageTool()
	tool.SendFunc = func(ctx context.Context, channel, target, message string, opts MessageOptions) error {
		return nil
	}
	
	params := MessageParams{
		Action:  "send",
		Channel: "telegram",
		Target:  "",
		Message: "Hello",
	}
	args, _ := json.Marshal(params)
	
	result, _ := tool.Execute(context.Background(), args)
	if !result.IsError {
		t.Error("Expected error for empty target")
	}
}

func TestMessageTool_InvalidParams(t *testing.T) {
	tool := NewMessageTool()
	
	result, err := tool.Execute(context.Background(), []byte("invalid"))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for invalid params")
	}
}

func TestMessageTool_UnknownAction(t *testing.T) {
	tool := NewMessageTool()
	
	params := MessageParams{
		Action: "unknown",
	}
	args, _ := json.Marshal(params)
	
	result, _ := tool.Execute(context.Background(), args)
	if !result.IsError {
		t.Error("Expected error for unknown action")
	}
}
