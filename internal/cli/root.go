package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/user/openclaw-go/internal/agents"
	"github.com/user/openclaw-go/internal/agents/anthropic"
	"github.com/user/openclaw-go/internal/agents/deepseek"
	"github.com/user/openclaw-go/internal/agents/tools"
	"github.com/user/openclaw-go/internal/channels"
	"github.com/user/openclaw-go/internal/channels/telegram"
	"github.com/user/openclaw-go/internal/config"
	"github.com/user/openclaw-go/internal/cron"
	"github.com/user/openclaw-go/internal/gateway"
	"github.com/user/openclaw-go/internal/sessions"
)

var (
	cfgFile string
	verbose bool

	// 版本信息
	cliVersion = "dev"
	cliCommit  = "unknown"
	cliDate    = "unknown"
)

// SetVersionInfo 设置版本信息
func SetVersionInfo(version, commit, date string) {
	cliVersion = version
	cliCommit = commit
	cliDate = date
}

var rootCmd = &cobra.Command{
	Use:   "openclaw",
	Short: "OpenClaw - AI Agent Gateway",
	Long: `OpenClaw bridges messaging platforms (WhatsApp, Telegram, Discord, etc.) 
to AI agents with tool execution capabilities.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.openclaw/openclaw.json)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(telegramCmd)
}

func initConfig() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	if verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	if cfgFile != "" {
		config.SetConfigFile(cfgFile)
	}
}

// gateway 命令
var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the OpenClaw gateway server",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		bind, _ := cmd.Flags().GetString("bind")
		
		cfg, err := config.Load()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to load config, using defaults")
			cfg = &config.Config{}
		}
		
		if port != 0 {
			cfg.Gateway.Port = port
		}
		if cfg.Gateway.Port == 0 {
			cfg.Gateway.Port = 18789
		}
		if bind != "" {
			cfg.Gateway.Bind = bind
		}
		if cfg.Gateway.Bind == "" {
			cfg.Gateway.Bind = "127.0.0.1"
		}
		
		server := gateway.NewServer(cfg)
		
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		
		go func() {
			<-sigCh
			log.Info().Msg("Shutting down...")
			server.Stop()
		}()
		
		return server.Start()
	},
}

func init() {
	gatewayCmd.Flags().IntP("port", "p", 18789, "WebSocket/HTTP port")
	gatewayCmd.Flags().String("bind", "127.0.0.1", "Bind address")
	gatewayCmd.Flags().String("token", "", "Gateway authentication token")
}

// chat 命令 - 直接在终端对话
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	RunE: func(cmd *cobra.Command, args []string) error {
		model, _ := cmd.Flags().GetString("model")
		apiKey, _ := cmd.Flags().GetString("api-key")
		workspace, _ := cmd.Flags().GetString("workspace")
		providerName, _ := cmd.Flags().GetString("provider")
		
		// 根据 provider 设置默认 model
		if model == "" {
			switch providerName {
			case "deepseek":
				model = "deepseek-chat"
			default:
				model = "claude-sonnet-4-20250514"
			}
		}
		
		// 如果 model 包含 deepseek，自动选择 provider
		if strings.Contains(model, "deepseek") {
			providerName = "deepseek"
		}
		
		// 默认 workspace
		if workspace == "" {
			home, _ := os.UserHomeDir()
			workspace = filepath.Join(home, ".openclaw", "workspace")
		}
		
		// 确保 workspace 存在
		os.MkdirAll(workspace, 0755)
		
		// 创建 provider
		var provider agents.Provider
		switch providerName {
		case "deepseek":
			if deepseek.GetAPIKey(apiKey) == "" {
				fmt.Println("Error: DEEPSEEK_API_KEY not set")
				fmt.Println()
				fmt.Println("Set it via:")
				fmt.Println("  export DEEPSEEK_API_KEY=\"sk-...\"")
				fmt.Println("Or pass --api-key flag")
				return nil
			}
			provider = deepseek.NewClient(apiKey)
		default: // anthropic
			providerName = "anthropic"
			if anthropic.GetAPIKey(apiKey) == "" {
				fmt.Println("Error: ANTHROPIC_API_KEY not set")
				fmt.Println()
				fmt.Println("Set it via:")
				fmt.Println("  export ANTHROPIC_API_KEY=\"sk-ant-...\"")
				fmt.Println("Or pass --api-key flag")
				return nil
			}
			provider = anthropic.NewClient(apiKey)
		}
		
		// 创建会话
		sessionMgr := sessions.NewManager()
		session := sessionMgr.GetOrCreate("cli", "main", "CLI Session")
		
		// 创建 cron scheduler
		home, _ := os.UserHomeDir()
		stateDir := filepath.Join(home, ".openclaw", "state")
		cronScheduler := cron.NewScheduler(stateDir, nil)
		cronScheduler.Start()
		defer cronScheduler.Stop()
		
		// 创建并注册工具
		toolRegistry := createToolRegistry(workspace, cronScheduler)
		
		// 构建 system prompt
		toolList := getToolList()
		systemPrompt := agents.BuildSystemPrompt(workspace, toolList)
		
		// 创建 agent loop
		loop := sessions.NewAgentLoop(provider, toolRegistry, session, systemPrompt, model)
		
		fmt.Println("OpenClaw Chat (type 'exit' to quit, 'clear' to reset)")
		fmt.Println("Provider:", providerName)
		fmt.Println("Model:", model)
		fmt.Println("Workspace:", workspace)
		fmt.Println()
		
		scanner := bufio.NewScanner(os.Stdin)
		
		for {
			fmt.Print("You: ")
			if !scanner.Scan() {
				break
			}
			
			input := strings.TrimSpace(scanner.Text())
			if input == "" {
				continue
			}
			if input == "exit" || input == "quit" {
				break
			}
			if input == "clear" {
				session.ClearMessages()
				fmt.Println("Session cleared.")
				continue
			}
			
			fmt.Print("\nAssistant: ")
			
			ctx := context.Background()
			_, err := loop.Run(ctx, input, func(delta string) {
				fmt.Print(delta)
			})
			
			if err != nil {
				fmt.Printf("\nError: %v\n", err)
			}
			
			fmt.Println("\n")
		}
		
		return nil
	},
}

func init() {
	chatCmd.Flags().StringP("model", "m", "", "Model to use (default depends on provider)")
	chatCmd.Flags().StringP("provider", "p", "anthropic", "Provider: anthropic, deepseek")
	chatCmd.Flags().String("api-key", "", "API key (or set ANTHROPIC_API_KEY / DEEPSEEK_API_KEY)")
	chatCmd.Flags().StringP("workspace", "w", "", "Workspace directory")
}

// createToolRegistry 创建并注册所有工具
func createToolRegistry(workspace string, cronScheduler *cron.Scheduler) *sessions.ToolRegistry {
	registry := sessions.NewToolRegistry()
	
	// 文件操作工具
	readTool := tools.NewReadTool(workspace)
	registry.Register(readTool.Name(), func(ctx context.Context, args json.RawMessage) (string, error) {
		result, err := readTool.Execute(ctx, args)
		if err != nil {
			return "", err
		}
		return result.Content, nil
	})
	
	writeTool := tools.NewWriteTool(workspace)
	registry.Register(writeTool.Name(), func(ctx context.Context, args json.RawMessage) (string, error) {
		result, err := writeTool.Execute(ctx, args)
		if err != nil {
			return "", err
		}
		return result.Content, nil
	})
	
	editTool := tools.NewEditTool(workspace)
	registry.Register(editTool.Name(), func(ctx context.Context, args json.RawMessage) (string, error) {
		result, err := editTool.Execute(ctx, args)
		if err != nil {
			return "", err
		}
		return result.Content, nil
	})
	
	// 命令执行工具
	execTool := tools.NewExecTool(workspace)
	registry.Register(execTool.Name(), func(ctx context.Context, args json.RawMessage) (string, error) {
		result, err := execTool.Execute(ctx, args)
		if err != nil {
			return "", err
		}
		return result.Content, nil
	})
	
	// Web 工具
	webSearchTool := tools.NewWebSearchTool()
	registry.Register(webSearchTool.Name(), func(ctx context.Context, args json.RawMessage) (string, error) {
		result, err := webSearchTool.Execute(ctx, args)
		if err != nil {
			return "", err
		}
		return result.Content, nil
	})
	
	webFetchTool := tools.NewWebFetchTool()
	registry.Register(webFetchTool.Name(), func(ctx context.Context, args json.RawMessage) (string, error) {
		result, err := webFetchTool.Execute(ctx, args)
		if err != nil {
			return "", err
		}
		return result.Content, nil
	})
	
	// 浏览器工具
	browserTool := tools.NewBrowserTool()
	registry.Register(browserTool.Name(), func(ctx context.Context, args json.RawMessage) (string, error) {
		result, err := browserTool.Execute(ctx, args)
		if err != nil {
			return "", err
		}
		return result.Content, nil
	})
	
	// 记忆工具
	memorySearchTool := tools.NewMemorySearchTool(workspace)
	registry.Register(memorySearchTool.Name(), func(ctx context.Context, args json.RawMessage) (string, error) {
		result, err := memorySearchTool.Execute(ctx, args)
		if err != nil {
			return "", err
		}
		return result.Content, nil
	})
	
	memoryGetTool := tools.NewMemoryGetTool(workspace)
	registry.Register(memoryGetTool.Name(), func(ctx context.Context, args json.RawMessage) (string, error) {
		result, err := memoryGetTool.Execute(ctx, args)
		if err != nil {
			return "", err
		}
		return result.Content, nil
	})
	
	// Cron 工具
	cronTool := tools.NewCronTool(cronScheduler)
	registry.Register(cronTool.Name(), func(ctx context.Context, args json.RawMessage) (string, error) {
		result, err := cronTool.Execute(ctx, args)
		if err != nil {
			return "", err
		}
		return result.Content, nil
	})
	
	return registry
}

// getToolList 获取工具列表用于 system prompt
func getToolList() []agents.Tool {
	return []agents.Tool{
		{Name: "read", Description: "Read file contents. Supports text and images."},
		{Name: "write", Description: "Write content to file. Creates directories automatically."},
		{Name: "edit", Description: "Edit file by replacing exact text."},
		{Name: "exec", Description: "Execute shell commands."},
		{Name: "web_search", Description: "Search the web using Brave Search API."},
		{Name: "web_fetch", Description: "Fetch and extract content from URL (HTML → markdown)."},
		{Name: "browser", Description: "Control web browser: navigate, screenshot, interact."},
		{Name: "memory_search", Description: "Search MEMORY.md and memory/*.md files."},
		{Name: "memory_get", Description: "Read snippet from memory files."},
		{Name: "cron", Description: "Manage cron jobs: add, list, update, remove, run."},
	}
}

// version 命令
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("openclaw-go version %s\n", cliVersion)
		fmt.Printf("Protocol version: 3\n")
		if cliCommit != "unknown" {
			fmt.Printf("Commit: %s\n", cliCommit)
		}
		if cliDate != "unknown" {
			fmt.Printf("Built: %s\n", cliDate)
		}
	},
}

// telegram 命令 - 测试 Telegram bot
var telegramCmd = &cobra.Command{
	Use:   "telegram",
	Short: "Run Telegram bot with AI agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		botToken, _ := cmd.Flags().GetString("token")
		model, _ := cmd.Flags().GetString("model")
		providerName, _ := cmd.Flags().GetString("provider")
		workspace, _ := cmd.Flags().GetString("workspace")
		allowFrom, _ := cmd.Flags().GetStringSlice("allow")
		
		if botToken == "" {
			botToken = os.Getenv("TELEGRAM_BOT_TOKEN")
		}
		if botToken == "" {
			fmt.Println("Error: Telegram bot token required")
			fmt.Println()
			fmt.Println("Set via:")
			fmt.Println("  export TELEGRAM_BOT_TOKEN=\"your-token\"")
			fmt.Println("Or: --token flag")
			fmt.Println()
			fmt.Println("Get token from @BotFather on Telegram")
			return nil
		}
		
		// 默认设置
		if model == "" {
			switch providerName {
			case "deepseek":
				model = "deepseek-chat"
			default:
				model = "claude-sonnet-4-20250514"
			}
		}
		if strings.Contains(model, "deepseek") {
			providerName = "deepseek"
		}
		if workspace == "" {
			home, _ := os.UserHomeDir()
			workspace = filepath.Join(home, ".openclaw", "workspace")
		}
		os.MkdirAll(workspace, 0755)
		
		// 创建 provider
		var provider agents.Provider
		switch providerName {
		case "deepseek":
			if deepseek.GetAPIKey("") == "" {
				fmt.Println("Error: DEEPSEEK_API_KEY not set")
				return nil
			}
			provider = deepseek.NewClient("")
		default:
			providerName = "anthropic"
			if anthropic.GetAPIKey("") == "" {
				fmt.Println("Error: ANTHROPIC_API_KEY not set")
				return nil
			}
			provider = anthropic.NewClient("")
		}
		
		// 创建 Telegram channel
		tgConfig := &config.TelegramConfig{
			BotToken:  botToken,
			AllowFrom: allowFrom,
			Enabled:   true,
		}
		tgChannel := telegram.New(tgConfig)
		
		// 创建会话管理器
		sessionMgr := sessions.NewManager()
		
		// 创建 cron scheduler
		home, _ := os.UserHomeDir()
		stateDir := filepath.Join(home, ".openclaw", "state")
		cronScheduler := cron.NewScheduler(stateDir, nil)
		cronScheduler.Start()
		defer cronScheduler.Stop()
		
		// 创建工具注册表
		toolRegistry := createToolRegistry(workspace, cronScheduler)
		
		// 构建 system prompt
		toolList := getToolList()
		systemPrompt := agents.BuildSystemPrompt(workspace, toolList)
		
		// 设置消息处理器
		tgChannel.SetMessageHandler(func(msg *channels.InboundMessage) {
			log.Info().
				Str("from", msg.SenderName).
				Str("chatId", msg.ChatID).
				Str("text", truncate(msg.Text, 50)).
				Msg("Received message")
			
			// 获取或创建会话
			sessionKey := fmt.Sprintf("telegram:%s", msg.ChatID)
			session := sessionMgr.GetOrCreate(sessionKey, "telegram", msg.SenderName)
			
			// 创建 agent loop
			loop := sessions.NewAgentLoop(provider, toolRegistry, session, systemPrompt, model)
			
			// 运行 agent
			ctx := context.Background()
			var response strings.Builder
			
			resp, err := loop.Run(ctx, msg.Text, func(delta string) {
				response.WriteString(delta)
			})
			
			if err != nil {
				log.Error().Err(err).Msg("Agent error")
				tgChannel.Send(ctx, &channels.OutboundMessage{
					ChatID: msg.ChatID,
					Text:   "Error: " + err.Error(),
				})
				return
			}
			
			// 发送响应
			replyText := response.String()
			if replyText == "" && resp != nil {
				replyText = resp.Content
			}
			if replyText == "" {
				replyText = "No response"
			}
			
			// Telegram 限制 4096 字符
			if len(replyText) > 4000 {
				replyText = replyText[:4000] + "\n...[truncated]"
			}
			
			_, err = tgChannel.Send(ctx, &channels.OutboundMessage{
				ChatID:  msg.ChatID,
				Text:    replyText,
				ReplyTo: msg.ID,
			})
			if err != nil {
				log.Error().Err(err).Msg("Failed to send reply")
			}
		})
		
		// 启动 Telegram channel
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		
		if err := tgChannel.Start(ctx); err != nil {
			return fmt.Errorf("failed to start telegram: %w", err)
		}
		
		fmt.Println("Telegram bot started!")
		fmt.Println("Provider:", providerName)
		fmt.Println("Model:", model)
		fmt.Println("Workspace:", workspace)
		if len(allowFrom) > 0 {
			fmt.Println("Allowed users:", strings.Join(allowFrom, ", "))
		} else {
			fmt.Println("Allowed users: everyone")
		}
		fmt.Println()
		fmt.Println("Send a message to your bot to test!")
		fmt.Println("Press Ctrl+C to stop")
		
		// 等待信号
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		
		fmt.Println("\nStopping...")
		tgChannel.Stop()
		
		return nil
	},
}

func init() {
	telegramCmd.Flags().String("token", "", "Telegram bot token (or TELEGRAM_BOT_TOKEN env)")
	telegramCmd.Flags().StringP("model", "m", "", "Model to use")
	telegramCmd.Flags().StringP("provider", "p", "anthropic", "Provider: anthropic, deepseek")
	telegramCmd.Flags().StringP("workspace", "w", "", "Workspace directory")
	telegramCmd.Flags().StringSlice("allow", nil, "Allowed user IDs/usernames (empty = everyone)")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
