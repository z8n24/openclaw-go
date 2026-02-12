package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCanvasTool_Name(t *testing.T) {
	tool := NewCanvasTool()
	if tool.Name() != ToolCanvas {
		t.Errorf("Name should be %s, got %s", ToolCanvas, tool.Name())
	}
}

func TestCanvasTool_Description(t *testing.T) {
	tool := NewCanvasTool()
	desc := tool.Description()
	
	if !strings.Contains(desc, "canvas") {
		t.Error("Description should mention canvas")
	}
	if !strings.Contains(desc, "present") {
		t.Error("Description should mention present")
	}
	if !strings.Contains(desc, "snapshot") {
		t.Error("Description should mention snapshot")
	}
}

func TestCanvasTool_Parameters(t *testing.T) {
	tool := NewCanvasTool()
	params := tool.Parameters()
	
	var schema map[string]interface{}
	if err := json.Unmarshal(params, &schema); err != nil {
		t.Fatalf("Parameters should be valid JSON: %v", err)
	}
	
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema should have properties")
	}
	
	if _, ok := props["action"]; !ok {
		t.Error("Schema should have 'action' property")
	}
	if _, ok := props["url"]; !ok {
		t.Error("Schema should have 'url' property")
	}
}

func TestCanvasTool_PresentNoFunc(t *testing.T) {
	tool := NewCanvasTool()
	
	params := CanvasParams{
		Action: "present",
		URL:    "http://example.com",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error when PresentFunc is not set")
	}
}

func TestCanvasTool_PresentWithFunc(t *testing.T) {
	tool := NewCanvasTool()
	
	var presentedURL string
	tool.PresentFunc = func(ctx context.Context, url string, opts CanvasOptions) error {
		presentedURL = url
		return nil
	}
	
	params := CanvasParams{
		Action: "present",
		URL:    "http://example.com/page",
		Width:  800,
		Height: 600,
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if presentedURL != "http://example.com/page" {
		t.Errorf("URL mismatch: %s", presentedURL)
	}
}

func TestCanvasTool_Hide(t *testing.T) {
	tool := NewCanvasTool()
	
	var hiddenTarget string
	tool.HideFunc = func(ctx context.Context, target string) error {
		hiddenTarget = target
		return nil
	}
	
	params := CanvasParams{
		Action: "hide",
		Target: "main-canvas",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if hiddenTarget != "main-canvas" {
		t.Errorf("Target mismatch: %s", hiddenTarget)
	}
}

func TestCanvasTool_Navigate(t *testing.T) {
	tool := NewCanvasTool()
	
	var navURL, navTarget string
	tool.NavigateFunc = func(ctx context.Context, url, target string) error {
		navURL = url
		navTarget = target
		return nil
	}
	
	params := CanvasParams{
		Action: "navigate",
		URL:    "http://new.url",
		Target: "canvas-1",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if navURL != "http://new.url" {
		t.Errorf("URL mismatch: %s", navURL)
	}
	if navTarget != "canvas-1" {
		t.Errorf("Target mismatch: %s", navTarget)
	}
}

func TestCanvasTool_Eval(t *testing.T) {
	tool := NewCanvasTool()
	
	tool.EvalFunc = func(ctx context.Context, js, target string) (string, error) {
		return "eval result: " + js[:10], nil
	}
	
	params := CanvasParams{
		Action:     "eval",
		JavaScript: "document.title",
		Target:     "canvas-1",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "eval result") {
		t.Errorf("Result mismatch: %s", result.Content)
	}
}

func TestCanvasTool_Snapshot(t *testing.T) {
	tool := NewCanvasTool()
	
	tool.SnapshotFunc = func(ctx context.Context, target string, opts SnapshotOptions) ([]byte, error) {
		return []byte("fake-png-data"), nil
	}
	
	params := CanvasParams{
		Action:       "snapshot",
		Target:       "canvas-1",
		OutputFormat: "png",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
}

func TestCanvasTool_UnknownAction(t *testing.T) {
	tool := NewCanvasTool()
	
	params := CanvasParams{
		Action: "invalid",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for unknown action")
	}
}

func TestCanvasTool_InvalidParams(t *testing.T) {
	tool := NewCanvasTool()
	
	result, err := tool.Execute(context.Background(), []byte("invalid"))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for invalid params")
	}
}
