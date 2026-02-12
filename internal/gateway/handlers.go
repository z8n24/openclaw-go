package gateway

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/z8n24/openclaw-go/internal/cron"
	"github.com/z8n24/openclaw-go/internal/gateway/protocol"
	"github.com/z8n24/openclaw-go/internal/skills"
)

// Dependencies 用于注入依赖
type Dependencies struct {
	CronScheduler *cron.Scheduler
	SkillLoader   *skills.Loader
	// SessionManager 等其他依赖可以后续添加
}

// SetDependencies 设置依赖
func (s *Server) SetDependencies(deps Dependencies) {
	s.deps = deps
}

// registerDefaultHandlers 注册默认的 RPC 处理器
func (s *Server) registerDefaultHandlers() {
	// Config 相关
	s.RegisterHandler("config.get", s.handleConfigGet)
	s.RegisterHandler("config.set", s.handleConfigSet)
	s.RegisterHandler("config.schema", s.handleConfigSchema)
	s.RegisterHandler("config.apply", s.handleConfigApply)

	// Sessions 相关
	s.RegisterHandler("sessions.list", s.handleSessionsList)
	s.RegisterHandler("sessions.preview", s.handleSessionsPreview)
	s.RegisterHandler("sessions.reset", s.handleSessionsReset)
	s.RegisterHandler("sessions.delete", s.handleSessionsDelete)
	s.RegisterHandler("sessions.compact", s.handleSessionsCompact)

	// Channels 相关
	s.RegisterHandler("channels.status", s.handleChannelsStatus)
	s.RegisterHandler("channels.logout", s.handleChannelsLogout)

	// Agents 相关
	s.RegisterHandler("agents.list", s.handleAgentsList)
	s.RegisterHandler("agent.identity", s.handleAgentIdentity)
	s.RegisterHandler("wake", s.handleWake)

	// Models 相关
	s.RegisterHandler("models.list", s.handleModelsList)

	// Cron 相关
	s.RegisterHandler("cron.list", s.handleCronList)
	s.RegisterHandler("cron.status", s.handleCronStatus)
	s.RegisterHandler("cron.add", s.handleCronAdd)
	s.RegisterHandler("cron.update", s.handleCronUpdate)
	s.RegisterHandler("cron.remove", s.handleCronRemove)
	s.RegisterHandler("cron.run", s.handleCronRun)

	// Chat 相关
	s.RegisterHandler("chat.send", s.handleChatSend)
	s.RegisterHandler("chat.history", s.handleChatHistory)
	s.RegisterHandler("chat.abort", s.handleChatAbort)
	s.RegisterHandler("chat.inject", s.handleChatInject)

	// Skills 相关
	s.RegisterHandler("skills.status", s.handleSkillsStatus)
	s.RegisterHandler("skills.bins", s.handleSkillsBins)
	s.RegisterHandler("skills.install", s.handleSkillsInstall)
	s.RegisterHandler("skills.update", s.handleSkillsUpdate)

	// Logs 相关
	s.RegisterHandler("logs.tail", s.handleLogsTail)

	// Exec approvals
	s.RegisterHandler("exec.approvals.get", s.handleExecApprovalsGet)
	s.RegisterHandler("exec.approvals.set", s.handleExecApprovalsSet)

	// Node 相关
	s.RegisterHandler("node.list", s.handleNodeList)
	s.RegisterHandler("node.describe", s.handleNodeDescribe)
}

// ============================================================================
// Config handlers
// ============================================================================

func (s *Server) handleConfigGet(ctx *MethodContext) error {
	ctx.Respond(true, s.cfg)
	return nil
}

type ConfigSetParams struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

func (s *Server) handleConfigSet(ctx *MethodContext) error {
	var params ConfigSetParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	// TODO: 实际应用配置更改
	ctx.Respond(true, map[string]interface{}{"updated": true})
	return nil
}

func (s *Server) handleConfigSchema(ctx *MethodContext) error {
	schema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]interface{}{
			"gateway": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"port":  map[string]interface{}{"type": "integer", "default": 18789},
					"bind":  map[string]interface{}{"type": "string", "default": "127.0.0.1"},
					"token": map[string]interface{}{"type": "string"},
				},
			},
			"agent": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"defaultModel": map[string]interface{}{"type": "string"},
					"workspace":    map[string]interface{}{"type": "string"},
					"thinking":     map[string]interface{}{"type": "string", "enum": []string{"off", "low", "medium", "high"}},
				},
			},
		},
	}
	ctx.Respond(true, map[string]interface{}{"schema": schema})
	return nil
}

func (s *Server) handleConfigApply(ctx *MethodContext) error {
	// 重新加载配置
	ctx.Respond(true, map[string]interface{}{"applied": true})
	s.BroadcastEvent("stateChange", map[string]interface{}{"kind": "config"})
	return nil
}

// ============================================================================
// Sessions handlers
// ============================================================================

type SessionsListParams struct {
	ActiveMinutes *int `json:"activeMinutes,omitempty"`
	MessageLimit  *int `json:"messageLimit,omitempty"`
}

type SessionEntry struct {
	Key           string `json:"key"`
	Kind          string `json:"kind"`
	Label         string `json:"label,omitempty"`
	Channel       string `json:"channel,omitempty"`
	Model         string `json:"model,omitempty"`
	LastMessageAt int64  `json:"lastMessageAt,omitempty"`
	MessageCount  int    `json:"messageCount"`
	TokenCount    int64  `json:"tokenCount,omitempty"`
}

func (s *Server) handleSessionsList(ctx *MethodContext) error {
	// TODO: 从实际 session manager 获取
	sessions := []SessionEntry{
		{
			Key:          "main",
			Kind:         "main",
			Label:        "Main Session",
			MessageCount: 0,
		},
	}
	ctx.Respond(true, map[string]interface{}{"sessions": sessions})
	return nil
}

type SessionsPreviewParams struct {
	Key   string `json:"key"`
	Limit *int   `json:"limit,omitempty"`
}

func (s *Server) handleSessionsPreview(ctx *MethodContext) error {
	var params SessionsPreviewParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	// TODO: 实际获取预览
	ctx.Respond(true, map[string]interface{}{
		"key":      params.Key,
		"messages": []interface{}{},
	})
	return nil
}

type SessionKeyParams struct {
	Key string `json:"key"`
}

func (s *Server) handleSessionsReset(ctx *MethodContext) error {
	var params SessionKeyParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	// TODO: 实际重置会话
	ctx.Respond(true, map[string]interface{}{"reset": true})
	s.BroadcastEvent("stateChange", map[string]interface{}{"kind": "sessions"})
	return nil
}

func (s *Server) handleSessionsDelete(ctx *MethodContext) error {
	var params SessionKeyParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	if params.Key == "main" {
		ctx.RespondError(protocol.ErrorCodes.Forbidden, "Cannot delete main session")
		return nil
	}

	// TODO: 实际删除会话
	ctx.Respond(true, map[string]interface{}{"deleted": true})
	s.BroadcastEvent("stateChange", map[string]interface{}{"kind": "sessions"})
	return nil
}

func (s *Server) handleSessionsCompact(ctx *MethodContext) error {
	var params SessionKeyParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	// TODO: 实际压缩会话
	ctx.Respond(true, map[string]interface{}{
		"compacted":      true,
		"messagesBefore": 0,
		"messagesAfter":  0,
	})
	return nil
}

// ============================================================================
// Channels handlers
// ============================================================================

type ChannelStatus struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Status  string `json:"status"` // "connected" | "disconnected" | "connecting" | "error"
	Error   string `json:"error,omitempty"`
	Account string `json:"account,omitempty"`
}

func (s *Server) handleChannelsStatus(ctx *MethodContext) error {
	channels := []ChannelStatus{}

	if s.cfg.Channels.Telegram != nil && s.cfg.Channels.Telegram.Enabled {
		channels = append(channels, ChannelStatus{
			ID:     "telegram",
			Label:  "Telegram",
			Status: "disconnected",
		})
	}

	if s.cfg.Channels.WhatsApp != nil && s.cfg.Channels.WhatsApp.Enabled {
		channels = append(channels, ChannelStatus{
			ID:     "whatsapp",
			Label:  "WhatsApp",
			Status: "disconnected",
		})
	}

	if s.cfg.Channels.Discord != nil && s.cfg.Channels.Discord.Enabled {
		channels = append(channels, ChannelStatus{
			ID:     "discord",
			Label:  "Discord",
			Status: "disconnected",
		})
	}

	if s.cfg.Channels.Signal != nil && s.cfg.Channels.Signal.Enabled {
		channels = append(channels, ChannelStatus{
			ID:     "signal",
			Label:  "Signal",
			Status: "disconnected",
		})
	}

	ctx.Respond(true, map[string]interface{}{"channels": channels})
	return nil
}

type ChannelLogoutParams struct {
	Channel string `json:"channel"`
}

func (s *Server) handleChannelsLogout(ctx *MethodContext) error {
	var params ChannelLogoutParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	// TODO: 实际登出渠道
	ctx.Respond(true, map[string]interface{}{"loggedOut": true})
	s.BroadcastEvent("stateChange", map[string]interface{}{"kind": "channels"})
	return nil
}

// ============================================================================
// Agents handlers
// ============================================================================

type AgentSummary struct {
	ID        string `json:"id"`
	Label     string `json:"label,omitempty"`
	Model     string `json:"model,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Status    string `json:"status,omitempty"`
}

func (s *Server) handleAgentsList(ctx *MethodContext) error {
	agents := []AgentSummary{
		{
			ID:        "main",
			Label:     "Main Agent",
			Model:     s.cfg.Agent.DefaultModel,
			Workspace: s.cfg.Agent.Workspace,
			Status:    "idle",
		},
	}
	ctx.Respond(true, map[string]interface{}{"agents": agents})
	return nil
}

func (s *Server) handleAgentIdentity(ctx *MethodContext) error {
	ctx.Respond(true, map[string]interface{}{
		"id":        "main",
		"name":      "OpenClaw Agent",
		"model":     s.cfg.Agent.DefaultModel,
		"workspace": s.cfg.Agent.Workspace,
	})
	return nil
}

func (s *Server) handleWake(ctx *MethodContext) error {
	// 唤醒 agent (用于 heartbeat)
	ctx.Respond(true, map[string]interface{}{"woken": true})
	return nil
}

// ============================================================================
// Models handlers
// ============================================================================

type ModelChoice struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Provider string   `json:"provider"`
	Tags     []string `json:"tags,omitempty"`
}

func (s *Server) handleModelsList(ctx *MethodContext) error {
	models := []ModelChoice{
		{ID: "anthropic/claude-opus-4-5", Label: "Claude Opus 4.5", Provider: "anthropic", Tags: []string{"flagship"}},
		{ID: "anthropic/claude-sonnet-4-20250514", Label: "Claude Sonnet 4", Provider: "anthropic", Tags: []string{"balanced"}},
		{ID: "anthropic/claude-haiku-3-5", Label: "Claude Haiku 3.5", Provider: "anthropic", Tags: []string{"fast"}},
		{ID: "openai/gpt-4o", Label: "GPT-4o", Provider: "openai", Tags: []string{"flagship"}},
		{ID: "openai/gpt-4o-mini", Label: "GPT-4o Mini", Provider: "openai", Tags: []string{"fast"}},
		{ID: "openai/o1", Label: "o1", Provider: "openai", Tags: []string{"reasoning"}},
		{ID: "openai/o3", Label: "o3", Provider: "openai", Tags: []string{"reasoning"}},
		{ID: "deepseek/deepseek-chat", Label: "DeepSeek Chat", Provider: "deepseek"},
		{ID: "deepseek/deepseek-reasoner", Label: "DeepSeek Reasoner", Provider: "deepseek", Tags: []string{"reasoning"}},
		{ID: "google/gemini-2.0-flash", Label: "Gemini 2.0 Flash", Provider: "google"},
		{ID: "google/gemini-2.5-pro", Label: "Gemini 2.5 Pro", Provider: "google", Tags: []string{"flagship"}},
	}
	ctx.Respond(true, map[string]interface{}{"models": models})
	return nil
}

// ============================================================================
// Cron handlers
// ============================================================================

type CronJobEntry struct {
	ID            string      `json:"id"`
	Name          string      `json:"name,omitempty"`
	Schedule      interface{} `json:"schedule"`
	Payload       interface{} `json:"payload"`
	SessionTarget string      `json:"sessionTarget"`
	Enabled       bool        `json:"enabled"`
	NextRunAt     *int64      `json:"nextRunAt,omitempty"`
	LastRunAt     *int64      `json:"lastRunAt,omitempty"`
}

func (s *Server) handleCronList(ctx *MethodContext) error {
	var jobs []CronJobEntry

	if s.deps.CronScheduler != nil {
		for _, job := range s.deps.CronScheduler.ListJobs(true) {
			entry := CronJobEntry{
				ID:            job.ID,
				Name:          job.Name,
				Schedule:      job.Schedule,
				Payload:       job.Payload,
				SessionTarget: job.SessionTarget,
				Enabled:       job.Enabled,
			}
			if job.NextRunAt != nil {
				ms := job.NextRunAt.UnixMilli()
				entry.NextRunAt = &ms
			}
			if job.LastRunAt != nil {
				ms := job.LastRunAt.UnixMilli()
				entry.LastRunAt = &ms
			}
			jobs = append(jobs, entry)
		}
	}

	ctx.Respond(true, map[string]interface{}{"jobs": jobs})
	return nil
}

func (s *Server) handleCronStatus(ctx *MethodContext) error {
	status := map[string]interface{}{
		"enabled":  true,
		"jobCount": 0,
	}

	if s.deps.CronScheduler != nil {
		cronStatus := s.deps.CronScheduler.Status()
		status = cronStatus
	}

	ctx.Respond(true, status)
	return nil
}

type CronAddParams struct {
	Name          string      `json:"name,omitempty"`
	Schedule      cron.Schedule `json:"schedule"`
	Payload       cron.Payload `json:"payload"`
	SessionTarget string      `json:"sessionTarget,omitempty"`
	Enabled       *bool       `json:"enabled,omitempty"`
}

func (s *Server) handleCronAdd(ctx *MethodContext) error {
	var params CronAddParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	if s.deps.CronScheduler == nil {
		ctx.RespondError(protocol.ErrorCodes.ServiceUnavailable, "Cron scheduler not available")
		return nil
	}

	enabled := true
	if params.Enabled != nil {
		enabled = *params.Enabled
	}

	job := &cron.Job{
		Name:          params.Name,
		Schedule:      params.Schedule,
		Payload:       params.Payload,
		SessionTarget: params.SessionTarget,
		Enabled:       enabled,
	}

	if err := s.deps.CronScheduler.AddJob(job); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InternalError, err.Error())
		return nil
	}

	ctx.Respond(true, map[string]interface{}{"id": job.ID})
	s.BroadcastEvent("stateChange", map[string]interface{}{"kind": "cron"})
	return nil
}

type CronUpdateParams struct {
	ID     string                 `json:"id"`
	Patch  map[string]interface{} `json:"patch"`
}

func (s *Server) handleCronUpdate(ctx *MethodContext) error {
	var params CronUpdateParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	if s.deps.CronScheduler == nil {
		ctx.RespondError(protocol.ErrorCodes.ServiceUnavailable, "Cron scheduler not available")
		return nil
	}

	if err := s.deps.CronScheduler.UpdateJob(params.ID, params.Patch); err != nil {
		ctx.RespondError(protocol.ErrorCodes.NotFound, err.Error())
		return nil
	}

	ctx.Respond(true, map[string]interface{}{"updated": true})
	s.BroadcastEvent("stateChange", map[string]interface{}{"kind": "cron"})
	return nil
}

type CronIDParams struct {
	ID string `json:"id"`
}

func (s *Server) handleCronRemove(ctx *MethodContext) error {
	var params CronIDParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	if s.deps.CronScheduler == nil {
		ctx.RespondError(protocol.ErrorCodes.ServiceUnavailable, "Cron scheduler not available")
		return nil
	}

	if err := s.deps.CronScheduler.RemoveJob(params.ID); err != nil {
		ctx.RespondError(protocol.ErrorCodes.NotFound, err.Error())
		return nil
	}

	ctx.Respond(true, map[string]interface{}{"removed": true})
	s.BroadcastEvent("stateChange", map[string]interface{}{"kind": "cron"})
	return nil
}

func (s *Server) handleCronRun(ctx *MethodContext) error {
	var params CronIDParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	if s.deps.CronScheduler == nil {
		ctx.RespondError(protocol.ErrorCodes.ServiceUnavailable, "Cron scheduler not available")
		return nil
	}

	if err := s.deps.CronScheduler.RunJob(params.ID); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InternalError, err.Error())
		return nil
	}

	ctx.Respond(true, map[string]interface{}{"ran": true})
	return nil
}

// ============================================================================
// Chat handlers
// ============================================================================

type ChatSendParams struct {
	SessionKey string `json:"sessionKey,omitempty"`
	Message    string `json:"message"`
	Channel    string `json:"channel,omitempty"`
	Model      string `json:"model,omitempty"`
}

func (s *Server) handleChatSend(ctx *MethodContext) error {
	var params ChatSendParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	// TODO: 实际处理消息发送
	// 1. 确定 session
	// 2. 调用 agent
	// 3. 流式返回响应

	msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())

	ctx.Respond(true, map[string]interface{}{
		"messageId": msgID,
		"status":    "queued",
	})

	// 广播 chat 事件
	s.BroadcastEvent("chat", map[string]interface{}{
		"sessionKey": params.SessionKey,
		"messageId":  msgID,
		"status":     "processing",
	})

	return nil
}

type ChatHistoryParams struct {
	SessionKey   string `json:"sessionKey,omitempty"`
	Limit        *int   `json:"limit,omitempty"`
	IncludeTools bool   `json:"includeTools,omitempty"`
}

func (s *Server) handleChatHistory(ctx *MethodContext) error {
	var params ChatHistoryParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	// TODO: 从 session 获取历史记录
	messages := []interface{}{}

	ctx.Respond(true, map[string]interface{}{"messages": messages})
	return nil
}

type ChatAbortParams struct {
	SessionKey string `json:"sessionKey,omitempty"`
	MessageID  string `json:"messageId,omitempty"`
}

func (s *Server) handleChatAbort(ctx *MethodContext) error {
	var params ChatAbortParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	// TODO: 实际中止处理

	ctx.Respond(true, map[string]interface{}{"aborted": true})
	return nil
}

type ChatInjectParams struct {
	SessionKey string `json:"sessionKey,omitempty"`
	Role       string `json:"role"` // "user" | "assistant"
	Content    string `json:"content"`
}

func (s *Server) handleChatInject(ctx *MethodContext) error {
	var params ChatInjectParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	// TODO: 注入消息到会话

	ctx.Respond(true, map[string]interface{}{"injected": true})
	return nil
}

// ============================================================================
// Skills handlers
// ============================================================================

func (s *Server) handleSkillsStatus(ctx *MethodContext) error {
	if s.deps.SkillLoader == nil {
		ctx.Respond(true, map[string]interface{}{
			"totalSkills":   0,
			"enabledSkills": 0,
			"skills":        []interface{}{},
		})
		return nil
	}

	status := s.deps.SkillLoader.Status()
	skillList := s.deps.SkillLoader.List()

	skills := make([]map[string]interface{}, len(skillList))
	for i, sk := range skillList {
		skills[i] = map[string]interface{}{
			"id":          sk.ID,
			"name":        sk.Name,
			"description": sk.Description,
			"version":     sk.Version,
			"enabled":     sk.Enabled,
			"source":      sk.Source,
			"toolCount":   len(sk.Tools),
		}
	}

	ctx.Respond(true, map[string]interface{}{
		"totalSkills":   status.TotalSkills,
		"enabledSkills": status.EnabledSkills,
		"totalTools":    status.TotalTools,
		"skills":        skills,
	})
	return nil
}

func (s *Server) handleSkillsBins(ctx *MethodContext) error {
	bins := make(map[string]string)
	if s.deps.SkillLoader != nil {
		bins = s.deps.SkillLoader.GetBinaries()
	}

	ctx.Respond(true, map[string]interface{}{"bins": bins})
	return nil
}

type SkillsInstallParams struct {
	Source string `json:"source"`
}

func (s *Server) handleSkillsInstall(ctx *MethodContext) error {
	var params SkillsInstallParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	if s.deps.SkillLoader == nil {
		ctx.RespondError(protocol.ErrorCodes.ServiceUnavailable, "Skill loader not available")
		return nil
	}

	skill, err := s.deps.SkillLoader.Install(params.Source)
	if err != nil {
		ctx.RespondError(protocol.ErrorCodes.InternalError, err.Error())
		return nil
	}

	ctx.Respond(true, map[string]interface{}{
		"id":   skill.ID,
		"name": skill.Name,
	})
	return nil
}

type SkillsUpdateParams struct {
	ID string `json:"id"`
}

func (s *Server) handleSkillsUpdate(ctx *MethodContext) error {
	var params SkillsUpdateParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	if s.deps.SkillLoader == nil {
		ctx.RespondError(protocol.ErrorCodes.ServiceUnavailable, "Skill loader not available")
		return nil
	}

	if err := s.deps.SkillLoader.Update(params.ID); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InternalError, err.Error())
		return nil
	}

	ctx.Respond(true, map[string]interface{}{"updated": true})
	return nil
}

// ============================================================================
// Logs handlers
// ============================================================================

type LogsTailParams struct {
	Lines  *int   `json:"lines,omitempty"`
	Filter string `json:"filter,omitempty"`
}

func (s *Server) handleLogsTail(ctx *MethodContext) error {
	var params LogsTailParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	// TODO: 实际日志获取
	ctx.Respond(true, map[string]interface{}{
		"logs": []string{},
	})
	return nil
}

// ============================================================================
// Exec approvals handlers
// ============================================================================

func (s *Server) handleExecApprovalsGet(ctx *MethodContext) error {
	ctx.Respond(true, map[string]interface{}{
		"mode":      "allowlist",
		"allowlist": []string{"ls", "cat", "grep", "git"},
	})
	return nil
}

type ExecApprovalsSetParams struct {
	Mode      string   `json:"mode"`
	Allowlist []string `json:"allowlist,omitempty"`
}

func (s *Server) handleExecApprovalsSet(ctx *MethodContext) error {
	var params ExecApprovalsSetParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	// TODO: 实际设置
	ctx.Respond(true, map[string]interface{}{"set": true})
	return nil
}

// ============================================================================
// Node handlers
// ============================================================================

func (s *Server) handleNodeList(ctx *MethodContext) error {
	ctx.Respond(true, map[string]interface{}{
		"nodes": []interface{}{},
	})
	return nil
}

type NodeDescribeParams struct {
	ID string `json:"id"`
}

func (s *Server) handleNodeDescribe(ctx *MethodContext) error {
	var params NodeDescribeParams
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		ctx.RespondError(protocol.ErrorCodes.InvalidParams, "Invalid params")
		return nil
	}

	ctx.RespondError(protocol.ErrorCodes.NotFound, "Node not found")
	return nil
}
