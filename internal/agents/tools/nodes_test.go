package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// MockNodesManager 用于测试的模拟节点管理器
type MockNodesManager struct {
	nodes []NodeInfo
}

func (m *MockNodesManager) GetStatus() ([]NodeInfo, error) {
	return m.nodes, nil
}

func (m *MockNodesManager) DescribeNode(node string) (*NodeDescription, error) {
	for _, n := range m.nodes {
		if n.ID == node || n.Name == node {
			return &NodeDescription{
				NodeInfo:    n,
				DeviceModel: "Test Device",
				OSVersion:   "1.0",
				Battery:     85,
			}, nil
		}
	}
	return nil, nil
}

func (m *MockNodesManager) GetPendingPairings() ([]PairingRequest, error) {
	return []PairingRequest{
		{RequestID: "req-1", DeviceName: "iPhone", Platform: "ios"},
	}, nil
}

func (m *MockNodesManager) ApprovePairing(requestID string) error {
	return nil
}

func (m *MockNodesManager) RejectPairing(requestID string) error {
	return nil
}

func (m *MockNodesManager) SendNotification(node string, opts NotificationOptions) error {
	return nil
}

func (m *MockNodesManager) CameraSnap(node string, opts CameraOptions) ([]byte, error) {
	return []byte("fake-image"), nil
}

func (m *MockNodesManager) CameraList(node string) ([]CameraInfo, error) {
	return []CameraInfo{
		{DeviceID: "cam-1", Label: "Front Camera", Facing: "front"},
		{DeviceID: "cam-2", Label: "Back Camera", Facing: "back"},
	}, nil
}

func (m *MockNodesManager) CameraClip(node string, opts CameraClipOptions) (string, error) {
	return "/tmp/clip.mp4", nil
}

func (m *MockNodesManager) ScreenRecord(node string, opts ScreenRecordOptions) (string, error) {
	return "/tmp/screen.mp4", nil
}

func (m *MockNodesManager) GetLocation(node string, opts LocationOptions) (*LocationInfo, error) {
	return &LocationInfo{
		Latitude:  37.7749,
		Longitude: -122.4194,
		Accuracy:  10,
	}, nil
}

func (m *MockNodesManager) RunCommand(node string, command []string, opts RunOptions) (string, error) {
	return "command output", nil
}

func TestNodesTool_Name(t *testing.T) {
	tool := NewNodesTool()
	if tool.Name() != ToolNodes {
		t.Errorf("Name should be %s, got %s", ToolNodes, tool.Name())
	}
}

func TestNodesTool_Description(t *testing.T) {
	tool := NewNodesTool()
	desc := tool.Description()
	
	if !strings.Contains(desc, "node") {
		t.Error("Description should mention node")
	}
}

func TestNodesTool_Parameters(t *testing.T) {
	tool := NewNodesTool()
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
}

func TestNodesTool_NoManager(t *testing.T) {
	tool := NewNodesTool()
	
	params := NodesParams{
		Action: "status",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error when manager is not set")
	}
}

func TestNodesTool_Status(t *testing.T) {
	tool := NewNodesTool()
	tool.SetManager(&MockNodesManager{
		nodes: []NodeInfo{
			{ID: "node-1", Name: "iPhone", Platform: "ios", Online: true},
			{ID: "node-2", Name: "Mac", Platform: "macos", Online: false},
		},
	})
	
	params := NodesParams{
		Action: "status",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "iPhone") {
		t.Errorf("Should contain node name: %s", result.Content)
	}
}

func TestNodesTool_Describe(t *testing.T) {
	tool := NewNodesTool()
	tool.SetManager(&MockNodesManager{
		nodes: []NodeInfo{
			{ID: "node-1", Name: "iPhone", Platform: "ios", Online: true, LastSeen: time.Now()},
		},
	})
	
	params := NodesParams{
		Action: "describe",
		Node:   "node-1",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Test Device") {
		t.Errorf("Should contain device model: %s", result.Content)
	}
}

func TestNodesTool_Pending(t *testing.T) {
	tool := NewNodesTool()
	tool.SetManager(&MockNodesManager{})
	
	params := NodesParams{
		Action: "pending",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "req-1") {
		t.Errorf("Should contain request ID: %s", result.Content)
	}
}

func TestNodesTool_Notify(t *testing.T) {
	tool := NewNodesTool()
	tool.SetManager(&MockNodesManager{
		nodes: []NodeInfo{{ID: "node-1", Name: "iPhone", Online: true}},
	})
	
	params := NodesParams{
		Action: "notify",
		Node:   "node-1",
		Title:  "Test",
		Body:   "Hello!",
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

func TestNodesTool_CameraList(t *testing.T) {
	tool := NewNodesTool()
	tool.SetManager(&MockNodesManager{
		nodes: []NodeInfo{{ID: "node-1", Name: "iPhone", Online: true}},
	})
	
	params := NodesParams{
		Action: "camera_list",
		Node:   "node-1",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Front Camera") {
		t.Errorf("Should list cameras: %s", result.Content)
	}
}

func TestNodesTool_Location(t *testing.T) {
	tool := NewNodesTool()
	tool.SetManager(&MockNodesManager{
		nodes: []NodeInfo{{ID: "node-1", Name: "iPhone", Online: true}},
	})
	
	params := NodesParams{
		Action: "location_get",
		Node:   "node-1",
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "37.7749") {
		t.Errorf("Should contain latitude: %s", result.Content)
	}
}

func TestNodesTool_Run(t *testing.T) {
	tool := NewNodesTool()
	tool.SetManager(&MockNodesManager{
		nodes: []NodeInfo{{ID: "node-1", Name: "Mac", Platform: "macos", Online: true}},
	})
	
	params := NodesParams{
		Action:  "run",
		Node:    "node-1",
		Command: []string{"echo", "hello"},
	}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "command output") {
		t.Errorf("Should contain command output: %s", result.Content)
	}
}

func TestNodesTool_UnknownAction(t *testing.T) {
	tool := NewNodesTool()
	tool.SetManager(&MockNodesManager{})
	
	params := NodesParams{
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

func TestNodesTool_InvalidParams(t *testing.T) {
	tool := NewNodesTool()
	
	result, err := tool.Execute(context.Background(), []byte("invalid"))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for invalid params")
	}
}
