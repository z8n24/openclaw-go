package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// BuildSystemPrompt 构建完整的 system prompt
func BuildSystemPrompt(workspace string, tools []Tool) string {
	var sb strings.Builder
	
	sb.WriteString("You are a personal AI assistant with access to tools.\n\n")
	
	// Tools 说明
	sb.WriteString("## Tools\n\n")
	sb.WriteString("You can invoke tools by using function calls. Available tools:\n\n")
	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", t.Name, t.Description))
	}
	sb.WriteString("\n")
	
	// Workspace
	sb.WriteString("## Workspace\n\n")
	sb.WriteString(fmt.Sprintf("Your working directory is: %s\n", workspace))
	sb.WriteString("Treat this directory as your workspace for file operations.\n\n")
	
	// 读取 workspace 文件
	workspaceFiles := []string{"AGENTS.md", "SOUL.md", "USER.md", "TOOLS.md"}
	for _, filename := range workspaceFiles {
		path := filepath.Join(workspace, filename)
		content, err := os.ReadFile(path)
		if err == nil && len(content) > 0 {
			sb.WriteString(fmt.Sprintf("## %s\n\n", filename))
			sb.WriteString(string(content))
			sb.WriteString("\n\n")
		}
	}
	
	// Runtime 信息
	sb.WriteString("## Runtime\n\n")
	sb.WriteString(fmt.Sprintf("- OS: %s/%s\n", runtime.GOOS, runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("- Time: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- Timezone: %s\n", time.Now().Location().String()))
	sb.WriteString("\n")
	
	// 行为规则
	sb.WriteString(`## Behavior

- Be concise and helpful
- Use tools when needed - don't just describe what you would do
- For file operations, actually use read/write/edit tools
- For shell commands, use exec tool
- When making function calls, provide all required parameters
- If a task requires multiple steps, execute them in sequence

## Tool Call Style

- Don't narrate routine tool calls, just call them
- Narrate only for complex operations or when it helps understanding
- After tool results, continue with the task or respond to the user
`)
	
	return sb.String()
}
