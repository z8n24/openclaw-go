package skills

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Skill 表示一个技能
type Skill struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version,omitempty"`
	Author      string            `json:"author,omitempty"`
	Tools       []ToolSpec        `json:"tools,omitempty"`
	Permissions []string          `json:"permissions,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
	Path        string            `json:"path"`
	Enabled     bool              `json:"enabled"`
	LoadedAt    time.Time         `json:"loadedAt"`
	Source      SkillSource       `json:"source"`
}

// ToolSpec 工具定义
type ToolSpec struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Binary      string                 `json:"binary,omitempty"`
	Command     string                 `json:"command,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// SkillSource 技能来源
type SkillSource struct {
	Kind   string `json:"kind"` // "local" | "github" | "builtin"
	URL    string `json:"url,omitempty"`
	Ref    string `json:"ref,omitempty"` // git ref
	Commit string `json:"commit,omitempty"`
}

// ParseSKILLMD 解析 SKILL.md 文件
func ParseSKILLMD(path string) (*Skill, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open skill file: %w", err)
	}
	defer file.Close()

	skill := &Skill{
		Path:     filepath.Dir(path),
		Enabled:  true,
		LoadedAt: time.Now(),
		Source:   SkillSource{Kind: "local"},
		Config:   make(map[string]string),
	}

	scanner := bufio.NewScanner(file)
	var currentSection string
	var currentTool *ToolSpec
	var toolParamSection bool
	
	// 正则表达式
	headerRe := regexp.MustCompile(`^#+\s+(.+)`)
	kvRe := regexp.MustCompile(`^\s*[-*]\s*\*\*([^*]+)\*\*:\s*(.+)`)
	kvRe2 := regexp.MustCompile(`^\s*[-*]\s*([^:]+):\s*(.+)`)
	toolHeaderRe := regexp.MustCompile(`^###\s+(\w+)`)
	codeBlockRe := regexp.MustCompile("^```")

	inCodeBlock := false
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// 处理代码块
		if codeBlockRe.MatchString(line) {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		// 检查标题
		if m := headerRe.FindStringSubmatch(line); m != nil {
			header := strings.TrimSpace(m[1])
			headerLower := strings.ToLower(header)

			// 顶级标题是技能名称
			if strings.HasPrefix(line, "# ") && skill.Name == "" {
				skill.Name = header
				skill.ID = sanitizeID(header)
				continue
			}

			// 二级标题是章节
			if strings.HasPrefix(line, "## ") {
				currentSection = headerLower
				toolParamSection = false
				continue
			}

			// 三级标题可能是工具定义
			if strings.HasPrefix(line, "### ") {
				if currentSection == "tools" || currentSection == "tool definitions" {
					if m := toolHeaderRe.FindStringSubmatch(line); m != nil {
						tool := ToolSpec{Name: m[1]}
						skill.Tools = append(skill.Tools, tool)
						currentTool = &skill.Tools[len(skill.Tools)-1]
					}
				}
				continue
			}
			continue
		}

		// 解析键值对
		var key, value string
		if m := kvRe.FindStringSubmatch(line); m != nil {
			key = strings.ToLower(strings.TrimSpace(m[1]))
			value = strings.TrimSpace(m[2])
		} else if m := kvRe2.FindStringSubmatch(line); m != nil {
			key = strings.ToLower(strings.TrimSpace(m[1]))
			value = strings.TrimSpace(m[2])
		}

		if key != "" && value != "" {
			switch currentSection {
			case "metadata", "info", "":
				switch key {
				case "version":
					skill.Version = value
				case "author":
					skill.Author = value
				case "description":
					skill.Description = value
				case "id":
					skill.ID = value
				}
			case "permissions":
				skill.Permissions = append(skill.Permissions, value)
			case "config", "configuration":
				skill.Config[key] = value
			case "tools", "tool definitions":
				if currentTool != nil {
					switch key {
					case "description":
						currentTool.Description = value
					case "binary":
						currentTool.Binary = value
					case "command":
						currentTool.Command = value
					}
				}
			}
		}

		// 检查工具参数节
		if strings.Contains(strings.ToLower(line), "parameters") && currentTool != nil {
			toolParamSection = true
		}

		// 解析 description 段落
		if currentSection == "description" && strings.TrimSpace(line) != "" && skill.Description == "" {
			skill.Description = strings.TrimSpace(line)
		}

		_ = toolParamSection // 将来扩展参数解析
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan skill file: %w", err)
	}

	// 验证必要字段
	if skill.Name == "" {
		return nil, fmt.Errorf("skill name not found in %s", path)
	}
	if skill.ID == "" {
		skill.ID = sanitizeID(skill.Name)
	}

	return skill, nil
}

// sanitizeID 将名称转换为合法的ID
func sanitizeID(name string) string {
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	id = regexp.MustCompile(`[^a-z0-9\-]`).ReplaceAllString(id, "")
	return id
}

// Validate 验证技能配置
func (s *Skill) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("skill id is required")
	}
	if s.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if s.Path == "" {
		return fmt.Errorf("skill path is required")
	}
	
	// 检查路径存在
	if _, err := os.Stat(s.Path); err != nil {
		return fmt.Errorf("skill path not found: %s", s.Path)
	}
	
	return nil
}

// GetBinary 获取工具的可执行文件路径
func (s *Skill) GetBinary(toolName string) (string, error) {
	for _, tool := range s.Tools {
		if tool.Name == toolName {
			if tool.Binary != "" {
				binPath := filepath.Join(s.Path, tool.Binary)
				if _, err := os.Stat(binPath); err == nil {
					return binPath, nil
				}
				// 尝试 bin 目录
				binPath = filepath.Join(s.Path, "bin", tool.Binary)
				if _, err := os.Stat(binPath); err == nil {
					return binPath, nil
				}
			}
			return "", fmt.Errorf("binary not found for tool: %s", toolName)
		}
	}
	return "", fmt.Errorf("tool not found: %s", toolName)
}
