package anthropic

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AuthProfileStore OpenClaw auth-profiles.json 结构
type AuthProfileStore struct {
	Version  int                       `json:"version"`
	Profiles map[string]AuthProfile    `json:"profiles"`
}

// AuthProfile 单个 auth profile
type AuthProfile struct {
	Type     string `json:"type"`     // "api_key" | "oauth" | "token"
	Provider string `json:"provider"`
	Key      string `json:"key,omitempty"`      // for api_key
	Access   string `json:"access,omitempty"`   // for oauth
	Refresh  string `json:"refresh,omitempty"`  // for oauth
	Token    string `json:"token,omitempty"`    // for token
	Expires  int64  `json:"expires,omitempty"`
}

// GetAPIKey 获取 Anthropic API Key
// 优先级: 参数 > 环境变量 > OpenClaw auth-profiles > macOS Keychain
func GetAPIKey(explicit string) string {
	if explicit != "" {
		return explicit
	}
	
	// 环境变量
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	
	// OpenClaw 特定环境变量
	if key := os.Getenv("OPENCLAW_LIVE_ANTHROPIC_KEY"); key != "" {
		return key
	}
	
	// OpenClaw auth-profiles.json
	if key := getFromOpenClawAuthProfiles(); key != "" {
		return key
	}
	
	// macOS Keychain (fallback)
	if key := getFromKeychain("openclaw-anthropic-api-key"); key != "" {
		return key
	}
	
	return ""
}

// getFromOpenClawAuthProfiles 从 OpenClaw auth-profiles.json 读取 API key
func getFromOpenClawAuthProfiles() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	
	// 尝试多个可能的路径
	paths := []string{
		filepath.Join(home, ".openclaw", "agents", "main", "agent", "auth-profiles.json"),
		filepath.Join(home, ".openclaw", "auth-profiles.json"),
	}
	
	for _, path := range paths {
		key := readAuthProfilesFile(path)
		if key != "" {
			return key
		}
	}
	
	return ""
}

// readAuthProfilesFile 读取并解析 auth-profiles.json
func readAuthProfilesFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	
	var store AuthProfileStore
	if err := json.Unmarshal(data, &store); err != nil {
		return ""
	}
	
	// 查找 anthropic profile
	profileKeys := []string{
		"anthropic:default",
		"anthropic",
	}
	
	for _, profileKey := range profileKeys {
		if profile, ok := store.Profiles[profileKey]; ok {
			if profile.Type == "api_key" && profile.Key != "" {
				return profile.Key
			}
			// OAuth access token 也可以用
			if profile.Type == "oauth" && profile.Access != "" {
				return profile.Access
			}
		}
	}
	
	return ""
}

// getFromKeychain 从 macOS Keychain 读取密码
func getFromKeychain(service string) string {
	cmd := exec.Command("security", "find-generic-password", "-s", service, "-w")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
