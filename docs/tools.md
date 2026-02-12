# Tools

OpenClaw provides a comprehensive set of tools for AI agents to interact with the system.

## Core Tools

### read

Read file contents.

```json
{
  "path": "file.txt",
  "offset": 1,
  "limit": 100
}
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | File path (relative or absolute) |
| `offset` | int | Starting line number (1-indexed) |
| `limit` | int | Maximum lines to read |

### write

Write content to a file.

```json
{
  "path": "file.txt",
  "content": "Hello, World!"
}
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | File path |
| `content` | string | Content to write |

### edit

Replace exact text in a file.

```json
{
  "path": "file.txt",
  "oldText": "old content",
  "newText": "new content"
}
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | File path |
| `oldText` | string | Exact text to find |
| `newText` | string | Replacement text |

### exec

Execute shell commands.

```json
{
  "command": "ls -la",
  "workdir": "/path/to/dir",
  "timeout": 30,
  "background": false,
  "pty": false
}
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `command` | string | Shell command |
| `workdir` | string | Working directory |
| `timeout` | int | Timeout in seconds |
| `background` | bool | Run in background |
| `pty` | bool | Use pseudo-terminal |

### process

Manage background processes.

```json
{
  "action": "list|poll|log|write|send-keys|kill",
  "sessionId": "session-1"
}
```

| Action | Description |
|--------|-------------|
| `list` | List all sessions |
| `poll` | Get session status |
| `log` | Get session output |
| `write` | Write to stdin |
| `send-keys` | Send key sequences |
| `kill` | Terminate session |

## Web Tools

### web_search

Search the web using Brave Search API.

```json
{
  "query": "OpenClaw documentation",
  "count": 5,
  "freshness": "pw"
}
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `query` | string | Search query |
| `count` | int | Results count (1-10) |
| `country` | string | 2-letter country code |
| `freshness` | string | Time filter: pd/pw/pm/py |

### web_fetch

Fetch and extract content from a URL.

```json
{
  "url": "https://example.com",
  "extractMode": "markdown",
  "maxChars": 50000
}
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string | URL to fetch |
| `extractMode` | string | "markdown" or "text" |
| `maxChars` | int | Maximum characters |

### browser

Control web browser.

```json
{
  "action": "navigate|snapshot|screenshot|act",
  "url": "https://example.com"
}
```

| Action | Description |
|--------|-------------|
| `start` | Start browser |
| `stop` | Stop browser |
| `navigate` | Go to URL |
| `snapshot` | Get page content |
| `screenshot` | Take screenshot |
| `act` | Interact with elements |

## Memory Tools

### memory_search

Search memory files.

```json
{
  "query": "previous decision",
  "maxResults": 10
}
```

### memory_get

Read memory file snippet.

```json
{
  "path": "MEMORY.md",
  "from": 10,
  "lines": 20
}
```

## System Tools

### cron

Manage scheduled jobs.

```json
{
  "action": "list|add|remove|run",
  "job": {
    "name": "Daily check",
    "schedule": { "kind": "cron", "expr": "0 9 * * *" },
    "payload": { "kind": "systemEvent", "text": "Morning check" }
  }
}
```

### message

Send messages across channels.

```json
{
  "action": "send",
  "channel": "telegram",
  "target": "123456789",
  "message": "Hello!"
}
```

### canvas

Control node canvases.

```json
{
  "action": "present|hide|snapshot",
  "url": "http://localhost:3000",
  "target": "main"
}
```

### nodes

Manage paired devices.

```json
{
  "action": "status|notify|camera_snap|location_get",
  "node": "iphone"
}
```

## Tool Configuration

Tools can be configured in the config file:

```json
{
  "tools": {
    "exec": {
      "allowlist": ["git", "npm", "node"],
      "denylist": ["rm -rf", "sudo"],
      "defaultTimeout": 30
    },
    "browser": {
      "headless": true,
      "viewport": { "width": 1920, "height": 1080 }
    },
    "web_search": {
      "defaultCount": 5
    }
  }
}
```
