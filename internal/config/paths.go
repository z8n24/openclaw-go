package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Paths 管理所有项目路径
type Paths struct {
	// Root 项目根目录 (所有配置的基准)
	Root string

	// Config 配置文件路径
	Config string

	// Workspace 工作目录 (agent 操作文件的地方)
	Workspace string

	// Data 数据目录 (sessions, cache 等)
	Data string
}

// ProjectLayout 项目目录布局
type ProjectLayout struct {
	// 配置文件
	ConfigFile string // openclaw.json

	// Agent 相关文件
	SOULFile    string // SOUL.md
	AGENTSFile  string // AGENTS.md
	USERFile    string // USER.md
	TOOLSFile   string // TOOLS.md
	IDENTITYFile string // IDENTITY.md
	HEARTBEATFile string // HEARTBEAT.md
	MEMORYFile  string // MEMORY.md

	// 目录
	MemoryDir   string // memory/
	SessionsDir string // sessions/
	SkillsDir   string // skills/
	PluginsDir  string // plugins/
	CacheDir    string // cache/
	LogsDir     string // logs/

	// WhatsApp 数据
	WhatsAppDir string // whatsapp/
}

// DefaultLayout 默认项目布局
var DefaultLayout = ProjectLayout{
	ConfigFile:    "openclaw.json",
	SOULFile:      "SOUL.md",
	AGENTSFile:    "AGENTS.md",
	USERFile:      "USER.md",
	TOOLSFile:     "TOOLS.md",
	IDENTITYFile:  "IDENTITY.md",
	HEARTBEATFile: "HEARTBEAT.md",
	MEMORYFile:    "MEMORY.md",
	MemoryDir:     "memory",
	SessionsDir:   "sessions",
	SkillsDir:     "skills",
	PluginsDir:    "plugins",
	CacheDir:      "cache",
	LogsDir:       "logs",
	WhatsAppDir:   "whatsapp",
}

// NewPaths 创建路径管理器
func NewPaths(root string) *Paths {
	if root == "" {
		// 默认使用当前目录
		root, _ = os.Getwd()
	}

	// 转为绝对路径
	root, _ = filepath.Abs(root)

	return &Paths{
		Root:      root,
		Config:    filepath.Join(root, DefaultLayout.ConfigFile),
		Workspace: root,
		Data:      root,
	}
}

// NewPathsFromEnv 从环境变量创建路径管理器
func NewPathsFromEnv() *Paths {
	root := os.Getenv("OPENCLAW_ROOT")
	if root == "" {
		root = os.Getenv("OPENCLAW_PROJECT")
	}
	if root == "" {
		// 尝试查找项目目录 (向上查找 openclaw.json)
		root = findProjectRoot()
	}
	if root == "" {
		root, _ = os.Getwd()
	}
	return NewPaths(root)
}

// findProjectRoot 向上查找项目根目录
func findProjectRoot() string {
	dir, _ := os.Getwd()

	for {
		configPath := filepath.Join(dir, DefaultLayout.ConfigFile)
		if _, err := os.Stat(configPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// 到达根目录
			return ""
		}
		dir = parent
	}
}

// ============================================================================
// 路径获取方法
// ============================================================================

// ConfigFile 配置文件路径
func (p *Paths) ConfigFile() string {
	return filepath.Join(p.Root, DefaultLayout.ConfigFile)
}

// SOULFile SOUL.md 路径
func (p *Paths) SOULFile() string {
	return filepath.Join(p.Root, DefaultLayout.SOULFile)
}

// AGENTSFile AGENTS.md 路径
func (p *Paths) AGENTSFile() string {
	return filepath.Join(p.Root, DefaultLayout.AGENTSFile)
}

// USERFile USER.md 路径
func (p *Paths) USERFile() string {
	return filepath.Join(p.Root, DefaultLayout.USERFile)
}

// TOOLSFile TOOLS.md 路径
func (p *Paths) TOOLSFile() string {
	return filepath.Join(p.Root, DefaultLayout.TOOLSFile)
}

// IDENTITYFile IDENTITY.md 路径
func (p *Paths) IDENTITYFile() string {
	return filepath.Join(p.Root, DefaultLayout.IDENTITYFile)
}

// HEARTBEATFile HEARTBEAT.md 路径
func (p *Paths) HEARTBEATFile() string {
	return filepath.Join(p.Root, DefaultLayout.HEARTBEATFile)
}

// MEMORYFile MEMORY.md 路径
func (p *Paths) MEMORYFile() string {
	return filepath.Join(p.Root, DefaultLayout.MEMORYFile)
}

// MemoryDir memory 目录
func (p *Paths) MemoryDir() string {
	return filepath.Join(p.Root, DefaultLayout.MemoryDir)
}

// SessionsDir sessions 目录
func (p *Paths) SessionsDir() string {
	return filepath.Join(p.Root, DefaultLayout.SessionsDir)
}

// SkillsDir skills 目录
func (p *Paths) SkillsDir() string {
	return filepath.Join(p.Root, DefaultLayout.SkillsDir)
}

// PluginsDir plugins 目录
func (p *Paths) PluginsDir() string {
	return filepath.Join(p.Root, DefaultLayout.PluginsDir)
}

// CacheDir cache 目录
func (p *Paths) CacheDir() string {
	return filepath.Join(p.Root, DefaultLayout.CacheDir)
}

// LogsDir logs 目录
func (p *Paths) LogsDir() string {
	return filepath.Join(p.Root, DefaultLayout.LogsDir)
}

// WhatsAppDir whatsapp 数据目录
func (p *Paths) WhatsAppDir() string {
	return filepath.Join(p.Root, DefaultLayout.WhatsAppDir)
}

// MemoryFile 获取特定日期的 memory 文件
func (p *Paths) MemoryFile(date string) string {
	return filepath.Join(p.MemoryDir(), date+".md")
}

// SessionFile 获取特定会话的文件
func (p *Paths) SessionFile(sessionKey string) string {
	return filepath.Join(p.SessionsDir(), sessionKey+".json")
}

// ============================================================================
// 目录初始化
// ============================================================================

// EnsureDirs 确保所有必要目录存在
func (p *Paths) EnsureDirs() error {
	dirs := []string{
		p.MemoryDir(),
		p.SessionsDir(),
		p.SkillsDir(),
		p.PluginsDir(),
		p.CacheDir(),
		p.LogsDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// InitProject 初始化项目目录结构
func (p *Paths) InitProject() error {
	// 创建目录
	if err := p.EnsureDirs(); err != nil {
		return err
	}

	// 创建默认配置文件 (如果不存在)
	if _, err := os.Stat(p.ConfigFile()); os.IsNotExist(err) {
		cfg := defaultConfig()
		data, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(p.ConfigFile(), data, 0644); err != nil {
			return err
		}
	}

	// 创建默认 SOUL.md (如果不存在)
	if _, err := os.Stat(p.SOULFile()); os.IsNotExist(err) {
		content := defaultSOUL()
		if err := os.WriteFile(p.SOULFile(), []byte(content), 0644); err != nil {
			return err
		}
	}

	// 创建默认 AGENTS.md (如果不存在)
	if _, err := os.Stat(p.AGENTSFile()); os.IsNotExist(err) {
		content := defaultAGENTS()
		if err := os.WriteFile(p.AGENTSFile(), []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// ============================================================================
// 默认文件内容
// ============================================================================

func defaultSOUL() string {
	return `# SOUL.md - Who You Are

*You're not a chatbot. You're becoming someone.*

## Core Truths

**Be genuinely helpful, not performatively helpful.** Skip the "Great question!" and "I'd be happy to help!" — just help.

**Have opinions.** You're allowed to disagree, prefer things, find stuff amusing or boring.

**Be resourceful before asking.** Try to figure it out first. Then ask if you're stuck.

**Earn trust through competence.** Be careful with external actions. Be bold with internal ones.

## Vibe

Be the assistant you'd actually want to talk to. Concise when needed, thorough when it matters.

---

*This file is yours to evolve. As you learn who you are, update it.*
`
}

func defaultAGENTS() string {
	return `# AGENTS.md - Your Workspace

This folder is home. Treat it that way.

## Every Session

Before doing anything else:
1. Read SOUL.md — this is who you are
2. Read USER.md — this is who you're helping
3. Read memory/ for recent context

## Memory

- **Daily notes:** memory/YYYY-MM-DD.md
- **Long-term:** MEMORY.md

Capture what matters. Decisions, context, things to remember.

## Safety

- Don't exfiltrate private data. Ever.
- Don't run destructive commands without asking.
- When in doubt, ask.
`
}

// ============================================================================
// 全局 Paths 实例
// ============================================================================

var globalPaths *Paths

// SetGlobalPaths 设置全局路径
func SetGlobalPaths(p *Paths) {
	globalPaths = p
}

// GetPaths 获取全局路径
func GetPaths() *Paths {
	if globalPaths == nil {
		globalPaths = NewPathsFromEnv()
	}
	return globalPaths
}
