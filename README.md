# OpenClaw Go

OpenClaw 的 Go 语言完整复刻版本。

## 架构概览

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CLI (cobra)                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         Gateway Server                               │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐   │   │
│  │  │ WebSocket    │  │ HTTP API     │  │ Control UI (静态文件)    │   │   │
│  │  │ Server       │  │ Server       │  │                          │   │   │
│  │  └──────────────┘  └──────────────┘  └──────────────────────────┘   │   │
│  │                                                                      │   │
│  │  ┌──────────────────────────────────────────────────────────────┐   │   │
│  │  │                    Session Manager                            │   │   │
│  │  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐  │   │   │
│  │  │  │ main    │  │ group:1 │  │ group:2 │  │ isolated:xxx    │  │   │   │
│  │  │  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘  │   │   │
│  │  └──────────────────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌──────────────────────┐  ┌──────────────────────┐                        │
│  │    Channel Layer     │  │     Agent Layer      │                        │
│  │  ┌────────────────┐  │  │  ┌────────────────┐  │                        │
│  │  │ Telegram       │  │  │  │ LLM Provider   │  │                        │
│  │  │ WhatsApp       │  │  │  │ (Anthropic/    │  │                        │
│  │  │ Discord        │  │  │  │  OpenAI/etc)   │  │                        │
│  │  │ Signal         │  │  │  └────────────────┘  │                        │
│  │  │ iMessage       │  │  │  ┌────────────────┐  │                        │
│  │  │ WebChat        │  │  │  │ Tool Executor  │  │                        │
│  │  │ (Plugins...)   │  │  │  │ (exec/read/    │  │                        │
│  │  └────────────────┘  │  │  │  write/browse) │  │                        │
│  └──────────────────────┘  │  └────────────────┘  │                        │
│                            └──────────────────────┘                        │
│                                                                             │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────┐  │
│  │ Config Manager   │  │ Cron Scheduler   │  │ Plugin System            │  │
│  │ (JSON + Schema)  │  │ (robfig/cron)    │  │ (Go plugins / WASM?)     │  │
│  └──────────────────┘  └──────────────────┘  └──────────────────────────┘  │
│                                                                             │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────┐  │
│  │ Memory/Search    │  │ Browser Control  │  │ Media Processing         │  │
│  │ (Embeddings)     │  │ (chromedp/rod)   │  │ (ffmpeg bindings)        │  │
│  └──────────────────┘  └──────────────────┘  └──────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

## 目录结构

```
openclaw-go/
├── cmd/
│   └── openclaw/           # 主入口
│       └── main.go
├── internal/
│   ├── gateway/            # Gateway 服务器核心
│   │   ├── server.go       # WebSocket + HTTP 服务器
│   │   ├── protocol/       # WebSocket 协议定义
│   │   │   ├── frames.go   # req/res/event frames
│   │   │   ├── methods.go  # RPC 方法注册
│   │   │   └── schema.go   # JSON Schema 定义
│   │   ├── sessions/       # 会话管理
│   │   ├── handlers/       # RPC 方法处理器
│   │   └── bridge/         # Agent 桥接
│   ├── channels/           # 消息渠道
│   │   ├── interface.go    # Channel 接口定义
│   │   ├── registry.go     # Channel 注册表
│   │   ├── telegram/
│   │   ├── whatsapp/
│   │   ├── discord/
│   │   ├── signal/
│   │   ├── imessage/
│   │   └── webchat/
│   ├── agents/             # Agent / LLM 集成
│   │   ├── provider.go     # Provider 接口
│   │   ├── anthropic/
│   │   ├── openai/
│   │   ├── tools/          # Tool 定义和执行
│   │   │   ├── exec.go
│   │   │   ├── read.go
│   │   │   ├── write.go
│   │   │   ├── edit.go
│   │   │   ├── browser.go
│   │   │   ├── web.go
│   │   │   ├── memory.go
│   │   │   ├── cron.go
│   │   │   ├── message.go
│   │   │   └── ...
│   │   └── streaming/      # 流式响应处理
│   ├── config/             # 配置管理
│   │   ├── schema.go       # 配置 Schema
│   │   ├── loader.go       # 配置加载
│   │   ├── validate.go     # 配置校验
│   │   └── watch.go        # 热重载
│   ├── cron/               # 定时任务
│   ├── memory/             # 记忆/向量搜索
│   ├── browser/            # 浏览器控制
│   ├── media/              # 媒体处理
│   ├── plugins/            # 插件系统
│   ├── skills/             # Skills 加载
│   └── utils/              # 工具函数
├── pkg/
│   └── protocol/           # 可导出的协议定义 (供客户端使用)
├── web/                    # Control UI 静态文件
├── docs/                   # 文档
├── configs/                # 示例配置
├── scripts/                # 构建脚本
├── go.mod
├── go.sum
└── Makefile
```

## 技术选型

| 组件 | 选择 | 备选 |
|------|------|------|
| CLI | `cobra` | `urfave/cli/v2` |
| HTTP | `gin` | `chi`, `fiber` |
| WebSocket | `gorilla/websocket` | `nhooyr.io/websocket` |
| Config | `viper` + `fsnotify` | `koanf` |
| JSON Schema | `santhosh-tekuri/jsonschema` | `xeipuuv/gojsonschema` |
| Telegram | `go-telegram-bot-api` | `telebot` |
| WhatsApp | `go-whatsapp` (Baileys port) | `whatsmeow` |
| Discord | `discordgo` | `arikawa` |
| LLM | `sashabaranov/go-openai` | 自己封装 |
| Cron | `robfig/cron/v3` | - |
| Browser | `chromedp` | `rod` |
| SQLite | `modernc.org/sqlite` | `mattn/go-sqlite3` |
| Embedding DB | `qdrant` client | `milvus`, 本地向量 |
| PTY | `creack/pty` | - |
| Logging | `zerolog` | `zap`, `slog` |

## 开发路线图

### Phase 1: 基础框架 (2-3 周)
- [ ] 项目骨架和 CLI 框架
- [ ] 配置加载和校验
- [ ] Gateway WebSocket 服务器
- [ ] 基础协议实现 (hello, req/res/event frames)
- [ ] 单元测试框架

### Phase 2: Agent 集成 (2-3 周)
- [ ] LLM Provider 抽象 (Anthropic Claude)
- [ ] 流式响应处理
- [ ] 基础 Tools (exec, read, write, edit)
- [ ] 会话管理

### Phase 3: 首个渠道 (1-2 周)
- [ ] Telegram Bot 集成
- [ ] 消息收发完整流程
- [ ] WebChat (HTTP SSE)

### Phase 4: 完整 Tools (2-3 周)
- [ ] Browser 控制 (chromedp)
- [ ] Web Search / Fetch
- [ ] Memory 语义搜索
- [ ] Cron 定时任务
- [ ] Message 工具

### Phase 5: 更多渠道 (3-4 周)
- [ ] Discord
- [ ] WhatsApp (whatsmeow)
- [ ] Signal (signal-cli REST)
- [ ] iMessage (macOS only)

### Phase 6: 高级功能 (2-3 周)
- [ ] 插件系统
- [ ] Skills 加载
- [ ] Node 配对
- [ ] Canvas
- [ ] Control UI

### Phase 7: 完善和优化 (ongoing)
- [ ] 性能优化
- [ ] 错误处理完善
- [ ] 文档完善
- [ ] 兼容性测试

## 协议版本

与 OpenClaw TypeScript 版本保持协议兼容: `PROTOCOL_VERSION = 3`

## 构建

```bash
# 开发构建
make build

# 发布构建 (多平台)
make release

# 运行测试
make test

# 运行 Gateway
./bin/openclaw gateway
```

## License

MIT
