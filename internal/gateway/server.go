package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/z8n24/openclaw-go/internal/config"
	"github.com/z8n24/openclaw-go/internal/gateway/protocol"
)

const (
	Version = "0.1.0"
)

// Server 是 Gateway 服务器
type Server struct {
	cfg      *config.Config
	addr     string
	token    string
	
	// WebSocket
	upgrader websocket.Upgrader
	clients  map[string]*Client
	clientMu sync.RWMutex
	
	// RPC 方法处理器
	handlers map[string]MethodHandler
	handlerMu sync.RWMutex
	
	// 状态
	stateVersion protocol.StateVersion
	stateMu      sync.RWMutex
	
	// HTTP 服务器
	httpServer *http.Server
	
	// 依赖注入
	deps Dependencies
	
	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
}

// Client 表示一个 WebSocket 连接
type Client struct {
	ID          string
	Conn        *websocket.Conn
	Info        protocol.ClientInfo
	ConnectedAt time.Time
	
	sendMu sync.Mutex
	server *Server
}

// MethodHandler 是 RPC 方法处理器
type MethodHandler func(ctx *MethodContext) error

// MethodContext 是方法调用上下文
type MethodContext struct {
	Client  *Client
	Request *protocol.RequestFrame
	Server  *Server
}

// NewServer 创建新的 Gateway 服务器
func NewServer(cfg *config.Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	
	s := &Server{
		cfg:    cfg,
		addr:   fmt.Sprintf("%s:%d", cfg.Gateway.Bind, cfg.Gateway.Port),
		token:  cfg.Gateway.Token,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024 * 64,
			WriteBufferSize: 1024 * 64,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		clients:  make(map[string]*Client),
		handlers: make(map[string]MethodHandler),
		ctx:      ctx,
		cancel:   cancel,
	}
	
	// 注册默认处理器
	s.registerDefaultHandlers()
	
	return s
}

// RegisterHandler 注册 RPC 方法处理器
func (s *Server) RegisterHandler(method string, handler MethodHandler) {
	s.handlerMu.Lock()
	defer s.handlerMu.Unlock()
	s.handlers[method] = handler
}

// Start 启动服务器
func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	
	// 首页 - 优先使用 web/index.html
	if _, err := os.Stat("./web/index.html"); err == nil {
		router.GET("/", func(c *gin.Context) {
			c.File("./web/index.html")
		})
		router.StaticFS("/ui", http.Dir("./web"))
	} else {
		router.GET("/", s.handleIndex)
	}
	
	// WebSocket endpoint
	router.GET("/ws", s.handleWebSocket)
	
	// HTTP API endpoints
	api := router.Group("/api")
	{
		api.GET("/status", s.handleStatus)
		api.GET("/health", s.handleHealth)
	}
	
	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: router,
	}
	
	log.Info().Str("addr", s.addr).Msg("Starting Gateway server")
	
	// 启动心跳
	go s.tickLoop()
	
	return s.httpServer.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.cancel()
	
	// 关闭所有客户端连接
	s.clientMu.Lock()
	for _, client := range s.clients {
		client.Conn.Close()
	}
	s.clientMu.Unlock()
	
	// 关闭 HTTP 服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// handleWebSocket 处理 WebSocket 连接
func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade WebSocket")
		return
	}
	
	connID := uuid.New().String()
	client := &Client{
		ID:          connID,
		Conn:        conn,
		ConnectedAt: time.Now(),
		server:      s,
	}
	
	// 等待 hello 消息
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read hello message")
		conn.Close()
		return
	}
	
	var connectParams protocol.ConnectParams
	if err := json.Unmarshal(msg, &connectParams); err != nil {
		s.sendError(conn, "", protocol.ErrorCodes.InvalidRequest, "Invalid connect params")
		conn.Close()
		return
	}
	
	// 验证 token
	if s.token != "" {
		if connectParams.Auth == nil || connectParams.Auth.Token != s.token {
			s.sendError(conn, "", protocol.ErrorCodes.Unauthorized, "Invalid token")
			conn.Close()
			return
		}
	}
	
	// 验证协议版本
	if connectParams.MinProtocol > protocol.PROTOCOL_VERSION {
		s.sendError(conn, "", protocol.ErrorCodes.InvalidRequest, 
			fmt.Sprintf("Protocol version %d not supported", connectParams.MinProtocol))
		conn.Close()
		return
	}
	
	client.Info = connectParams.Client
	
	// 发送 hello-ok
	helloOK := s.buildHelloOK(connID)
	if err := s.sendJSON(conn, helloOK); err != nil {
		log.Error().Err(err).Msg("Failed to send hello-ok")
		conn.Close()
		return
	}
	
	// 注册客户端
	s.clientMu.Lock()
	s.clients[connID] = client
	s.clientMu.Unlock()
	
	log.Info().
		Str("connId", connID).
		Str("clientId", client.Info.ID).
		Str("mode", client.Info.Mode).
		Msg("Client connected")
	
	// 广播 presence
	s.broadcastPresence()
	
	// 处理消息
	go s.handleClient(client)
}

// handleClient 处理客户端消息
func (s *Server) handleClient(client *Client) {
	defer func() {
		s.clientMu.Lock()
		delete(s.clients, client.ID)
		s.clientMu.Unlock()
		client.Conn.Close()
		s.broadcastPresence()
		log.Info().Str("connId", client.ID).Msg("Client disconnected")
	}()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		
		_, msg, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Msg("WebSocket read error")
			}
			return
		}
		
		frame, err := protocol.ParseFrame(msg)
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse frame")
			continue
		}
		
		switch f := frame.(type) {
		case *protocol.RequestFrame:
			go s.handleRequest(client, f)
		case *protocol.EventFrame:
			log.Debug().Str("event", f.Event).Msg("Received event from client")
		}
	}
}

// handleRequest 处理 RPC 请求
func (s *Server) handleRequest(client *Client, req *protocol.RequestFrame) {
	s.handlerMu.RLock()
	handler, ok := s.handlers[req.Method]
	s.handlerMu.RUnlock()
	
	if !ok {
		s.sendResponse(client, req.ID, false, nil, 
			protocol.NewError(protocol.ErrorCodes.MethodNotFound, 
				fmt.Sprintf("Method %s not found", req.Method)))
		return
	}
	
	ctx := &MethodContext{
		Client:  client,
		Request: req,
		Server:  s,
	}
	
	if err := handler(ctx); err != nil {
		s.sendResponse(client, req.ID, false, nil,
			protocol.NewError(protocol.ErrorCodes.InternalError, err.Error()))
	}
}

// sendResponse 发送响应
func (s *Server) sendResponse(client *Client, id string, ok bool, payload interface{}, err *protocol.ErrorShape) {
	var payloadBytes json.RawMessage
	if payload != nil {
		var e error
		payloadBytes, e = json.Marshal(payload)
		if e != nil {
			log.Error().Err(e).Msg("Failed to marshal payload")
			return
		}
	}
	
	resp := &protocol.ResponseFrame{
		Type:    protocol.FrameTypeResponse,
		ID:      id,
		OK:      ok,
		Payload: payloadBytes,
		Error:   err,
	}
	
	client.sendMu.Lock()
	defer client.sendMu.Unlock()
	
	if e := client.Conn.WriteJSON(resp); e != nil {
		log.Error().Err(e).Msg("Failed to send response")
	}
}

// Respond 便捷方法用于处理器
func (ctx *MethodContext) Respond(ok bool, payload interface{}) {
	ctx.Server.sendResponse(ctx.Client, ctx.Request.ID, ok, payload, nil)
}

// RespondError 便捷方法用于处理器
func (ctx *MethodContext) RespondError(code, message string) {
	ctx.Server.sendResponse(ctx.Client, ctx.Request.ID, false, nil,
		protocol.NewError(code, message))
}

// BroadcastEvent 广播事件到所有客户端
func (s *Server) BroadcastEvent(event string, payload interface{}) {
	var payloadBytes json.RawMessage
	if payload != nil {
		var err error
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal event payload")
			return
		}
	}
	
	s.stateMu.RLock()
	stateVersion := s.stateVersion
	s.stateMu.RUnlock()
	
	eventFrame := &protocol.EventFrame{
		Type:         protocol.FrameTypeEvent,
		Event:        event,
		Payload:      payloadBytes,
		StateVersion: &stateVersion,
	}
	
	s.clientMu.RLock()
	defer s.clientMu.RUnlock()
	
	for _, client := range s.clients {
		client.sendMu.Lock()
		client.Conn.WriteJSON(eventFrame)
		client.sendMu.Unlock()
	}
}

// tickLoop 心跳循环
func (s *Server) tickLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.BroadcastEvent("tick", &protocol.TickEvent{
				Ts: time.Now().UnixMilli(),
			})
		}
	}
}

// broadcastPresence 广播在线状态
func (s *Server) broadcastPresence() {
	s.clientMu.RLock()
	presence := make([]protocol.PresenceEntry, 0, len(s.clients))
	for _, client := range s.clients {
		presence = append(presence, protocol.PresenceEntry{
			ConnID:      client.ID,
			ClientID:    client.Info.ID,
			DisplayName: client.Info.DisplayName,
			Mode:        client.Info.Mode,
			ConnectedAt: client.ConnectedAt.UnixMilli(),
		})
	}
	s.clientMu.RUnlock()
	
	s.BroadcastEvent("presence", presence)
}

// buildHelloOK 构建握手响应
func (s *Server) buildHelloOK(connID string) *protocol.HelloOK {
	s.stateMu.RLock()
	stateVersion := s.stateVersion
	s.stateMu.RUnlock()
	
	s.clientMu.RLock()
	presence := make([]protocol.PresenceEntry, 0, len(s.clients))
	for _, client := range s.clients {
		presence = append(presence, protocol.PresenceEntry{
			ConnID:      client.ID,
			ClientID:    client.Info.ID,
			DisplayName: client.Info.DisplayName,
			Mode:        client.Info.Mode,
			ConnectedAt: client.ConnectedAt.UnixMilli(),
		})
	}
	s.clientMu.RUnlock()
	
	return &protocol.HelloOK{
		Type:     "hello-ok",
		Protocol: protocol.PROTOCOL_VERSION,
		Server: protocol.ServerInfo{
			Version: Version,
			ConnID:  connID,
		},
		Features: protocol.Features{
			Methods: protocol.SupportedMethods,
			Events:  protocol.SupportedEvents,
		},
		Snapshot: protocol.Snapshot{
			Presence:     presence,
			StateVersion: stateVersion,
		},
		Policy: protocol.Policy{
			MaxPayload:       1024 * 1024,      // 1MB
			MaxBufferedBytes: 1024 * 1024 * 10, // 10MB
			TickIntervalMs:   30000,
		},
	}
}

// HTTP handlers

func (s *Server) handleStatus(c *gin.Context) {
	s.clientMu.RLock()
	clientCount := len(s.clients)
	s.clientMu.RUnlock()
	
	c.JSON(http.StatusOK, gin.H{
		"version":     Version,
		"protocol":    protocol.PROTOCOL_VERSION,
		"clients":     clientCount,
		"uptime":      time.Since(time.Now()).String(), // TODO: track actual uptime
	})
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleIndex(c *gin.Context) {
	html := `<!DOCTYPE html>
<html>
<head>
	<title>OpenClaw Go Gateway</title>
	<style>
		body { font-family: system-ui, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
		h1 { color: #e44d26; }
		code { background: #f4f4f4; padding: 2px 6px; border-radius: 4px; }
		pre { background: #f4f4f4; padding: 15px; border-radius: 8px; overflow-x: auto; }
		a { color: #0066cc; }
	</style>
</head>
<body>
	<h1>🦞 OpenClaw Go Gateway</h1>
	<p>Version: ` + Version + ` | Protocol: 3</p>
	
	<h2>Endpoints</h2>
	<ul>
		<li><code>GET /api/status</code> - <a href="/api/status">Gateway status</a></li>
		<li><code>GET /api/health</code> - <a href="/api/health">Health check</a></li>
		<li><code>WS /ws</code> - WebSocket connection</li>
	</ul>
	
	<h2>WebSocket</h2>
	<pre>wscat -c ws://127.0.0.1:` + fmt.Sprintf("%d", s.cfg.Gateway.Port) + `/ws</pre>
	
	<h2>Status</h2>
	<pre id="status">Loading...</pre>
	
	<script>
		fetch('/api/status')
			.then(r => r.json())
			.then(d => document.getElementById('status').textContent = JSON.stringify(d, null, 2));
	</script>
</body>
</html>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// helpers

func (s *Server) sendJSON(conn *websocket.Conn, v interface{}) error {
	return conn.WriteJSON(v)
}

func (s *Server) sendError(conn *websocket.Conn, id, code, message string) {
	resp := &protocol.ResponseFrame{
		Type:  protocol.FrameTypeResponse,
		ID:    id,
		OK:    false,
		Error: protocol.NewError(code, message),
	}
	conn.WriteJSON(resp)
}
