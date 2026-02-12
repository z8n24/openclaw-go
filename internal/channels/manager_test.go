package channels

import (
	"context"
	"sync"
	"testing"
	"time"
)

// MockChannel 用于测试的模拟渠道
type MockChannel struct {
	id        string
	label     string
	handler   MessageHandler
	started   bool
	messages  []*OutboundMessage
	mu        sync.Mutex
}

func NewMockChannel(id, label string) *MockChannel {
	return &MockChannel{
		id:    id,
		label: label,
	}
}

func (c *MockChannel) ID() string {
	return c.id
}

func (c *MockChannel) Label() string {
	return c.label
}

func (c *MockChannel) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started = true
	return nil
}

func (c *MockChannel) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started = false
	return nil
}

func (c *MockChannel) Status() ChannelStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return ChannelStatus{Connected: c.started}
}

func (c *MockChannel) Send(ctx context.Context, msg *OutboundMessage) (*SendResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msg)
	return &SendResult{
		MessageID: "msg-123",
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (c *MockChannel) SetMessageHandler(handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

func (c *MockChannel) SimulateMessage(msg *InboundMessage) {
	c.mu.Lock()
	handler := c.handler
	c.mu.Unlock()
	if handler != nil {
		handler(msg)
	}
}

func (c *MockChannel) GetSentMessages() []*OutboundMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.messages
}

// Tests

func TestManager_Register(t *testing.T) {
	m := NewManager()
	ch := NewMockChannel("test", "Test Channel")
	
	m.Register(ch)
	
	got, ok := m.Get("test")
	if !ok {
		t.Fatal("Channel not found after registration")
	}
	if got.ID() != "test" {
		t.Errorf("Channel ID mismatch: got %s", got.ID())
	}
}

func TestManager_List(t *testing.T) {
	m := NewManager()
	m.Register(NewMockChannel("ch1", "Channel 1"))
	m.Register(NewMockChannel("ch2", "Channel 2"))
	
	channels := m.List()
	if len(channels) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(channels))
	}
}

func TestManager_StartAll(t *testing.T) {
	m := NewManager()
	ch1 := NewMockChannel("ch1", "Channel 1")
	ch2 := NewMockChannel("ch2", "Channel 2")
	m.Register(ch1)
	m.Register(ch2)
	
	m.StartAll()
	
	if !ch1.Status().Connected {
		t.Error("Channel 1 should be connected")
	}
	if !ch2.Status().Connected {
		t.Error("Channel 2 should be connected")
	}
}

func TestManager_StopAll(t *testing.T) {
	m := NewManager()
	ch := NewMockChannel("test", "Test")
	m.Register(ch)
	m.StartAll()
	
	m.StopAll()
	
	if ch.Status().Connected {
		t.Error("Channel should be disconnected after StopAll")
	}
}

func TestManager_Send(t *testing.T) {
	m := NewManager()
	ch := NewMockChannel("test", "Test")
	m.Register(ch)
	
	msg := &OutboundMessage{
		ChatID: "chat-123",
		Text:   "Hello!",
	}
	
	result, err := m.Send(context.Background(), "test", msg)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if result.MessageID == "" {
		t.Error("Expected message ID")
	}
	
	sent := ch.GetSentMessages()
	if len(sent) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(sent))
	}
	if sent[0].Text != "Hello!" {
		t.Errorf("Message text mismatch: %s", sent[0].Text)
	}
}

func TestManager_SendChannelNotFound(t *testing.T) {
	m := NewManager()
	
	_, err := m.Send(context.Background(), "nonexistent", &OutboundMessage{})
	if err == nil {
		t.Error("Expected error for nonexistent channel")
	}
}

func TestManager_MessageHandler(t *testing.T) {
	m := NewManager()
	ch := NewMockChannel("test", "Test")
	m.Register(ch)
	
	received := make(chan *InboundMessage, 1)
	m.SetMessageHandler(func(msg *InboundMessage) error {
		received <- msg
		return nil
	})
	
	// 模拟收到消息
	testMsg := &InboundMessage{
		ID:       "msg-1",
		Channel:  "test",
		ChatID:   "chat-123",
		SenderID: "user-1",
		Text:     "Test message",
	}
	ch.SimulateMessage(testMsg)
	
	select {
	case msg := <-received:
		if msg.Text != "Test message" {
			t.Errorf("Message text mismatch: %s", msg.Text)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestManager_Reply(t *testing.T) {
	m := NewManager()
	ch := NewMockChannel("test", "Test")
	m.Register(ch)
	
	original := &InboundMessage{
		ID:      "msg-1",
		Channel: "test",
		ChatID:  "chat-123",
	}
	
	_, err := m.Reply(context.Background(), original, "Reply text")
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}
	
	sent := ch.GetSentMessages()
	if len(sent) != 1 {
		t.Fatalf("Expected 1 sent message")
	}
	if sent[0].ReplyTo != "msg-1" {
		t.Errorf("ReplyTo should be set: %s", sent[0].ReplyTo)
	}
}

func TestManager_Status(t *testing.T) {
	m := NewManager()
	ch := NewMockChannel("test", "Test")
	m.Register(ch)
	m.StartAll()
	
	status := m.Status()
	
	if !status["test"].Connected {
		t.Error("Status should show connected")
	}
}

func TestStreamingResponder_Write(t *testing.T) {
	m := NewManager()
	
	var lastContent string
	var lastDone bool
	m.SetResponseHandler(func(chatID, messageID, content string, done bool) {
		lastContent = content
		lastDone = done
	})
	
	r := m.NewStreamingResponder("test", "chat-1", "msg-1")
	
	r.Write([]byte("Hello"))
	if lastContent != "Hello" {
		t.Errorf("Content mismatch: %s", lastContent)
	}
	if lastDone {
		t.Error("Should not be done yet")
	}
	
	r.Write([]byte(" World"))
	if lastContent != "Hello World" {
		t.Errorf("Content should accumulate: %s", lastContent)
	}
	
	r.Done()
	if !lastDone {
		t.Error("Should be done")
	}
}

func TestStreamingResponder_Content(t *testing.T) {
	m := NewManager()
	r := m.NewStreamingResponder("test", "chat-1", "msg-1")
	
	r.Write([]byte("Test"))
	r.Write([]byte(" Content"))
	
	if r.Content() != "Test Content" {
		t.Errorf("Content mismatch: %s", r.Content())
	}
}

func TestMessageRouter(t *testing.T) {
	m := NewManager()
	ch := NewMockChannel("test", "Test")
	m.Register(ch)
	
	router := NewMessageRouter(m)
	
	// 设置会话解析器
	router.SetSessionResolver(func(channelID, chatID string) (string, bool) {
		return "session-" + chatID, true
	})
	
	// 设置 Agent 运行器
	agentCalled := make(chan bool, 1)
	router.SetAgentRunner(func(ctx context.Context, sessionKey string, msg *InboundMessage) (string, error) {
		agentCalled <- true
		return "Agent response", nil
	})
	
	// 模拟收到消息
	ch.SimulateMessage(&InboundMessage{
		ID:      "msg-1",
		Channel: "test",
		ChatID:  "chat-123",
		Text:    "Hello agent",
	})
	
	// 等待 agent 被调用
	select {
	case <-agentCalled:
		// OK
	case <-time.After(2 * time.Second):
		t.Error("Agent was not called")
	}
	
	// 等待响应发送
	time.Sleep(100 * time.Millisecond)
	
	sent := ch.GetSentMessages()
	if len(sent) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(sent))
	}
	if sent[0].Text != "Agent response" {
		t.Errorf("Response mismatch: %s", sent[0].Text)
	}
}
