# Channels

OpenClaw supports multiple messaging channels. Each channel bridges a messaging platform to your AI agent.

## Supported Channels

| Channel | Status | Notes |
|---------|--------|-------|
| Telegram | ✅ Full | Bot API, recommended for getting started |
| WebChat | ✅ Full | Built-in web interface |
| Discord | ✅ Full | Bot with slash commands |
| WhatsApp | 🔄 Beta | Personal number via whatsmeow |
| Signal | 🔄 Beta | Requires signal-cli |
| iMessage | 🔄 Beta | macOS only |
| Slack | 🔄 Beta | Socket Mode |

## Telegram

The easiest way to get started.

### Setup

1. Create a bot with [@BotFather](https://t.me/BotFather)
2. Get your bot token
3. Configure:

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "botToken": "123456:ABC-DEF...",
      "allowFrom": ["your_username"]
    }
  }
}
```

### Options

| Field | Type | Description |
|-------|------|-------------|
| `botToken` | string | Bot token from BotFather |
| `allowFrom` | []string | Allowed usernames or user IDs |
| `allowGroups` | bool | Allow messages from groups |

## Discord

### Setup

1. Create application at [Discord Developer Portal](https://discord.com/developers)
2. Create a bot and get the token
3. Enable required intents (Message Content)
4. Generate invite URL with permissions
5. Configure:

```json
{
  "channels": {
    "discord": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "applicationId": "YOUR_APP_ID",
      "allowedGuilds": ["guild_id"]
    }
  }
}
```

## WebChat

Built-in web chat interface with SSE streaming.

### Setup

WebChat is enabled by default. Access at `http://localhost:18789/ui`.

```json
{
  "channels": {
    "webchat": {
      "enabled": true
    }
  }
}
```

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/webchat/message` | POST | Send a message |
| `/webchat/events` | GET (SSE) | Stream responses |
| `/webchat/status` | GET | Channel status |

## WhatsApp

Uses [whatsmeow](https://github.com/tulir/whatsmeow) for WhatsApp Web connection.

### Setup

1. Configure:

```json
{
  "channels": {
    "whatsapp": {
      "enabled": true,
      "storeDir": "~/.openclaw/whatsapp"
    }
  }
}
```

2. Start gateway and scan QR code

⚠️ **Warning**: Use a separate phone number. Your account may be banned.

## Signal

Requires [signal-cli](https://github.com/AsamK/signal-cli) in REST API mode.

### Setup

1. Install signal-cli and link device
2. Start signal-cli REST API
3. Configure:

```json
{
  "channels": {
    "signal": {
      "enabled": true,
      "restUrl": "http://localhost:8080",
      "phoneNumber": "+1234567890"
    }
  }
}
```

## iMessage (macOS only)

Uses macOS Messages database and AppleScript.

### Setup

1. Grant Terminal/app Full Disk Access
2. Configure:

```json
{
  "channels": {
    "imessage": {
      "enabled": true,
      "allowFrom": ["+1234567890", "email@example.com"]
    }
  }
}
```

## Multiple Channels

You can enable multiple channels simultaneously:

```json
{
  "channels": {
    "telegram": { "enabled": true, "botToken": "..." },
    "discord": { "enabled": true, "token": "..." },
    "webchat": { "enabled": true }
  }
}
```

Each channel maintains separate sessions, but you can reference conversations across channels using session labels.
