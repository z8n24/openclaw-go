# Configuration

OpenClaw Go uses a JSON configuration file located at `~/.openclaw/openclaw.json`.

## Quick Setup

```bash
# Create default config
openclaw init

# Edit config
openclaw config edit

# Validate config
openclaw config validate
```

## Configuration File Structure

```json
{
  "gateway": {
    "bind": "127.0.0.1",
    "port": 18789,
    "token": "your-secure-token"
  },
  "agent": {
    "defaultModel": "anthropic/claude-sonnet-4",
    "workspace": "~/.openclaw/workspace",
    "systemPrompt": "",
    "maxTokens": 8192,
    "temperature": 0.7
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "botToken": "YOUR_BOT_TOKEN",
      "allowFrom": ["username", "123456789"]
    },
    "discord": {
      "enabled": false,
      "token": "YOUR_DISCORD_TOKEN",
      "applicationId": "YOUR_APP_ID"
    },
    "webchat": {
      "enabled": true
    }
  },
  "tools": {
    "exec": {
      "allowlist": [],
      "denylist": ["rm -rf /", "sudo"]
    },
    "browser": {
      "headless": true
    }
  },
  "skills": {
    "enabled": true,
    "directories": ["~/.openclaw/skills"]
  }
}
```

## Section Reference

### Gateway

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `bind` | string | `127.0.0.1` | IP address to bind to |
| `port` | int | `18789` | Port number |
| `token` | string | - | Authentication token (required) |

### Agent

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `defaultModel` | string | `anthropic/claude-sonnet-4` | Default LLM model |
| `workspace` | string | `~/.openclaw/workspace` | Working directory |
| `maxTokens` | int | `8192` | Maximum output tokens |
| `temperature` | float | `0.7` | Sampling temperature |

### Channels

See [Channels Documentation](./channels.md) for channel-specific configuration.

### Tools

| Field | Type | Description |
|-------|------|-------------|
| `exec.allowlist` | []string | Only allow these command prefixes |
| `exec.denylist` | []string | Block these commands |
| `browser.headless` | bool | Run browser in headless mode |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENCLAW_CONFIG` | Custom config file path |
| `OPENCLAW_TOKEN` | Gateway token (overrides config) |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `DEEPSEEK_API_KEY` | DeepSeek API key |
| `BRAVE_API_KEY` | Brave Search API key |

## Model Aliases

Short aliases for common models:

| Alias | Model |
|-------|-------|
| `claude` / `sonnet` | `anthropic/claude-sonnet-4` |
| `opus` | `anthropic/claude-opus-4-5` |
| `haiku` | `anthropic/claude-3-5-haiku` |
| `gpt4` / `gpt4o` | `openai/gpt-4o` |
| `deepseek` | `deepseek/deepseek-chat` |
| `gemini` | `google/gemini-2.0-flash` |
