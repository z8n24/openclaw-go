# Changelog

All notable changes to OpenClaw Go will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial Go implementation of OpenClaw
- CLI framework with cobra
- Gateway server (WebSocket + HTTP)
- LLM providers: Anthropic, OpenAI, DeepSeek, Google Gemini, OpenRouter
- Session management with compaction
- All core tools: read, write, edit, exec, process, browser, web_search, web_fetch
- Memory tools: memory_search, memory_get
- System tools: cron, message, canvas, nodes
- Channels: Telegram, WebChat (SSE), Discord, WhatsApp, Signal, iMessage
- Channel manager and message router
- Skills system with GitHub/local installation
- Cron scheduler with persistence
- Control UI (web interface)
- Comprehensive documentation
- 163+ unit tests
- Benchmark tests

### Changed
- Switched from Node.js to Go for better performance
- Single binary deployment (no runtime dependencies)

## [0.1.0] - 2026-02-12

### Added
- Initial release
- Protocol version 3 compatibility with TypeScript version

---

## Version History

| Version | Date | Highlights |
|---------|------|------------|
| 0.1.0 | 2026-02-12 | Initial release |
