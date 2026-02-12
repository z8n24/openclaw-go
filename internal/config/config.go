package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

var (
	configPath string
	cfg        *Config
	cfgMu      sync.RWMutex
	watchers   []func(*Config)
)

// Config 是 OpenClaw 的完整配置结构
type Config struct {
	// Gateway 配置
	Gateway GatewayConfig `json:"gateway,omitempty"`

	// 渠道配置
	Channels ChannelsConfig `json:"channels,omitempty"`

	// Agent 配置
	Agent AgentConfig `json:"agent,omitempty"`

	// 消息配置
	Messages MessagesConfig `json:"messages,omitempty"`

	// 插件配置
	Plugins PluginsConfig `json:"plugins,omitempty"`

	// Tools 配置
	Tools ToolsConfig `json:"tools,omitempty"`

	// Cron 配置
	Cron CronConfig `json:"cron,omitempty"`

	// TTS 配置
	TTS TTSConfig `json:"tts,omitempty"`
}

type GatewayConfig struct {
	Port       int    `json:"port,omitempty"`
	Bind       string `json:"bind,omitempty"`
	Token      string `json:"token,omitempty"`
	CanvasHost struct {
		Port int `json:"port,omitempty"`
	} `json:"canvasHost,omitempty"`
}

type ChannelsConfig struct {
	Telegram  *TelegramConfig  `json:"telegram,omitempty"`
	WhatsApp  *WhatsAppConfig  `json:"whatsapp,omitempty"`
	Discord   *DiscordConfig   `json:"discord,omitempty"`
	Signal    *SignalConfig    `json:"signal,omitempty"`
	IMessage  *IMessageConfig  `json:"imessage,omitempty"`
	WebChat   *WebChatConfig   `json:"webchat,omitempty"`
}

type TelegramConfig struct {
	BotToken  string   `json:"botToken,omitempty"`
	AllowFrom []string `json:"allowFrom,omitempty"`
	Enabled   bool     `json:"enabled,omitempty"`
}

type WhatsAppConfig struct {
	AllowFrom []string              `json:"allowFrom,omitempty"`
	Groups    map[string]GroupConfig `json:"groups,omitempty"`
	Enabled   bool                  `json:"enabled,omitempty"`
}

type DiscordConfig struct {
	BotToken  string   `json:"botToken,omitempty"`
	AllowFrom []string `json:"allowFrom,omitempty"`
	Guilds    []string `json:"guilds,omitempty"`
	Enabled   bool     `json:"enabled,omitempty"`
}

type SignalConfig struct {
	RESTUrl   string   `json:"restUrl,omitempty"`
	Number    string   `json:"number,omitempty"`
	AllowFrom []string `json:"allowFrom,omitempty"`
	Enabled   bool     `json:"enabled,omitempty"`
}

type IMessageConfig struct {
	AllowFrom []string `json:"allowFrom,omitempty"`
	Enabled   bool     `json:"enabled,omitempty"`
}

type WebChatConfig struct {
	Enabled bool `json:"enabled,omitempty"`
}

type GroupConfig struct {
	RequireMention bool     `json:"requireMention,omitempty"`
	AllowFrom      []string `json:"allowFrom,omitempty"`
}

type AgentConfig struct {
	DefaultModel string            `json:"defaultModel,omitempty"`
	Workspace    string            `json:"workspace,omitempty"`
	Models       map[string]string `json:"models,omitempty"` // 别名映射
	Thinking     string            `json:"thinking,omitempty"`
}

type MessagesConfig struct {
	GroupChat struct {
		MentionPatterns []string `json:"mentionPatterns,omitempty"`
	} `json:"groupChat,omitempty"`
}

type PluginsConfig struct {
	Enabled bool                   `json:"enabled,omitempty"`
	Allow   []string               `json:"allow,omitempty"`
	Deny    []string               `json:"deny,omitempty"`
	Entries map[string]PluginEntry `json:"entries,omitempty"`
}

type PluginEntry struct {
	Enabled bool        `json:"enabled,omitempty"`
	Config  interface{} `json:"config,omitempty"`
}

type ToolsConfig struct {
	Exec struct {
		Enabled   bool     `json:"enabled,omitempty"`
		Allowlist []string `json:"allowlist,omitempty"`
	} `json:"exec,omitempty"`
	Browser struct {
		Enabled bool   `json:"enabled,omitempty"`
		Profile string `json:"profile,omitempty"`
	} `json:"browser,omitempty"`
}

type CronConfig struct {
	Enabled bool      `json:"enabled,omitempty"`
	Jobs    []CronJob `json:"jobs,omitempty"`
}

type CronJob struct {
	ID            string      `json:"id,omitempty"`
	Name          string      `json:"name,omitempty"`
	Schedule      interface{} `json:"schedule,omitempty"`
	Payload       interface{} `json:"payload,omitempty"`
	SessionTarget string      `json:"sessionTarget,omitempty"`
	Enabled       bool        `json:"enabled,omitempty"`
}

type TTSConfig struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Provider string `json:"provider,omitempty"`
	Voice    string `json:"voice,omitempty"`
}

// SetConfigFile 设置配置文件路径
func SetConfigFile(path string) {
	configPath = path
}

// GetConfigPath 返回配置文件路径
func GetConfigPath() string {
	if configPath != "" {
		return configPath
	}
	
	// 使用项目路径系统
	return GetPaths().ConfigFile()
}

// Load 加载配置
func Load() (*Config, error) {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	path := GetConfigPath()
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 返回默认配置
			cfg = defaultConfig()
			return cfg, nil
		}
		return nil, err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}

	cfg = &c
	return cfg, nil
}

// Get 获取当前配置 (线程安全)
func Get() *Config {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfg
}

// Watch 监听配置变化
func Watch(callback func(*Config)) error {
	watchers = append(watchers, callback)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					log.Info().Msg("Config file changed, reloading...")
					if newCfg, err := Load(); err == nil {
						for _, w := range watchers {
							w(newCfg)
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error().Err(err).Msg("Config watcher error")
			}
		}
	}()

	return watcher.Add(GetConfigPath())
}

func defaultConfig() *Config {
	return &Config{
		Gateway: GatewayConfig{
			Port: 18789,
			Bind: "127.0.0.1",
		},
		Agent: AgentConfig{
			DefaultModel: "anthropic/claude-sonnet-4-20250514",
		},
	}
}

// Save 保存配置
func Save(c *Config) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	path := GetConfigPath()
	
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	cfg = c
	return os.WriteFile(path, data, 0644)
}
