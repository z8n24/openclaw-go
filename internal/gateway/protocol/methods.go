package protocol

// 所有支持的 RPC 方法
var SupportedMethods = []string{
	// Agent 相关
	"agent.send",
	"agent.poll",
	"agent.identity",
	"agent.wait",
	"agents.list",
	"wake",

	// Session 相关
	"sessions.list",
	"sessions.preview",
	"sessions.resolve",
	"sessions.patch",
	"sessions.reset",
	"sessions.delete",
	"sessions.compact",

	// Chat 相关
	"chat.history",
	"chat.send",
	"chat.abort",
	"chat.inject",

	// Config 相关
	"config.get",
	"config.set",
	"config.apply",
	"config.patch",
	"config.schema",

	// Channels 相关
	"channels.status",
	"channels.logout",

	// Node 相关
	"node.pair.request",
	"node.pair.list",
	"node.pair.approve",
	"node.pair.reject",
	"node.pair.verify",
	"node.rename",
	"node.list",
	"node.describe",
	"node.invoke",
	"node.invoke.result",
	"node.event",

	// Cron 相关
	"cron.list",
	"cron.status",
	"cron.add",
	"cron.update",
	"cron.remove",
	"cron.run",
	"cron.runs",

	// Device 相关
	"device.pair.list",
	"device.pair.approve",
	"device.pair.reject",
	"device.token.rotate",
	"device.token.revoke",

	// Exec Approvals
	"exec.approvals.get",
	"exec.approvals.set",
	"exec.approval.request",
	"exec.approval.resolve",
	"exec.approvals.node.get",
	"exec.approvals.node.set",

	// Logs
	"logs.tail",

	// Models
	"models.list",

	// Skills
	"skills.status",
	"skills.bins",
	"skills.install",
	"skills.update",

	// Wizard
	"wizard.start",
	"wizard.next",
	"wizard.cancel",
	"wizard.status",

	// Talk Mode
	"talkMode",

	// Update
	"update.run",

	// Web Login
	"webLogin.start",
	"webLogin.wait",
}

// 所有支持的事件
var SupportedEvents = []string{
	"tick",
	"shutdown",
	"presence",
	"stateChange",
	"agent",
	"chat",
	"node.invoke.request",
	"device.pair.requested",
	"device.pair.resolved",
	"exec.approval.request",
}

// ErrorCodes 标准错误码
var ErrorCodes = struct {
	InvalidRequest    string
	MethodNotFound    string
	InvalidParams     string
	InternalError     string
	Unauthorized      string
	Forbidden         string
	NotFound          string
	Conflict          string
	RateLimited       string
	ServiceUnavailable string
}{
	InvalidRequest:    "INVALID_REQUEST",
	MethodNotFound:    "METHOD_NOT_FOUND",
	InvalidParams:     "INVALID_PARAMS",
	InternalError:     "INTERNAL_ERROR",
	Unauthorized:      "UNAUTHORIZED",
	Forbidden:         "FORBIDDEN",
	NotFound:          "NOT_FOUND",
	Conflict:          "CONFLICT",
	RateLimited:       "RATE_LIMITED",
	ServiceUnavailable: "SERVICE_UNAVAILABLE",
}

// NewError 创建标准错误
func NewError(code, message string) *ErrorShape {
	return &ErrorShape{
		Code:    code,
		Message: message,
	}
}

// NewErrorWithDetails 创建带详情的错误
func NewErrorWithDetails(code, message string, details interface{}) *ErrorShape {
	return &ErrorShape{
		Code:    code,
		Message: message,
		Details: details,
	}
}
