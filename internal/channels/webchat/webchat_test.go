package webchat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/z8n24/openclaw-go/internal/channels"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter(ch *Channel) *gin.Engine {
	router := gin.New()
	group := router.Group("/webchat")
	ch.RegisterRoutes(group)
	return router
}

func TestChannel_IDAndLabel(t *testing.T) {
	ch := New()
	
	if ch.ID() != channels.ChannelWebChat {
		t.Errorf("ID mismatch: got %s", ch.ID())
	}
	if ch.Label() != "WebChat" {
		t.Errorf("Label mismatch: got %s", ch.Label())
	}
}

func TestChannel_StartStop(t *testing.T) {
	ch := New()
	
	if err := ch.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	
	status := ch.Status()
	if !status.Connected {
		t.Error("Should be connected after start")
	}
	
	if err := ch.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	
	status = ch.Status()
	if status.Connected {
		t.Error("Should be disconnected after stop")
	}
}

func TestChannel_HandleMessage(t *testing.T) {
	ch := New()
	ch.Start(context.Background())
	defer ch.Stop()
	
	received := make(chan *channels.InboundMessage, 1)
	ch.SetMessageHandler(func(msg *channels.InboundMessage) {
		received <- msg
	})
	
	router := setupTestRouter(ch)
	
	// 发送消息
	body := `{"sessionId":"session-123","text":"Hello"}`
	req := httptest.NewRequest("POST", "/webchat/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	
	// 验证响应
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	
	if resp["sessionId"] != "session-123" {
		t.Errorf("SessionId mismatch: %v", resp["sessionId"])
	}
	
	// 等待消息处理
	select {
	case msg := <-received:
		if msg.Text != "Hello" {
			t.Errorf("Text mismatch: %s", msg.Text)
		}
		if msg.ChatID != "session-123" {
			t.Errorf("ChatID mismatch: %s", msg.ChatID)
		}
	case <-time.After(time.Second):
		t.Error("Message not received")
	}
}

func TestChannel_HandleMessageEmpty(t *testing.T) {
	ch := New()
	ch.Start(context.Background())
	defer ch.Stop()
	
	router := setupTestRouter(ch)
	
	body := `{"sessionId":"session-123","text":""}`
	req := httptest.NewRequest("POST", "/webchat/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty message, got %d", w.Code)
	}
}

func TestChannel_HandleStatus(t *testing.T) {
	ch := New()
	ch.Start(context.Background())
	defer ch.Stop()
	
	router := setupTestRouter(ch)
	
	req := httptest.NewRequest("GET", "/webchat/status", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	
	if resp["connected"] != true {
		t.Error("Should show connected")
	}
}

func TestChannel_Send(t *testing.T) {
	ch := New()
	ch.Start(context.Background())
	defer ch.Stop()
	
	// 没有活跃客户端时发送应该失败
	_, err := ch.Send(context.Background(), &channels.OutboundMessage{
		ChatID: "session-123",
		Text:   "Hello",
	})
	
	if err == nil {
		t.Error("Expected error when no active client")
	}
}

func TestChannel_SendDelta(t *testing.T) {
	ch := New()
	ch.Start(context.Background())
	defer ch.Stop()
	
	// 添加一个模拟客户端
	client := &Client{
		ID:        "client-1",
		SessionID: "session-123",
		Events:    make(chan *SSEEvent, 10),
		Done:      make(chan struct{}),
	}
	
	ch.mu.Lock()
	ch.clients[client.ID] = client
	ch.mu.Unlock()
	
	// 发送 delta
	ch.SendDelta("session-123", "msg-1", "partial content")
	
	select {
	case event := <-client.Events:
		if event.Event != "delta" {
			t.Errorf("Event type mismatch: %s", event.Event)
		}
	case <-time.After(time.Second):
		t.Error("Delta event not received")
	}
}

func TestChannel_SendDone(t *testing.T) {
	ch := New()
	ch.Start(context.Background())
	defer ch.Stop()
	
	client := &Client{
		ID:        "client-1",
		SessionID: "session-123",
		Events:    make(chan *SSEEvent, 10),
		Done:      make(chan struct{}),
	}
	
	ch.mu.Lock()
	ch.clients[client.ID] = client
	ch.mu.Unlock()
	
	ch.SendDone("session-123", "msg-1")
	
	select {
	case event := <-client.Events:
		if event.Event != "done" {
			t.Errorf("Event type mismatch: %s", event.Event)
		}
	case <-time.After(time.Second):
		t.Error("Done event not received")
	}
}

func TestChannel_Capabilities(t *testing.T) {
	ch := New()
	caps := ch.Capabilities()
	
	if !caps.SupportsImages {
		t.Error("Should support images")
	}
	if !caps.SupportsMarkdown {
		t.Error("Should support markdown")
	}
	if caps.SupportsButtons {
		t.Error("Should not support buttons")
	}
}

func TestDetectAttachmentType(t *testing.T) {
	tests := []struct {
		mimeType string
		expected channels.AttachmentType
	}{
		{"image/png", channels.AttachmentTypeImage},
		{"image/jpeg", channels.AttachmentTypeImage},
		{"audio/mp3", channels.AttachmentTypeAudio},
		{"video/mp4", channels.AttachmentTypeVideo},
		{"application/pdf", channels.AttachmentTypeDocument},
		{"text/plain", channels.AttachmentTypeDocument},
	}
	
	for _, tt := range tests {
		result := detectAttachmentType(tt.mimeType)
		if result != tt.expected {
			t.Errorf("detectAttachmentType(%s) = %s, want %s", tt.mimeType, result, tt.expected)
		}
	}
}

// SSE 连接测试需要更复杂的设置，这里简化处理
func TestChannel_SSEEndpoint(t *testing.T) {
	ch := New()
	ch.Start(context.Background())
	defer ch.Stop()
	
	router := setupTestRouter(ch)
	
	// 创建请求但不等待响应（SSE 是长连接）
	req := httptest.NewRequest("GET", "/webchat/events?sessionId=test-123", nil)
	w := httptest.NewRecorder()
	
	// 使用 goroutine 处理请求
	done := make(chan bool)
	go func() {
		router.ServeHTTP(w, req)
		done <- true
	}()
	
	// 给一点时间让连接建立
	time.Sleep(100 * time.Millisecond)
	
	// 验证客户端已注册
	ch.mu.RLock()
	clientCount := len(ch.clients)
	ch.mu.RUnlock()
	
	if clientCount != 1 {
		t.Errorf("Expected 1 client, got %d", clientCount)
	}
	
	// 发送取消信号
	ch.Stop()
	
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("SSE handler did not exit")
	}
	
	// 验证 SSE headers
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Content-Type should be text/event-stream, got %s", contentType)
	}
}

// 验证消息生成新的 session ID
func TestChannel_AutoGenerateSessionID(t *testing.T) {
	ch := New()
	ch.Start(context.Background())
	defer ch.Stop()
	
	received := make(chan *channels.InboundMessage, 1)
	ch.SetMessageHandler(func(msg *channels.InboundMessage) {
		received <- msg
	})
	
	router := setupTestRouter(ch)
	
	// 不提供 sessionId
	body := `{"text":"Hello"}`
	req := httptest.NewRequest("POST", "/webchat/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	
	// 应该自动生成 sessionId
	if resp["sessionId"] == nil || resp["sessionId"] == "" {
		t.Error("Should auto-generate sessionId")
	}
}

// 辅助函数
func readSSEEvents(r io.Reader) ([]map[string]interface{}, error) {
	var events []map[string]interface{}
	// 简化实现
	return events, nil
}
