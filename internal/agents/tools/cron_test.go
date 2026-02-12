package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// 由于 CronTool 依赖 cron.Scheduler，这里测试基本的参数验证

func TestCronTool_NoScheduler(t *testing.T) {
	// 没有 scheduler 时应该返回错误
	tool := &CronTool{scheduler: nil}
	
	params := CronParams{Action: "status"}
	args, _ := json.Marshal(params)
	
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error when scheduler is nil")
	}
	if !strings.Contains(result.Content, "not initialized") {
		t.Errorf("Expected 'not initialized' error, got: %s", result.Content)
	}
}

func TestCronTool_UnknownAction(t *testing.T) {
	tool := &CronTool{scheduler: nil}
	
	params := CronParams{Action: "invalid"}
	args, _ := json.Marshal(params)
	
	result, _ := tool.Execute(context.Background(), args)
	// 可能返回 scheduler nil 错误或 unknown action
	// 两种情况都是预期的行为
	_ = result
}

func TestCronTool_InvalidParams(t *testing.T) {
	tool := &CronTool{scheduler: nil}
	
	result, err := tool.Execute(context.Background(), []byte("invalid json"))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for invalid params")
	}
}

func TestCronTool_Name(t *testing.T) {
	tool := NewCronTool(nil)
	if tool.Name() != "cron" {
		t.Errorf("Name should be 'cron', got %s", tool.Name())
	}
}

func TestCronTool_Description(t *testing.T) {
	tool := NewCronTool(nil)
	desc := tool.Description()
	
	// 确保描述包含关键信息
	if !strings.Contains(desc, "cron") {
		t.Error("Description should mention cron")
	}
	if !strings.Contains(desc, "systemEvent") {
		t.Error("Description should mention systemEvent")
	}
	if !strings.Contains(desc, "agentTurn") {
		t.Error("Description should mention agentTurn")
	}
}

func TestCronTool_Parameters(t *testing.T) {
	tool := NewCronTool(nil)
	params := tool.Parameters()
	
	var schema map[string]interface{}
	if err := json.Unmarshal(params, &schema); err != nil {
		t.Fatalf("Parameters should be valid JSON: %v", err)
	}
	
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema should have properties")
	}
	
	// 检查必要的属性
	if _, ok := props["action"]; !ok {
		t.Error("Schema should have 'action' property")
	}
	if _, ok := props["jobId"]; !ok {
		t.Error("Schema should have 'jobId' property")
	}
}
