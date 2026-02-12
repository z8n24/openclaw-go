package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebFetchTool_BasicFetch(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
			<html>
			<head><title>Test Page</title></head>
			<body>
				<main>
					<h1>Hello World</h1>
					<p>This is test content.</p>
				</main>
			</body>
			</html>`))
	}))
	defer server.Close()

	tool := NewWebFetchTool()
	params := WebFetchParams{URL: server.URL}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Hello World") {
		t.Errorf("Expected 'Hello World', got: %s", result.Content)
	}
}

func TestWebFetchTool_TextMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><p>Plain text content</p></body></html>`))
	}))
	defer server.Close()

	tool := NewWebFetchTool()
	params := WebFetchParams{
		URL:         server.URL,
		ExtractMode: "text",
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Plain text content") {
		t.Errorf("Expected text content, got: %s", result.Content)
	}
}

func TestWebFetchTool_MaxChars(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Long content
		content := strings.Repeat("x", 10000)
		w.Write([]byte(`<html><body><main>` + content + `</main></body></html>`))
	}))
	defer server.Close()

	tool := NewWebFetchTool()
	params := WebFetchParams{
		URL:      server.URL,
		MaxChars: 100,
	}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if len(result.Content) > 200 { // Some overhead for truncation message
		t.Errorf("Expected truncated content, got %d chars", len(result.Content))
	}
	if !strings.Contains(result.Content, "Truncated") {
		t.Error("Expected truncation message")
	}
}

func TestWebFetchTool_RemovesScripts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>
			<script>alert('bad')</script>
			<main>Good content</main>
			<style>.bad{}</style>
		</body></html>`))
	}))
	defer server.Close()

	tool := NewWebFetchTool()
	params := WebFetchParams{URL: server.URL}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if strings.Contains(result.Content, "alert") {
		t.Error("Script content should be removed")
	}
	if !strings.Contains(result.Content, "Good content") {
		t.Error("Main content should be preserved")
	}
}

func TestWebFetchTool_404Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tool := NewWebFetchTool()
	params := WebFetchParams{URL: server.URL}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for 404")
	}
	if !strings.Contains(result.Content, "404") {
		t.Errorf("Expected 404 in error, got: %s", result.Content)
	}
}

func TestWebFetchTool_InvalidURL(t *testing.T) {
	tool := NewWebFetchTool()
	params := WebFetchParams{URL: "not-a-valid-url"}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for invalid URL")
	}
}

func TestWebFetchTool_EmptyURL(t *testing.T) {
	tool := NewWebFetchTool()
	params := WebFetchParams{URL: ""}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for empty URL")
	}
}

func TestWebFetchTool_NonHTTPURL(t *testing.T) {
	tool := NewWebFetchTool()
	params := WebFetchParams{URL: "ftp://example.com/file"}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for non-HTTP URL")
	}
}

func TestWebSearchTool_NoAPIKey(t *testing.T) {
	tool := NewWebSearchTool()
	tool.apiKey = "" // Ensure no API key

	params := WebSearchParams{Query: "test"}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error when API key not set")
	}
	if !strings.Contains(result.Content, "API_KEY") {
		t.Errorf("Expected API key error, got: %s", result.Content)
	}
}

func TestWebSearchTool_EmptyQuery(t *testing.T) {
	tool := NewWebSearchTool()
	tool.apiKey = "test-key"

	params := WebSearchParams{Query: ""}
	args, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for empty query")
	}
}

func TestCleanText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello\n\n\n\nworld", "hello\n\nworld"},
		{"  spaces  ", "spaces"},
		{"\n\n\nstart", "start"},
		{"end\n\n\n", "end"},
	}

	for _, tt := range tests {
		result := cleanText(tt.input)
		if result != tt.expected {
			t.Errorf("cleanText(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
