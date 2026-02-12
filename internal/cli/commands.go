package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/z8n24/openclaw-go/internal/config"
	"github.com/z8n24/openclaw-go/internal/gateway"
)

// ============================================================================
// status 命令
// ============================================================================

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show OpenClaw status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦞 OpenClaw Go Status")
		fmt.Println(strings.Repeat("─", 40))
		fmt.Println()

		// 版本信息
		fmt.Printf("Version:  %s\n", gateway.Version)
		fmt.Printf("Protocol: 3\n")
		fmt.Println()

		// 配置文件
		cfgPath := config.GetConfigPath()
		fmt.Printf("Config:   %s\n", cfgPath)
		if _, err := os.Stat(cfgPath); err == nil {
			fmt.Printf("          (exists)\n")
		} else {
			fmt.Printf("          (not found)\n")
		}
		fmt.Println()

		// Gateway 状态
		fmt.Println("Gateway:")
		cfg, _ := config.Load()
		port := 18789
		if cfg != nil && cfg.Gateway.Port != 0 {
			port = cfg.Gateway.Port
		}

		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/health", port))
		if err != nil {
			fmt.Printf("  Status: ❌ Not running\n")
		} else {
			resp.Body.Close()
			fmt.Printf("  Status: ✅ Running on port %d\n", port)

			// 获取详细状态
			resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/status", port))
			if err == nil {
				defer resp.Body.Close()
				var status map[string]interface{}
				if json.NewDecoder(resp.Body).Decode(&status) == nil {
					if clients, ok := status["clients"].(float64); ok {
						fmt.Printf("  Clients: %d connected\n", int(clients))
					}
				}
			}
		}
		fmt.Println()

		// 工作空间
		workspace := ""
		if cfg != nil && cfg.Agent.Workspace != "" {
			workspace = cfg.Agent.Workspace
		} else {
			home, _ := os.UserHomeDir()
			workspace = filepath.Join(home, ".openclaw", "workspace")
		}
		fmt.Printf("Workspace: %s\n", workspace)
		if _, err := os.Stat(workspace); err == nil {
			fmt.Printf("           (exists)\n")
		}
		fmt.Println()

		// 模型
		if cfg != nil && cfg.Agent.DefaultModel != "" {
			fmt.Printf("Default Model: %s\n", cfg.Agent.DefaultModel)
		}

		// 渠道状态
		fmt.Println()
		fmt.Println("Channels:")
		if cfg != nil {
			if cfg.Channels.Telegram != nil && cfg.Channels.Telegram.Enabled {
				fmt.Println("  - Telegram: enabled")
			}
			if cfg.Channels.WhatsApp != nil && cfg.Channels.WhatsApp.Enabled {
				fmt.Println("  - WhatsApp: enabled")
			}
			if cfg.Channels.Discord != nil && cfg.Channels.Discord.Enabled {
				fmt.Println("  - Discord: enabled")
			}
			if cfg.Channels.Signal != nil && cfg.Channels.Signal.Enabled {
				fmt.Println("  - Signal: enabled")
			}
		}

		return nil
	},
}

// ============================================================================
// doctor 命令
// ============================================================================

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose potential issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🩺 OpenClaw Doctor")
		fmt.Println(strings.Repeat("─", 40))
		fmt.Println()

		allGood := true

		// 检查 Go 版本
		fmt.Print("Go version: ")
		goVersion := runtime.Version()
		fmt.Printf("%s ✅\n", goVersion)

		// 检查配置文件
		fmt.Print("Config file: ")
		cfgPath := config.GetConfigPath()
		if _, err := os.Stat(cfgPath); err == nil {
			fmt.Println("✅ Found")
		} else {
			fmt.Println("⚠️  Not found (using defaults)")
		}

		// 检查工作空间
		fmt.Print("Workspace: ")
		home, _ := os.UserHomeDir()
		workspace := filepath.Join(home, ".openclaw", "workspace")
		if _, err := os.Stat(workspace); err == nil {
			fmt.Println("✅ Exists")
		} else {
			fmt.Println("⚠️  Not found (will be created)")
		}

		// 检查 API Keys
		fmt.Println()
		fmt.Println("API Keys:")

		if os.Getenv("ANTHROPIC_API_KEY") != "" {
			fmt.Println("  ANTHROPIC_API_KEY: ✅ Set")
		} else {
			fmt.Println("  ANTHROPIC_API_KEY: ❌ Not set")
			allGood = false
		}

		if os.Getenv("OPENAI_API_KEY") != "" {
			fmt.Println("  OPENAI_API_KEY: ✅ Set")
		} else {
			fmt.Println("  OPENAI_API_KEY: ⚠️  Not set (optional)")
		}

		if os.Getenv("DEEPSEEK_API_KEY") != "" {
			fmt.Println("  DEEPSEEK_API_KEY: ✅ Set")
		} else {
			fmt.Println("  DEEPSEEK_API_KEY: ⚠️  Not set (optional)")
		}

		if os.Getenv("BRAVE_API_KEY") != "" {
			fmt.Println("  BRAVE_API_KEY: ✅ Set")
		} else {
			fmt.Println("  BRAVE_API_KEY: ⚠️  Not set (web_search disabled)")
		}

		// 检查外部依赖
		fmt.Println()
		fmt.Println("External tools:")

		checkCommand := func(name string, args ...string) {
			cmd := exec.Command(name, args...)
			if err := cmd.Run(); err == nil {
				fmt.Printf("  %s: ✅ Found\n", name)
			} else {
				fmt.Printf("  %s: ⚠️  Not found\n", name)
			}
		}

		checkCommand("git", "--version")
		checkCommand("curl", "--version")

		// 检查 Gateway 状态
		fmt.Println()
		fmt.Print("Gateway: ")
		cfg, _ := config.Load()
		port := 18789
		if cfg != nil && cfg.Gateway.Port != 0 {
			port = cfg.Gateway.Port
		}

		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/health", port))
		if err != nil {
			fmt.Println("⚠️  Not running")
			fmt.Println("  Run: openclaw gateway")
		} else {
			resp.Body.Close()
			fmt.Printf("✅ Running on port %d\n", port)
		}

		fmt.Println()
		if allGood {
			fmt.Println("✨ Everything looks good!")
		} else {
			fmt.Println("⚠️  Some issues found. See above for details.")
		}

		return nil
	},
}

// ============================================================================
// channels 命令
// ============================================================================

var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "Manage messaging channels",
}

var channelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		fmt.Println("Configured Channels:")
		fmt.Println()

		printChannel := func(name string, enabled bool, details string) {
			status := "❌"
			if enabled {
				status = "✅"
			}
			fmt.Printf("  %s %s", status, name)
			if details != "" {
				fmt.Printf(" - %s", details)
			}
			fmt.Println()
		}

		if cfg.Channels.Telegram != nil {
			printChannel("Telegram", cfg.Channels.Telegram.Enabled, "")
		}
		if cfg.Channels.WhatsApp != nil {
			printChannel("WhatsApp", cfg.Channels.WhatsApp.Enabled, "")
		}
		if cfg.Channels.Discord != nil {
			printChannel("Discord", cfg.Channels.Discord.Enabled, "")
		}
		if cfg.Channels.Signal != nil {
			printChannel("Signal", cfg.Channels.Signal.Enabled, "")
		}
		if cfg.Channels.IMessage != nil {
			printChannel("iMessage", cfg.Channels.IMessage.Enabled, "")
		}
		if cfg.Channels.WebChat != nil {
			printChannel("WebChat", cfg.Channels.WebChat.Enabled, "")
		}

		return nil
	},
}

var channelsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show channel connection status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		port := 18789
		if cfg != nil && cfg.Gateway.Port != 0 {
			port = cfg.Gateway.Port
		}

		// 尝试从 Gateway 获取实时状态
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/status", port))
		if err != nil {
			fmt.Println("Gateway not running. Start with: openclaw gateway")
			return nil
		}
		defer resp.Body.Close()

		fmt.Println("Channel Status:")
		fmt.Println("(Gateway is running)")
		// TODO: 从 Gateway 获取实时渠道状态

		return nil
	},
}

func init() {
	channelsCmd.AddCommand(channelsListCmd)
	channelsCmd.AddCommand(channelsStatusCmd)
}

// ============================================================================
// models 命令
// ============================================================================

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available AI models",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Available Models:")
		fmt.Println()

		models := []struct {
			ID       string
			Label    string
			Provider string
			Tags     string
		}{
			{"anthropic/claude-opus-4-5", "Claude Opus 4.5", "Anthropic", "flagship"},
			{"anthropic/claude-sonnet-4-20250514", "Claude Sonnet 4", "Anthropic", "balanced"},
			{"anthropic/claude-haiku-3-5", "Claude Haiku 3.5", "Anthropic", "fast"},
			{"openai/gpt-4o", "GPT-4o", "OpenAI", "flagship"},
			{"openai/gpt-4o-mini", "GPT-4o Mini", "OpenAI", "fast"},
			{"openai/o1", "o1", "OpenAI", "reasoning"},
			{"openai/o3", "o3", "OpenAI", "reasoning"},
			{"deepseek/deepseek-chat", "DeepSeek Chat", "DeepSeek", ""},
			{"deepseek/deepseek-reasoner", "DeepSeek Reasoner", "DeepSeek", "reasoning"},
			{"google/gemini-2.0-flash", "Gemini 2.0 Flash", "Google", "fast"},
			{"google/gemini-2.5-pro", "Gemini 2.5 Pro", "Google", "flagship"},
		}

		currentProvider := ""
		for _, m := range models {
			if m.Provider != currentProvider {
				if currentProvider != "" {
					fmt.Println()
				}
				fmt.Printf("─── %s ───\n", m.Provider)
				currentProvider = m.Provider
			}

			tag := ""
			if m.Tags != "" {
				tag = fmt.Sprintf(" [%s]", m.Tags)
			}
			fmt.Printf("  %s%s\n", m.ID, tag)
			fmt.Printf("    %s\n", m.Label)
		}

		// 显示当前默认模型
		cfg, _ := config.Load()
		if cfg != nil && cfg.Agent.DefaultModel != "" {
			fmt.Println()
			fmt.Printf("Current default: %s\n", cfg.Agent.DefaultModel)
		}

		return nil
	},
}

// ============================================================================
// skills 命令
// ============================================================================

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage skills",
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		skillsDir := filepath.Join(home, ".openclaw", "skills")

		entries, err := os.ReadDir(skillsDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No skills installed.")
				fmt.Println()
				fmt.Println("Install skills with:")
				fmt.Println("  openclaw skills install <source>")
				return nil
			}
			return err
		}

		fmt.Println("Installed Skills:")
		fmt.Println()

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillFile); err != nil {
				continue
			}
			fmt.Printf("  - %s\n", entry.Name())
		}

		return nil
	},
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install <source>",
	Short: "Install a skill",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		fmt.Printf("Installing skill from: %s\n", source)

		// TODO: 调用 skills loader

		return nil
	},
}

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
}

// ============================================================================
// logs 命令
// ============================================================================

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View gateway logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		follow, _ := cmd.Flags().GetBool("follow")
		lines, _ := cmd.Flags().GetInt("lines")

		home, _ := os.UserHomeDir()
		logFile := filepath.Join(home, ".openclaw", "logs", "gateway.log")

		if _, err := os.Stat(logFile); err != nil {
			fmt.Println("No log file found.")
			fmt.Println("Gateway logs are written when the gateway is running.")
			return nil
		}

		if follow {
			// tail -f
			tailCmd := exec.Command("tail", "-f", "-n", fmt.Sprintf("%d", lines), logFile)
			tailCmd.Stdout = os.Stdout
			tailCmd.Stderr = os.Stderr
			return tailCmd.Run()
		}

		// tail
		tailCmd := exec.Command("tail", "-n", fmt.Sprintf("%d", lines), logFile)
		tailCmd.Stdout = os.Stdout
		tailCmd.Stderr = os.Stderr
		return tailCmd.Run()
	},
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	logsCmd.Flags().IntP("lines", "n", 50, "Number of lines to show")
}

// ============================================================================
// cron 命令
// ============================================================================

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage cron jobs",
}

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cron jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		port := 18789
		if cfg != nil && cfg.Gateway.Port != 0 {
			port = cfg.Gateway.Port
		}

		// 尝试从 Gateway 获取
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/status", port))
		if err != nil {
			fmt.Println("Gateway not running. Start with: openclaw gateway")
			return nil
		}
		resp.Body.Close()

		fmt.Println("Cron Jobs:")
		fmt.Println("(Query gateway for live status)")

		return nil
	},
}

func init() {
	cronCmd.AddCommand(cronListCmd)
}

// ============================================================================
// init 命令
// ============================================================================

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize OpenClaw project in a directory",
	Long: `Initialize a new OpenClaw project with all configuration files in the project directory.

If no directory is specified, initializes in the current directory.

Project structure:
  ./
  ├── openclaw.json     # Main configuration
  ├── SOUL.md           # Agent personality
  ├── AGENTS.md         # Agent instructions
  ├── USER.md           # User information
  ├── TOOLS.md          # Tool configuration
  ├── IDENTITY.md       # Agent identity
  ├── MEMORY.md         # Long-term memory
  ├── HEARTBEAT.md      # Heartbeat tasks
  ├── memory/           # Daily memory files
  ├── sessions/         # Session transcripts
  ├── skills/           # Installed skills
  ├── plugins/          # Plugins
  ├── cache/            # Cache files
  └── logs/             # Log files
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 确定项目目录
		projectDir := "."
		if len(args) > 0 {
			projectDir = args[0]
		}

		// 转为绝对路径
		projectDir, _ = filepath.Abs(projectDir)

		// 创建路径管理器
		paths := config.NewPaths(projectDir)

		fmt.Printf("🦞 Initializing OpenClaw project in: %s\n", projectDir)
		fmt.Println()

		// 初始化项目
		if err := paths.InitProject(); err != nil {
			return fmt.Errorf("init project: %w", err)
		}

		// 显示创建的内容
		fmt.Println("Created directories:")
		dirs := []string{
			paths.MemoryDir(),
			paths.SessionsDir(),
			paths.SkillsDir(),
			paths.PluginsDir(),
			paths.CacheDir(),
			paths.LogsDir(),
		}
		for _, dir := range dirs {
			rel, _ := filepath.Rel(projectDir, dir)
			fmt.Printf("  ✅ %s/\n", rel)
		}

		fmt.Println()
		fmt.Println("Created files:")
		files := []string{
			paths.ConfigFile(),
			paths.SOULFile(),
			paths.AGENTSFile(),
		}
		for _, f := range files {
			rel, _ := filepath.Rel(projectDir, f)
			fmt.Printf("  ✅ %s\n", rel)
		}

		// 创建其他可选文件
		optionalFiles := map[string]string{
			paths.USERFile(): `# USER.md - About Your Human

*Learn about the person you're helping. Update this as you go.*

- **Name:** 
- **What to call them:** 
- **Timezone:** 
- **Notes:** 

## Context

*(What do they care about? What projects are they working on?)*
`,
			paths.TOOLSFile(): `# TOOLS.md - Local Notes

Add your local tool configuration here (camera names, SSH hosts, etc).
`,
			paths.IDENTITYFile(): `# IDENTITY.md - Who Am I?

- **Name:** 
- **Creature:** 
- **Vibe:** 
- **Emoji:** 
`,
			paths.HEARTBEATFile(): `# HEARTBEAT.md

# Keep this file empty to skip heartbeat tasks.
# Add tasks below when you want periodic checks.
`,
			paths.MEMORYFile(): `# MEMORY.md - Long-term Memory

*Your curated memories. Update this as you learn.*
`,
		}

		for path, content := range optionalFiles {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				os.WriteFile(path, []byte(content), 0644)
				rel, _ := filepath.Rel(projectDir, path)
				fmt.Printf("  ✅ %s\n", rel)
			}
		}

		fmt.Println()
		fmt.Println("🎉 Project initialized!")
		fmt.Println()
		fmt.Println("All configuration is now in this directory.")
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  1. Set your API key:")
		fmt.Println("     export ANTHROPIC_API_KEY=\"sk-ant-...\"")
		fmt.Println()
		fmt.Println("  2. Edit SOUL.md to customize your agent")
		fmt.Println()
		fmt.Println("  3. Start the gateway:")
		fmt.Printf("     cd %s && openclaw gateway\n", projectDir)
		fmt.Println()
		fmt.Println("  4. Or chat directly:")
		fmt.Printf("     cd %s && openclaw chat\n", projectDir)

		return nil
	},
}

// ============================================================================
// config 命令
// ============================================================================

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(data))
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file path",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.GetConfigPath())
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}

		cfgPath := config.GetConfigPath()
		editCmd := exec.Command(editor, cfgPath)
		editCmd.Stdin = os.Stdin
		editCmd.Stdout = os.Stdout
		editCmd.Stderr = os.Stderr

		return editCmd.Run()
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configEditCmd)
}

// ============================================================================
// 注册到 root
// ============================================================================

func init() {
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(channelsCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(cronCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
}
