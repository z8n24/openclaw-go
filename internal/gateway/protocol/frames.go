package protocol

import "encoding/json"

const (
	// PROTOCOL_VERSION 当前协议版本，与 TypeScript 版本保持一致
	PROTOCOL_VERSION = 3
)

// FrameType 帧类型
type FrameType string

const (
	FrameTypeRequest  FrameType = "req"
	FrameTypeResponse FrameType = "res"
	FrameTypeEvent    FrameType = "event"
)

// Frame 是所有帧的基础接口
type Frame interface {
	GetType() FrameType
}

// RequestFrame 请求帧
type RequestFrame struct {
	Type   FrameType       `json:"type"` // always "req"
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

func (f *RequestFrame) GetType() FrameType { return FrameTypeRequest }

// ResponseFrame 响应帧
type ResponseFrame struct {
	Type    FrameType       `json:"type"` // always "res"
	ID      string          `json:"id"`
	OK      bool            `json:"ok"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *ErrorShape     `json:"error,omitempty"`
}

func (f *ResponseFrame) GetType() FrameType { return FrameTypeResponse }

// EventFrame 事件帧
type EventFrame struct {
	Type         FrameType       `json:"type"` // always "event"
	Event        string          `json:"event"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	Seq          *int64          `json:"seq,omitempty"`
	StateVersion *StateVersion   `json:"stateVersion,omitempty"`
}

func (f *EventFrame) GetType() FrameType { return FrameTypeEvent }

// ErrorShape 错误结构
type ErrorShape struct {
	Code         string      `json:"code"`
	Message      string      `json:"message"`
	Details      interface{} `json:"details,omitempty"`
	Retryable    *bool       `json:"retryable,omitempty"`
	RetryAfterMs *int64      `json:"retryAfterMs,omitempty"`
}

// Error 实现 error 接口
func (e *ErrorShape) Error() string {
	return e.Code + ": " + e.Message
}

// StateVersion 状态版本
type StateVersion struct {
	Sessions int64 `json:"sessions"`
	Channels int64 `json:"channels"`
	Nodes    int64 `json:"nodes"`
	Cron     int64 `json:"cron"`
}

// ConnectParams 连接参数
type ConnectParams struct {
	MinProtocol int          `json:"minProtocol"`
	MaxProtocol int          `json:"maxProtocol"`
	Client      ClientInfo   `json:"client"`
	Caps        []string     `json:"caps,omitempty"`
	Commands    []string     `json:"commands,omitempty"`
	Permissions map[string]bool `json:"permissions,omitempty"`
	PathEnv     string       `json:"pathEnv,omitempty"`
	Role        string       `json:"role,omitempty"`
	Scopes      []string     `json:"scopes,omitempty"`
	Device      *DeviceInfo  `json:"device,omitempty"`
	Auth        *AuthInfo    `json:"auth,omitempty"`
	Locale      string       `json:"locale,omitempty"`
	UserAgent   string       `json:"userAgent,omitempty"`
}

// ClientInfo 客户端信息
type ClientInfo struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName,omitempty"`
	Version         string `json:"version"`
	Platform        string `json:"platform"`
	DeviceFamily    string `json:"deviceFamily,omitempty"`
	ModelIdentifier string `json:"modelIdentifier,omitempty"`
	Mode            string `json:"mode"` // "control" | "agent" | "node"
	InstanceID      string `json:"instanceId,omitempty"`
}

// DeviceInfo 设备信息 (用于配对)
type DeviceInfo struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
	SignedAt  int64  `json:"signedAt"`
	Nonce     string `json:"nonce,omitempty"`
}

// AuthInfo 认证信息
type AuthInfo struct {
	Token    string `json:"token,omitempty"`
	Password string `json:"password,omitempty"`
}

// HelloOK 握手成功响应
type HelloOK struct {
	Type          string        `json:"type"` // always "hello-ok"
	Protocol      int           `json:"protocol"`
	Server        ServerInfo    `json:"server"`
	Features      Features      `json:"features"`
	Snapshot      Snapshot      `json:"snapshot"`
	CanvasHostURL string        `json:"canvasHostUrl,omitempty"`
	Auth          *AuthResponse `json:"auth,omitempty"`
	Policy        Policy        `json:"policy"`
}

// ServerInfo 服务器信息
type ServerInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Host    string `json:"host,omitempty"`
	ConnID  string `json:"connId"`
}

// Features 支持的功能
type Features struct {
	Methods []string `json:"methods"`
	Events  []string `json:"events"`
}

// Snapshot 状态快照
type Snapshot struct {
	Presence     []PresenceEntry `json:"presence"`
	StateVersion StateVersion    `json:"stateVersion"`
}

// PresenceEntry 在线状态
type PresenceEntry struct {
	ConnID      string `json:"connId"`
	ClientID    string `json:"clientId"`
	DisplayName string `json:"displayName,omitempty"`
	Mode        string `json:"mode"`
	ConnectedAt int64  `json:"connectedAt"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	DeviceToken string   `json:"deviceToken"`
	Role        string   `json:"role"`
	Scopes      []string `json:"scopes"`
	IssuedAtMs  *int64   `json:"issuedAtMs,omitempty"`
}

// Policy 策略限制
type Policy struct {
	MaxPayload       int `json:"maxPayload"`
	MaxBufferedBytes int `json:"maxBufferedBytes"`
	TickIntervalMs   int `json:"tickIntervalMs"`
}

// TickEvent 心跳事件
type TickEvent struct {
	Ts int64 `json:"ts"`
}

// ShutdownEvent 关闭事件
type ShutdownEvent struct {
	Reason            string `json:"reason"`
	RestartExpectedMs *int64 `json:"restartExpectedMs,omitempty"`
}

// ParseFrame 解析帧
func ParseFrame(data []byte) (Frame, error) {
	// 先解析 type 字段
	var peek struct {
		Type FrameType `json:"type"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, err
	}

	switch peek.Type {
	case FrameTypeRequest:
		var f RequestFrame
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case FrameTypeResponse:
		var f ResponseFrame
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case FrameTypeEvent:
		var f EventFrame
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	default:
		return nil, &ErrorShape{Code: "INVALID_FRAME", Message: "unknown frame type"}
	}
}
