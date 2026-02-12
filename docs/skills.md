# Skills

Skills extend OpenClaw with additional tools and capabilities.

## Overview

A skill is a directory containing:
- `SKILL.md` - Skill metadata and tool definitions
- Scripts, binaries, or configuration files
- Optional assets

## Installing Skills

### From GitHub

```bash
openclaw skills install github.com/user/skill-repo
openclaw skills install user/skill-repo  # shorthand
```

### From Local Path

```bash
openclaw skills install ./my-skill
openclaw skills install ~/skills/weather
```

## Managing Skills

```bash
# List installed skills
openclaw skills list

# Enable/disable
openclaw skills enable weather
openclaw skills disable weather

# Update
openclaw skills update weather

# Uninstall
openclaw skills uninstall weather
```

## Creating Skills

### SKILL.md Format

```markdown
# My Awesome Skill

A brief description of what this skill does.

## Metadata

- **Version**: 1.0.0
- **Author**: Your Name
- **ID**: my-awesome-skill

## Permissions

- network
- filesystem
- exec

## Tools

### tool_name

Description of what this tool does.

- **Binary**: ./bin/tool
- **Description**: Detailed description
- **Parameters**:
  - `param1` (string): Description
  - `param2` (number): Description

### another_tool

Another tool description.

## Configuration

- **api_key**: Your API key for the service
```

### Skill Structure

```
my-skill/
├── SKILL.md           # Required: skill metadata
├── bin/               # Compiled binaries
│   └── tool
├── scripts/           # Script files
│   └── helper.sh
├── config/            # Configuration templates
│   └── default.json
└── README.md          # Additional documentation
```

### Tool Implementation

Tools can be implemented as:

1. **Binary executables** - Fastest, any language
2. **Shell scripts** - Simplest for basic tasks
3. **Python scripts** - Good for complex logic

#### Binary Tool

```go
// tool.go
package main

import (
    "encoding/json"
    "fmt"
    "os"
)

type Input struct {
    Query string `json:"query"`
}

type Output struct {
    Result string `json:"result"`
}

func main() {
    var input Input
    json.NewDecoder(os.Stdin).Decode(&input)
    
    output := Output{Result: "Processed: " + input.Query}
    json.NewEncoder(os.Stdout).Encode(output)
}
```

Build: `go build -o bin/tool tool.go`

#### Shell Script Tool

```bash
#!/bin/bash
# tool.sh

# Read JSON input
INPUT=$(cat)
QUERY=$(echo "$INPUT" | jq -r '.query')

# Process
RESULT="Processed: $QUERY"

# Output JSON
echo "{\"result\": \"$RESULT\"}"
```

### Permissions

| Permission | Description |
|------------|-------------|
| `network` | Make HTTP requests |
| `filesystem` | Read/write files |
| `exec` | Execute commands |
| `browser` | Control browser |
| `camera` | Access camera |
| `location` | Access location |

## Publishing Skills

1. Create a GitHub repository
2. Add SKILL.md with proper metadata
3. Tag a release
4. Share: `openclaw skills install your-username/skill-name`

### Best Practices

- Include a detailed README
- Provide example usage
- Document all parameters
- Use semantic versioning
- Include tests if possible
- Minimize permissions

## Built-in Skills

OpenClaw includes some built-in skills:

| Skill | Description |
|-------|-------------|
| `weather` | Get weather forecasts |
| `video-frames` | Extract frames from videos |
| `coding-agent` | Run coding assistants |

These are located in the OpenClaw installation directory.
