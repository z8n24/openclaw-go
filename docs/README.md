# OpenClaw Go Documentation

OpenClaw Go is a complete Go rewrite of OpenClaw, bridging messaging platforms to AI agents.

## Quick Start

### Installation

```bash
# Build from source
git clone https://github.com/openclaw/openclaw-go
cd openclaw-go
make build

# Or install directly
go install github.com/openclaw/openclaw-go/cmd/openclaw@latest
```

### Basic Usage

```bash
# Initialize configuration
openclaw init

# Start the gateway
openclaw gateway

# Check status
openclaw status

# Interactive chat (for testing)
openclaw chat -m anthropic/claude-sonnet-4
```

## Documentation

- [Configuration](./configuration.md) - Config file format and options
- [Channels](./channels.md) - Messaging platform setup
- [Tools](./tools.md) - Available tools and their parameters
- [Skills](./skills.md) - Installing and creating skills
- [API Reference](./api.md) - WebSocket and HTTP API

## Architecture

```
openclaw-go/
├── cmd/openclaw/          # CLI entry point
├── internal/
│   ├── gateway/           # WebSocket + HTTP server
│   ├── channels/          # Messaging channels (Telegram, Discord, etc.)
│   ├── agents/            # LLM providers and tools
│   ├── sessions/          # Session management
│   ├── cron/              # Scheduled tasks
│   ├── skills/            # Skill loading
│   └── config/            # Configuration
├── web/                   # Control UI static files
└── docs/                  # Documentation
```

## License

MIT
