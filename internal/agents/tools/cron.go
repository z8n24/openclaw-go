package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/user/openclaw-go/internal/cron"
)

// CronTool 定时任务工具
type CronTool struct {
	scheduler *cron.Scheduler
}

type CronParams struct {
	Action          string          `json:"action"` // status, list, add, update, remove, run, runs
	JobID           string          `json:"jobId,omitempty"`
	IncludeDisabled bool            `json:"includeDisabled,omitempty"`
	Job             *cron.Job       `json:"job,omitempty"`
	Patch           json.RawMessage `json:"patch,omitempty"`
}

func NewCronTool(scheduler *cron.Scheduler) *CronTool {
	return &CronTool{scheduler: scheduler}
}

func (t *CronTool) Name() string {
	return "cron"
}

func (t *CronTool) Description() string {
	return `Manage cron jobs: status/list/add/update/remove/run.

SCHEDULE TYPES:
- "at": One-shot at absolute time { "kind": "at", "atMs": <unix-ms-timestamp> }
- "every": Recurring interval { "kind": "every", "everyMs": <interval-ms> }
- "cron": Cron expression { "kind": "cron", "expr": "<cron-expression>" }

PAYLOAD TYPES:
- "systemEvent": { "kind": "systemEvent", "text": "<message>" }
- "agentTurn": { "kind": "agentTurn", "message": "<prompt>" }

sessionTarget must be "main" for systemEvent, "isolated" for agentTurn.`
}

func (t *CronTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["status", "list", "add", "update", "remove", "run"],
				"description": "Cron action"
			},
			"jobId": {"type": "string", "description": "Job ID for update/remove/run"},
			"includeDisabled": {"type": "boolean", "description": "Include disabled jobs in list"},
			"job": {
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"schedule": {"type": "object"},
					"payload": {"type": "object"},
					"sessionTarget": {"type": "string"},
					"enabled": {"type": "boolean"}
				}
			},
			"patch": {"type": "object", "description": "Fields to update"}
		},
		"required": ["action"]
	}`)
}

func (t *CronTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params CronParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if t.scheduler == nil {
		return &Result{Content: "Cron scheduler not initialized", IsError: true}, nil
	}

	switch params.Action {
	case "status":
		return t.status()
	case "list":
		return t.list(params.IncludeDisabled)
	case "add":
		return t.add(params.Job)
	case "update":
		return t.update(params.JobID, params.Patch)
	case "remove":
		return t.remove(params.JobID)
	case "run":
		return t.run(params.JobID)
	default:
		return &Result{Content: "Unknown action: " + params.Action, IsError: true}, nil
	}
}

func (t *CronTool) status() (*Result, error) {
	status := t.scheduler.Status()
	data, _ := json.MarshalIndent(status, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *CronTool) list(includeDisabled bool) (*Result, error) {
	jobs := t.scheduler.ListJobs(includeDisabled)
	
	if len(jobs) == 0 {
		return &Result{Content: "No cron jobs configured"}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d jobs:\n\n", len(jobs)))
	
	for _, job := range jobs {
		status := "enabled"
		if !job.Enabled {
			status = "disabled"
		}
		
		sb.WriteString(fmt.Sprintf("**%s** (%s)\n", job.Name, job.ID))
		sb.WriteString(fmt.Sprintf("  Status: %s\n", status))
		sb.WriteString(fmt.Sprintf("  Schedule: %s\n", formatSchedule(job.Schedule)))
		if job.NextRunAt != nil {
			sb.WriteString(fmt.Sprintf("  Next run: %s\n", job.NextRunAt.Format("2006-01-02 15:04:05")))
		}
		if job.LastRunAt != nil {
			sb.WriteString(fmt.Sprintf("  Last run: %s (%s)\n", job.LastRunAt.Format("2006-01-02 15:04:05"), job.LastResult))
		}
		sb.WriteString("\n")
	}

	return &Result{Content: sb.String()}, nil
}

func (t *CronTool) add(job *cron.Job) (*Result, error) {
	if job == nil {
		return &Result{Content: "Job is required", IsError: true}, nil
	}

	if job.Schedule.Kind == "" {
		return &Result{Content: "Schedule kind is required", IsError: true}, nil
	}

	if job.Payload.Kind == "" {
		return &Result{Content: "Payload kind is required", IsError: true}, nil
	}

	if job.SessionTarget == "" {
		if job.Payload.Kind == "systemEvent" {
			job.SessionTarget = "main"
		} else {
			job.SessionTarget = "isolated"
		}
	}

	if job.Enabled == false {
		job.Enabled = true // 默认启用
	}

	if err := t.scheduler.AddJob(job); err != nil {
		return &Result{Content: "Failed to add job: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Job added: %s (ID: %s)", job.Name, job.ID)}, nil
}

func (t *CronTool) update(jobID string, patch json.RawMessage) (*Result, error) {
	if jobID == "" {
		return &Result{Content: "Job ID is required", IsError: true}, nil
	}

	var patchMap map[string]interface{}
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return &Result{Content: "Invalid patch: " + err.Error(), IsError: true}, nil
	}

	if err := t.scheduler.UpdateJob(jobID, patchMap); err != nil {
		return &Result{Content: "Failed to update job: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: "Job updated: " + jobID}, nil
}

func (t *CronTool) remove(jobID string) (*Result, error) {
	if jobID == "" {
		return &Result{Content: "Job ID is required", IsError: true}, nil
	}

	if err := t.scheduler.RemoveJob(jobID); err != nil {
		return &Result{Content: "Failed to remove job: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: "Job removed: " + jobID}, nil
}

func (t *CronTool) run(jobID string) (*Result, error) {
	if jobID == "" {
		return &Result{Content: "Job ID is required", IsError: true}, nil
	}

	if err := t.scheduler.RunJob(jobID); err != nil {
		return &Result{Content: "Failed to run job: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: "Job triggered: " + jobID}, nil
}

func formatSchedule(s cron.Schedule) string {
	switch s.Kind {
	case "cron":
		return fmt.Sprintf("cron(%s)", s.Expr)
	case "every":
		sec := s.EveryMs / 1000
		if sec < 60 {
			return fmt.Sprintf("every %ds", sec)
		} else if sec < 3600 {
			return fmt.Sprintf("every %dm", sec/60)
		} else if sec < 86400 {
			return fmt.Sprintf("every %dh", sec/3600)
		}
		return fmt.Sprintf("every %dd", sec/86400)
	case "at":
		return fmt.Sprintf("at %d", s.AtMs)
	default:
		return "unknown"
	}
}
