package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	cron      *cron.Cron
	jobs      map[string]*Job
	jobsMu    sync.RWMutex
	stateFile string
	handler   JobHandler
}

// Job 定时任务
type Job struct {
	ID            string      `json:"id"`
	Name          string      `json:"name,omitempty"`
	Schedule      Schedule    `json:"schedule"`
	Payload       Payload     `json:"payload"`
	SessionTarget string      `json:"sessionTarget"` // "main" | "isolated"
	Enabled       bool        `json:"enabled"`
	CreatedAt     time.Time   `json:"createdAt"`
	NextRunAt     *time.Time  `json:"nextRunAt,omitempty"`
	LastRunAt     *time.Time  `json:"lastRunAt,omitempty"`
	LastResult    string      `json:"lastResult,omitempty"`
	
	entryID cron.EntryID `json:"-"`
}

// Schedule 调度配置
type Schedule struct {
	Kind     string `json:"kind"` // "at" | "every" | "cron"
	AtMs     int64  `json:"atMs,omitempty"`     // for "at": unix timestamp ms
	EveryMs  int64  `json:"everyMs,omitempty"`  // for "every": interval ms
	Expr     string `json:"expr,omitempty"`     // for "cron": cron expression
	Tz       string `json:"tz,omitempty"`       // timezone
	AnchorMs int64  `json:"anchorMs,omitempty"` // for "every": anchor time
}

// Payload 任务内容
type Payload struct {
	Kind    string `json:"kind"` // "systemEvent" | "agentTurn"
	Text    string `json:"text,omitempty"`
	Message string `json:"message,omitempty"`
	Model   string `json:"model,omitempty"`
}

// JobHandler 任务执行处理器
type JobHandler func(job *Job) error

// NewScheduler 创建调度器
func NewScheduler(stateDir string, handler JobHandler) *Scheduler {
	s := &Scheduler{
		cron:      cron.New(cron.WithSeconds()),
		jobs:      make(map[string]*Job),
		stateFile: filepath.Join(stateDir, "cron-jobs.json"),
		handler:   handler,
	}
	
	// 加载持久化的任务
	s.loadJobs()
	
	return s
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.cron.Start()
	log.Info().Msg("Cron scheduler started")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Info().Msg("Cron scheduler stopped")
}

// AddJob 添加任务
func (s *Scheduler) AddJob(job *Job) error {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	job.CreatedAt = time.Now()

	// 添加到 cron
	if job.Enabled {
		if err := s.scheduleJob(job); err != nil {
			return err
		}
	}

	s.jobs[job.ID] = job
	s.saveJobs()
	
	return nil
}

// UpdateJob 更新任务
func (s *Scheduler) UpdateJob(id string, patch map[string]interface{}) error {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	// 先移除旧的调度
	if job.entryID != 0 {
		s.cron.Remove(job.entryID)
	}

	// 应用更新
	if name, ok := patch["name"].(string); ok {
		job.Name = name
	}
	if enabled, ok := patch["enabled"].(bool); ok {
		job.Enabled = enabled
	}
	// TODO: 更多字段...

	// 重新调度
	if job.Enabled {
		if err := s.scheduleJob(job); err != nil {
			return err
		}
	}

	s.saveJobs()
	return nil
}

// RemoveJob 移除任务
func (s *Scheduler) RemoveJob(id string) error {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	if job.entryID != 0 {
		s.cron.Remove(job.entryID)
	}

	delete(s.jobs, id)
	s.saveJobs()
	
	return nil
}

// RunJob 立即执行任务
func (s *Scheduler) RunJob(id string) error {
	s.jobsMu.RLock()
	job, ok := s.jobs[id]
	s.jobsMu.RUnlock()

	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	return s.executeJob(job)
}

// ListJobs 列出所有任务
func (s *Scheduler) ListJobs(includeDisabled bool) []*Job {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	var jobs []*Job
	for _, job := range s.jobs {
		if includeDisabled || job.Enabled {
			// 更新 NextRunAt
			if job.entryID != 0 {
				entry := s.cron.Entry(job.entryID)
				if !entry.Next.IsZero() {
					next := entry.Next
					job.NextRunAt = &next
				}
			}
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// GetJob 获取任务
func (s *Scheduler) GetJob(id string) (*Job, bool) {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}

// scheduleJob 调度任务
func (s *Scheduler) scheduleJob(job *Job) error {
	var spec string
	
	switch job.Schedule.Kind {
	case "cron":
		spec = job.Schedule.Expr
	case "every":
		// 转换为 cron 表达式
		intervalSec := job.Schedule.EveryMs / 1000
		if intervalSec < 60 {
			spec = fmt.Sprintf("@every %ds", intervalSec)
		} else if intervalSec < 3600 {
			spec = fmt.Sprintf("@every %dm", intervalSec/60)
		} else {
			spec = fmt.Sprintf("@every %dh", intervalSec/3600)
		}
	case "at":
		// 一次性任务
		targetTime := time.UnixMilli(job.Schedule.AtMs)
		if targetTime.Before(time.Now()) {
			return fmt.Errorf("scheduled time is in the past")
		}
		// 使用 cron 的 once 语义
		spec = targetTime.Format("05 04 15 02 01 *") // second minute hour day month dow
	default:
		return fmt.Errorf("unknown schedule kind: %s", job.Schedule.Kind)
	}

	entryID, err := s.cron.AddFunc(spec, func() {
		s.executeJob(job)
	})
	if err != nil {
		return err
	}

	job.entryID = entryID
	return nil
}

// executeJob 执行任务
func (s *Scheduler) executeJob(job *Job) error {
	log.Info().Str("jobId", job.ID).Str("name", job.Name).Msg("Executing cron job")
	
	now := time.Now()
	job.LastRunAt = &now

	if s.handler != nil {
		if err := s.handler(job); err != nil {
			job.LastResult = "error: " + err.Error()
			return err
		}
		job.LastResult = "success"
	}

	s.saveJobs()
	return nil
}

// loadJobs 加载持久化的任务
func (s *Scheduler) loadJobs() {
	data, err := os.ReadFile(s.stateFile)
	if err != nil {
		return
	}

	var jobs []*Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		log.Warn().Err(err).Msg("Failed to load cron jobs")
		return
	}

	for _, job := range jobs {
		if job.Enabled {
			s.scheduleJob(job)
		}
		s.jobs[job.ID] = job
	}

	log.Info().Int("count", len(jobs)).Msg("Loaded cron jobs")
}

// saveJobs 保存任务
func (s *Scheduler) saveJobs() {
	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal cron jobs")
		return
	}

	os.MkdirAll(filepath.Dir(s.stateFile), 0755)
	if err := os.WriteFile(s.stateFile, data, 0644); err != nil {
		log.Error().Err(err).Msg("Failed to save cron jobs")
	}
}

// Status 获取调度器状态
func (s *Scheduler) Status() map[string]interface{} {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()
	
	enabledCount := 0
	for _, job := range s.jobs {
		if job.Enabled {
			enabledCount++
		}
	}

	return map[string]interface{}{
		"enabled":      true,
		"totalJobs":    len(s.jobs),
		"enabledJobs":  enabledCount,
		"runningTasks": len(s.cron.Entries()),
	}
}
