package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// NodesTool 设备配对和控制工具
type NodesTool struct {
	// 节点管理器接口
	manager NodesManager
}

// NodesManager 节点管理接口
type NodesManager interface {
	// 状态查询
	GetStatus() ([]NodeInfo, error)
	DescribeNode(node string) (*NodeDescription, error)
	
	// 配对管理
	GetPendingPairings() ([]PairingRequest, error)
	ApprovePairing(requestID string) error
	RejectPairing(requestID string) error
	
	// 通知
	SendNotification(node string, opts NotificationOptions) error
	
	// 相机
	CameraSnap(node string, opts CameraOptions) ([]byte, error)
	CameraList(node string) ([]CameraInfo, error)
	CameraClip(node string, opts CameraClipOptions) (string, error)
	
	// 屏幕
	ScreenRecord(node string, opts ScreenRecordOptions) (string, error)
	
	// 位置
	GetLocation(node string, opts LocationOptions) (*LocationInfo, error)
	
	// 命令执行
	RunCommand(node string, command []string, opts RunOptions) (string, error)
}

// NodeInfo 节点信息
type NodeInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Platform    string    `json:"platform"` // ios, android, macos, windows, linux
	Online      bool      `json:"online"`
	LastSeen    time.Time `json:"lastSeen,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// NodeDescription 节点详细描述
type NodeDescription struct {
	NodeInfo
	DeviceModel  string            `json:"deviceModel,omitempty"`
	OSVersion    string            `json:"osVersion,omitempty"`
	AppVersion   string            `json:"appVersion,omitempty"`
	Battery      int               `json:"battery,omitempty"`
	Charging     bool              `json:"charging,omitempty"`
	Network      string            `json:"network,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// PairingRequest 配对请求
type PairingRequest struct {
	RequestID   string    `json:"requestId"`
	DeviceName  string    `json:"deviceName"`
	Platform    string    `json:"platform"`
	RequestedAt time.Time `json:"requestedAt"`
}

// NotificationOptions 通知选项
type NotificationOptions struct {
	Title    string `json:"title,omitempty"`
	Body     string `json:"body"`
	Sound    string `json:"sound,omitempty"`
	Priority string `json:"priority,omitempty"` // passive, active, timeSensitive
	Delivery string `json:"delivery,omitempty"` // system, overlay, auto
}

// CameraOptions 拍照选项
type CameraOptions struct {
	Facing   string `json:"facing,omitempty"` // front, back, both
	MaxWidth int    `json:"maxWidth,omitempty"`
	Quality  int    `json:"quality,omitempty"` // 0-100
}

// CameraInfo 相机信息
type CameraInfo struct {
	DeviceID string `json:"deviceId"`
	Label    string `json:"label"`
	Facing   string `json:"facing,omitempty"`
}

// CameraClipOptions 录像选项
type CameraClipOptions struct {
	Facing     string `json:"facing,omitempty"`
	DurationMs int    `json:"durationMs,omitempty"`
	FPS        int    `json:"fps,omitempty"`
	MaxWidth   int    `json:"maxWidth,omitempty"`
	OutPath    string `json:"outPath,omitempty"`
}

// ScreenRecordOptions 屏幕录制选项
type ScreenRecordOptions struct {
	DurationMs   int    `json:"durationMs,omitempty"`
	ScreenIndex  int    `json:"screenIndex,omitempty"`
	IncludeAudio bool   `json:"includeAudio,omitempty"`
	OutPath      string `json:"outPath,omitempty"`
}

// LocationOptions 位置选项
type LocationOptions struct {
	DesiredAccuracy   string `json:"desiredAccuracy,omitempty"` // coarse, balanced, precise
	MaxAgeMs          int    `json:"maxAgeMs,omitempty"`
	LocationTimeoutMs int    `json:"locationTimeoutMs,omitempty"`
}

// LocationInfo 位置信息
type LocationInfo struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Accuracy  float64   `json:"accuracy,omitempty"`
	Altitude  float64   `json:"altitude,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// RunOptions 命令执行选项
type RunOptions struct {
	Cwd              string            `json:"cwd,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	TimeoutMs        int               `json:"timeoutMs,omitempty"`
	CommandTimeoutMs int               `json:"commandTimeoutMs,omitempty"`
}

// NodesParams nodes 工具参数
type NodesParams struct {
	Action            string   `json:"action"` // status, describe, pending, approve, reject, notify, camera_snap, camera_list, camera_clip, screen_record, location_get, run
	Node              string   `json:"node,omitempty"`
	RequestID         string   `json:"requestId,omitempty"`
	
	// Notification
	Title    string `json:"title,omitempty"`
	Body     string `json:"body,omitempty"`
	Sound    string `json:"sound,omitempty"`
	Priority string `json:"priority,omitempty"`
	Delivery string `json:"delivery,omitempty"`
	
	// Camera
	Facing   string `json:"facing,omitempty"`
	MaxWidth int    `json:"maxWidth,omitempty"`
	Quality  int    `json:"quality,omitempty"`
	
	// Recording
	DurationMs   int    `json:"durationMs,omitempty"`
	Duration     string `json:"duration,omitempty"`
	FPS          int    `json:"fps,omitempty"`
	ScreenIndex  int    `json:"screenIndex,omitempty"`
	IncludeAudio bool   `json:"includeAudio,omitempty"`
	OutPath      string `json:"outPath,omitempty"`
	
	// Location
	DesiredAccuracy   string `json:"desiredAccuracy,omitempty"`
	MaxAgeMs          int    `json:"maxAgeMs,omitempty"`
	LocationTimeoutMs int    `json:"locationTimeoutMs,omitempty"`
	
	// Run command
	Command          []string          `json:"command,omitempty"`
	Cwd              string            `json:"cwd,omitempty"`
	Env              []string          `json:"env,omitempty"`
	TimeoutMs        int               `json:"timeoutMs,omitempty"`
	CommandTimeoutMs int               `json:"commandTimeoutMs,omitempty"`
}

// NewNodesTool 创建 nodes 工具
func NewNodesTool() *NodesTool {
	return &NodesTool{}
}

func (t *NodesTool) Name() string {
	return ToolNodes
}

func (t *NodesTool) Description() string {
	return "Discover and control paired nodes (status/describe/pairing/notify/camera/screen/location/run)."
}

func (t *NodesTool) Parameters() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["status", "describe", "pending", "approve", "reject", "notify", "camera_snap", "camera_list", "camera_clip", "screen_record", "location_get", "run"],
				"description": "Nodes action"
			},
			"node": {
				"type": "string",
				"description": "Target node id or name"
			},
			"requestId": {
				"type": "string",
				"description": "Pairing request ID for approve/reject"
			},
			"title": {
				"type": "string",
				"description": "Notification title"
			},
			"body": {
				"type": "string",
				"description": "Notification body"
			},
			"sound": {
				"type": "string",
				"description": "Notification sound"
			},
			"priority": {
				"type": "string",
				"enum": ["passive", "active", "timeSensitive"],
				"description": "Notification priority"
			},
			"delivery": {
				"type": "string",
				"enum": ["system", "overlay", "auto"],
				"description": "Notification delivery method"
			},
			"facing": {
				"type": "string",
				"enum": ["front", "back", "both"],
				"description": "Camera facing direction"
			},
			"maxWidth": {
				"type": "number",
				"description": "Maximum width for camera/recording"
			},
			"quality": {
				"type": "number",
				"description": "Image quality (0-100)"
			},
			"durationMs": {
				"type": "number",
				"description": "Recording duration in milliseconds"
			},
			"fps": {
				"type": "number",
				"description": "Frames per second for recording"
			},
			"screenIndex": {
				"type": "number",
				"description": "Screen index for recording"
			},
			"includeAudio": {
				"type": "boolean",
				"description": "Include audio in screen recording"
			},
			"outPath": {
				"type": "string",
				"description": "Output path for recordings"
			},
			"desiredAccuracy": {
				"type": "string",
				"enum": ["coarse", "balanced", "precise"],
				"description": "Location accuracy"
			},
			"maxAgeMs": {
				"type": "number",
				"description": "Maximum age of cached location"
			},
			"locationTimeoutMs": {
				"type": "number",
				"description": "Location request timeout"
			},
			"command": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Command to run on node"
			},
			"cwd": {
				"type": "string",
				"description": "Working directory for command"
			},
			"env": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Environment variables (KEY=VALUE format)"
			},
			"timeoutMs": {
				"type": "number",
				"description": "Overall operation timeout"
			},
			"commandTimeoutMs": {
				"type": "number",
				"description": "Command execution timeout"
			}
		},
		"required": ["action"]
	}`
	return json.RawMessage(schema)
}

func (t *NodesTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params NodesParams
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Content: "Invalid parameters: " + err.Error(), IsError: true}, nil
	}

	if t.manager == nil {
		return &Result{Content: "Nodes manager not configured", IsError: true}, nil
	}

	switch params.Action {
	case "status":
		return t.status()
	case "describe":
		return t.describe(params.Node)
	case "pending":
		return t.pending()
	case "approve":
		return t.approve(params.RequestID)
	case "reject":
		return t.reject(params.RequestID)
	case "notify":
		return t.notify(ctx, &params)
	case "camera_snap":
		return t.cameraSnap(ctx, &params)
	case "camera_list":
		return t.cameraList(params.Node)
	case "camera_clip":
		return t.cameraClip(ctx, &params)
	case "screen_record":
		return t.screenRecord(ctx, &params)
	case "location_get":
		return t.locationGet(ctx, &params)
	case "run":
		return t.run(ctx, &params)
	default:
		return &Result{Content: "Unknown action: " + params.Action, IsError: true}, nil
	}
}

func (t *NodesTool) status() (*Result, error) {
	nodes, err := t.manager.GetStatus()
	if err != nil {
		return &Result{Content: "Failed to get status: " + err.Error(), IsError: true}, nil
	}

	data, _ := json.MarshalIndent(nodes, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *NodesTool) describe(node string) (*Result, error) {
	if node == "" {
		return &Result{Content: "Node id/name is required", IsError: true}, nil
	}

	desc, err := t.manager.DescribeNode(node)
	if err != nil {
		return &Result{Content: "Failed to describe node: " + err.Error(), IsError: true}, nil
	}

	data, _ := json.MarshalIndent(desc, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *NodesTool) pending() (*Result, error) {
	requests, err := t.manager.GetPendingPairings()
	if err != nil {
		return &Result{Content: "Failed to get pending pairings: " + err.Error(), IsError: true}, nil
	}

	data, _ := json.MarshalIndent(requests, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *NodesTool) approve(requestID string) (*Result, error) {
	if requestID == "" {
		return &Result{Content: "Request ID is required", IsError: true}, nil
	}

	if err := t.manager.ApprovePairing(requestID); err != nil {
		return &Result{Content: "Failed to approve: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: "Pairing approved"}, nil
}

func (t *NodesTool) reject(requestID string) (*Result, error) {
	if requestID == "" {
		return &Result{Content: "Request ID is required", IsError: true}, nil
	}

	if err := t.manager.RejectPairing(requestID); err != nil {
		return &Result{Content: "Failed to reject: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: "Pairing rejected"}, nil
}

func (t *NodesTool) notify(ctx context.Context, params *NodesParams) (*Result, error) {
	if params.Node == "" {
		return &Result{Content: "Node is required", IsError: true}, nil
	}
	if params.Body == "" {
		return &Result{Content: "Body is required for notification", IsError: true}, nil
	}

	opts := NotificationOptions{
		Title:    params.Title,
		Body:     params.Body,
		Sound:    params.Sound,
		Priority: params.Priority,
		Delivery: params.Delivery,
	}

	if err := t.manager.SendNotification(params.Node, opts); err != nil {
		return &Result{Content: "Failed to send notification: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: "Notification sent"}, nil
}

func (t *NodesTool) cameraSnap(ctx context.Context, params *NodesParams) (*Result, error) {
	if params.Node == "" {
		return &Result{Content: "Node is required", IsError: true}, nil
	}

	opts := CameraOptions{
		Facing:   params.Facing,
		MaxWidth: params.MaxWidth,
		Quality:  params.Quality,
	}

	data, err := t.manager.CameraSnap(params.Node, opts)
	if err != nil {
		return &Result{Content: "Failed to capture: " + err.Error(), IsError: true}, nil
	}

	return &Result{
		Content: fmt.Sprintf("Captured %d bytes", len(data)),
		Media: []MediaItem{
			{Type: "image", MimeType: "image/jpeg"},
		},
	}, nil
}

func (t *NodesTool) cameraList(node string) (*Result, error) {
	if node == "" {
		return &Result{Content: "Node is required", IsError: true}, nil
	}

	cameras, err := t.manager.CameraList(node)
	if err != nil {
		return &Result{Content: "Failed to list cameras: " + err.Error(), IsError: true}, nil
	}

	data, _ := json.MarshalIndent(cameras, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *NodesTool) cameraClip(ctx context.Context, params *NodesParams) (*Result, error) {
	if params.Node == "" {
		return &Result{Content: "Node is required", IsError: true}, nil
	}

	opts := CameraClipOptions{
		Facing:     params.Facing,
		DurationMs: params.DurationMs,
		FPS:        params.FPS,
		MaxWidth:   params.MaxWidth,
		OutPath:    params.OutPath,
	}

	path, err := t.manager.CameraClip(params.Node, opts)
	if err != nil {
		return &Result{Content: "Failed to record clip: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Clip saved to: %s", path)}, nil
}

func (t *NodesTool) screenRecord(ctx context.Context, params *NodesParams) (*Result, error) {
	if params.Node == "" {
		return &Result{Content: "Node is required", IsError: true}, nil
	}

	opts := ScreenRecordOptions{
		DurationMs:   params.DurationMs,
		ScreenIndex:  params.ScreenIndex,
		IncludeAudio: params.IncludeAudio,
		OutPath:      params.OutPath,
	}

	path, err := t.manager.ScreenRecord(params.Node, opts)
	if err != nil {
		return &Result{Content: "Failed to record screen: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("Recording saved to: %s", path)}, nil
}

func (t *NodesTool) locationGet(ctx context.Context, params *NodesParams) (*Result, error) {
	if params.Node == "" {
		return &Result{Content: "Node is required", IsError: true}, nil
	}

	opts := LocationOptions{
		DesiredAccuracy:   params.DesiredAccuracy,
		MaxAgeMs:          params.MaxAgeMs,
		LocationTimeoutMs: params.LocationTimeoutMs,
	}

	loc, err := t.manager.GetLocation(params.Node, opts)
	if err != nil {
		return &Result{Content: "Failed to get location: " + err.Error(), IsError: true}, nil
	}

	data, _ := json.MarshalIndent(loc, "", "  ")
	return &Result{Content: string(data)}, nil
}

func (t *NodesTool) run(ctx context.Context, params *NodesParams) (*Result, error) {
	if params.Node == "" {
		return &Result{Content: "Node is required", IsError: true}, nil
	}
	if len(params.Command) == 0 {
		return &Result{Content: "Command is required", IsError: true}, nil
	}

	// 解析环境变量
	env := make(map[string]string)
	for _, e := range params.Env {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				env[e[:i]] = e[i+1:]
				break
			}
		}
	}

	opts := RunOptions{
		Cwd:              params.Cwd,
		Env:              env,
		TimeoutMs:        params.TimeoutMs,
		CommandTimeoutMs: params.CommandTimeoutMs,
	}

	output, err := t.manager.RunCommand(params.Node, params.Command, opts)
	if err != nil {
		return &Result{Content: "Command failed: " + err.Error(), IsError: true}, nil
	}

	return &Result{Content: output}, nil
}

// SetManager 设置节点管理器
func (t *NodesTool) SetManager(m NodesManager) {
	t.manager = m
}
