package cron

import (
	"sync"
	"testing"
	"time"
)

func TestScheduler_New(t *testing.T) {
	tmpDir := t.TempDir()
	
	s := NewScheduler(tmpDir, nil)
	if s == nil {
		t.Fatal("NewScheduler returned nil")
	}
	if s.jobs == nil {
		t.Error("Jobs map should be initialized")
	}
}

func TestScheduler_AddJob(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewScheduler(tmpDir, nil)
	
	job := &Job{
		Name: "Test Job",
		Schedule: Schedule{
			Kind:    "every",
			EveryMs: 60000, // 1 minute
		},
		Payload: Payload{
			Kind: "systemEvent",
			Text: "Test message",
		},
		SessionTarget: "main",
		Enabled:       true,
	}
	
	err := s.AddJob(job)
	if err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}
	
	if job.ID == "" {
		t.Error("Job ID should be generated")
	}
	
	// 验证任务已添加
	jobs := s.ListJobs(true)
	if len(jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs))
	}
}

func TestScheduler_GetJob(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewScheduler(tmpDir, nil)
	
	job := &Job{
		Name: "Get Test",
		Schedule: Schedule{
			Kind:    "every",
			EveryMs: 60000,
		},
		Payload: Payload{
			Kind: "systemEvent",
			Text: "Test",
		},
		SessionTarget: "main",
		Enabled:       false,
	}
	s.AddJob(job)
	
	retrieved, ok := s.GetJob(job.ID)
	if !ok {
		t.Fatal("Job not found")
	}
	if retrieved.Name != "Get Test" {
		t.Errorf("Name mismatch: %s", retrieved.Name)
	}
}

func TestScheduler_GetJobNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewScheduler(tmpDir, nil)
	
	_, ok := s.GetJob("nonexistent")
	if ok {
		t.Error("Should not find nonexistent job")
	}
}

func TestScheduler_RemoveJob(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewScheduler(tmpDir, nil)
	
	job := &Job{
		Name: "To Remove",
		Schedule: Schedule{
			Kind:    "every",
			EveryMs: 60000,
		},
		Payload: Payload{
			Kind: "systemEvent",
			Text: "Test",
		},
		SessionTarget: "main",
		Enabled:       false,
	}
	s.AddJob(job)
	
	err := s.RemoveJob(job.ID)
	if err != nil {
		t.Fatalf("RemoveJob failed: %v", err)
	}
	
	_, ok := s.GetJob(job.ID)
	if ok {
		t.Error("Job should be removed")
	}
}

func TestScheduler_RemoveJobNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewScheduler(tmpDir, nil)
	
	err := s.RemoveJob("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent job")
	}
}

func TestScheduler_UpdateJob(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewScheduler(tmpDir, nil)
	
	job := &Job{
		Name: "Original Name",
		Schedule: Schedule{
			Kind:    "every",
			EveryMs: 60000,
		},
		Payload: Payload{
			Kind: "systemEvent",
			Text: "Test",
		},
		SessionTarget: "main",
		Enabled:       true,
	}
	s.AddJob(job)
	
	// 更新名称
	err := s.UpdateJob(job.ID, map[string]interface{}{
		"name": "Updated Name",
	})
	if err != nil {
		t.Fatalf("UpdateJob failed: %v", err)
	}
	
	updated, _ := s.GetJob(job.ID)
	if updated.Name != "Updated Name" {
		t.Errorf("Name should be updated: %s", updated.Name)
	}
}

func TestScheduler_UpdateJobNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewScheduler(tmpDir, nil)
	
	err := s.UpdateJob("nonexistent", map[string]interface{}{
		"name": "New Name",
	})
	if err == nil {
		t.Error("Expected error for nonexistent job")
	}
}

func TestScheduler_ListJobs(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewScheduler(tmpDir, nil)
	
	// 添加启用的任务
	s.AddJob(&Job{
		Name:          "Enabled Job",
		Schedule:      Schedule{Kind: "every", EveryMs: 60000},
		Payload:       Payload{Kind: "systemEvent", Text: "Test"},
		SessionTarget: "main",
		Enabled:       true,
	})
	
	// 添加禁用的任务
	s.AddJob(&Job{
		Name:          "Disabled Job",
		Schedule:      Schedule{Kind: "every", EveryMs: 60000},
		Payload:       Payload{Kind: "systemEvent", Text: "Test"},
		SessionTarget: "main",
		Enabled:       false,
	})
	
	// 只列出启用的
	enabledOnly := s.ListJobs(false)
	if len(enabledOnly) != 1 {
		t.Errorf("Expected 1 enabled job, got %d", len(enabledOnly))
	}
	
	// 列出所有
	all := s.ListJobs(true)
	if len(all) != 2 {
		t.Errorf("Expected 2 total jobs, got %d", len(all))
	}
}

func TestScheduler_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewScheduler(tmpDir, nil)
	
	s.Start()
	// 不应该 panic
	
	s.Stop()
	// 不应该 panic
}

func TestScheduler_JobExecution(t *testing.T) {
	tmpDir := t.TempDir()
	
	var executed bool
	var mu sync.Mutex
	
	handler := func(job *Job) error {
		mu.Lock()
		executed = true
		mu.Unlock()
		return nil
	}
	
	s := NewScheduler(tmpDir, handler)
	s.Start()
	defer s.Stop()
	
	// 添加一个立即执行的任务
	job := &Job{
		Name: "Quick Job",
		Schedule: Schedule{
			Kind: "cron",
			Expr: "* * * * * *", // 每秒执行
		},
		Payload: Payload{
			Kind: "systemEvent",
			Text: "Quick test",
		},
		SessionTarget: "main",
		Enabled:       true,
	}
	
	err := s.AddJob(job)
	if err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}
	
	// 等待执行
	time.Sleep(2 * time.Second)
	
	mu.Lock()
	wasExecuted := executed
	mu.Unlock()
	
	if !wasExecuted {
		t.Error("Job should have been executed")
	}
}

func TestScheduler_RunJobManually(t *testing.T) {
	tmpDir := t.TempDir()
	
	var executed bool
	var mu sync.Mutex
	
	handler := func(job *Job) error {
		mu.Lock()
		executed = true
		mu.Unlock()
		return nil
	}
	
	s := NewScheduler(tmpDir, handler)
	
	job := &Job{
		Name: "Manual Job",
		Schedule: Schedule{
			Kind:    "every",
			EveryMs: 3600000, // 1 hour
		},
		Payload: Payload{
			Kind: "systemEvent",
			Text: "Manual test",
		},
		SessionTarget: "main",
		Enabled:       false, // 禁用自动调度
	}
	s.AddJob(job)
	
	// 手动运行
	err := s.RunJob(job.ID)
	if err != nil {
		t.Fatalf("RunJob failed: %v", err)
	}
	
	mu.Lock()
	wasExecuted := executed
	mu.Unlock()
	
	if !wasExecuted {
		t.Error("Job should have been executed manually")
	}
}

func TestScheduler_RunJobNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewScheduler(tmpDir, nil)
	
	err := s.RunJob("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent job")
	}
}

func TestScheduler_Status(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewScheduler(tmpDir, nil)
	
	s.AddJob(&Job{
		Name:          "Job 1",
		Schedule:      Schedule{Kind: "every", EveryMs: 60000},
		Payload:       Payload{Kind: "systemEvent", Text: "Test"},
		SessionTarget: "main",
		Enabled:       true,
	})
	s.AddJob(&Job{
		Name:          "Job 2",
		Schedule:      Schedule{Kind: "every", EveryMs: 60000},
		Payload:       Payload{Kind: "systemEvent", Text: "Test"},
		SessionTarget: "main",
		Enabled:       false,
	})
	
	status := s.Status()
	totalJobs, _ := status["totalJobs"].(int)
	enabledJobs, _ := status["enabledJobs"].(int)
	
	if totalJobs != 2 {
		t.Errorf("TotalJobs should be 2, got %d", totalJobs)
	}
	if enabledJobs != 1 {
		t.Errorf("EnabledJobs should be 1, got %d", enabledJobs)
	}
}
