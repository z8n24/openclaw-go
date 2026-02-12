package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Loader 技能加载器
type Loader struct {
	skills     map[string]*Skill
	skillDirs  []string
	stateFile  string
	mu         sync.RWMutex
}

// LoaderConfig 加载器配置
type LoaderConfig struct {
	SkillDirs []string // 技能目录列表
	StateFile string   // 状态文件路径
}

// NewLoader 创建技能加载器
func NewLoader(cfg LoaderConfig) *Loader {
	if len(cfg.SkillDirs) == 0 {
		home, _ := os.UserHomeDir()
		cfg.SkillDirs = []string{
			filepath.Join(home, ".openclaw", "skills"),
		}
	}
	if cfg.StateFile == "" {
		home, _ := os.UserHomeDir()
		cfg.StateFile = filepath.Join(home, ".openclaw", "state", "skills.json")
	}

	l := &Loader{
		skills:    make(map[string]*Skill),
		skillDirs: cfg.SkillDirs,
		stateFile: cfg.StateFile,
	}

	// 确保目录存在
	for _, dir := range l.skillDirs {
		os.MkdirAll(dir, 0755)
	}

	return l
}

// LoadAll 加载所有技能
func (l *Loader) LoadAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 先加载状态文件中的已安装技能列表
	l.loadState()

	for _, dir := range l.skillDirs {
		if err := l.scanDirectory(dir); err != nil {
			log.Warn().Err(err).Str("dir", dir).Msg("Failed to scan skill directory")
		}
	}

	log.Info().Int("count", len(l.skills)).Msg("Loaded skills")
	return nil
}

// scanDirectory 扫描目录中的技能
func (l *Loader) scanDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(dir, entry.Name())
		skillFile := filepath.Join(skillPath, "SKILL.md")

		// 检查 SKILL.md 是否存在
		if _, err := os.Stat(skillFile); err != nil {
			continue
		}

		// 解析技能
		skill, err := ParseSKILLMD(skillFile)
		if err != nil {
			log.Warn().Err(err).Str("path", skillFile).Msg("Failed to parse skill")
			continue
		}

		// 恢复启用状态（从 state）
		if existing, ok := l.skills[skill.ID]; ok {
			skill.Enabled = existing.Enabled
		}

		l.skills[skill.ID] = skill
		log.Debug().Str("id", skill.ID).Str("name", skill.Name).Msg("Loaded skill")
	}

	return nil
}

// Get 获取技能
func (l *Loader) Get(id string) (*Skill, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	skill, ok := l.skills[id]
	return skill, ok
}

// List 列出所有技能
func (l *Loader) List() []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	skills := make([]*Skill, 0, len(l.skills))
	for _, s := range l.skills {
		skills = append(skills, s)
	}
	return skills
}

// ListEnabled 列出已启用的技能
func (l *Loader) ListEnabled() []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var skills []*Skill
	for _, s := range l.skills {
		if s.Enabled {
			skills = append(skills, s)
		}
	}
	return skills
}

// Enable 启用技能
func (l *Loader) Enable(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	skill, ok := l.skills[id]
	if !ok {
		return fmt.Errorf("skill not found: %s", id)
	}

	skill.Enabled = true
	l.saveState()
	return nil
}

// Disable 禁用技能
func (l *Loader) Disable(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	skill, ok := l.skills[id]
	if !ok {
		return fmt.Errorf("skill not found: %s", id)
	}

	skill.Enabled = false
	l.saveState()
	return nil
}

// Install 安装技能
func (l *Loader) Install(source string) (*Skill, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var skill *Skill
	var err error

	// 判断来源类型
	if strings.HasPrefix(source, "github.com/") || strings.HasPrefix(source, "https://github.com/") {
		skill, err = l.installFromGitHub(source)
	} else if strings.HasPrefix(source, "/") || strings.HasPrefix(source, "./") || strings.HasPrefix(source, "~") {
		skill, err = l.installFromLocal(source)
	} else {
		// 假设是 GitHub 短格式 (user/repo)
		skill, err = l.installFromGitHub("github.com/" + source)
	}

	if err != nil {
		return nil, err
	}

	l.skills[skill.ID] = skill
	l.saveState()

	log.Info().Str("id", skill.ID).Str("name", skill.Name).Msg("Installed skill")
	return skill, nil
}

// installFromGitHub 从 GitHub 安装
func (l *Loader) installFromGitHub(source string) (*Skill, error) {
	// 解析 URL
	source = strings.TrimPrefix(source, "https://")
	parts := strings.Split(source, "/")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid github url: %s", source)
	}

	user := parts[1]
	repo := parts[2]
	ref := "main"

	// 检查是否指定了 ref (@branch 或 #branch)
	if idx := strings.Index(repo, "@"); idx != -1 {
		ref = repo[idx+1:]
		repo = repo[:idx]
	} else if idx := strings.Index(repo, "#"); idx != -1 {
		ref = repo[idx+1:]
		repo = repo[:idx]
	}

	// 确定安装目录
	destDir := filepath.Join(l.skillDirs[0], repo)

	// 如果目录已存在，先删除
	if _, err := os.Stat(destDir); err == nil {
		os.RemoveAll(destDir)
	}

	// git clone
	cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", user, repo)
	cmd := exec.Command("git", "clone", "--depth", "1", "-b", ref, cloneURL, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git clone failed: %w", err)
	}

	// 解析 SKILL.md
	skillFile := filepath.Join(destDir, "SKILL.md")
	skill, err := ParseSKILLMD(skillFile)
	if err != nil {
		os.RemoveAll(destDir)
		return nil, fmt.Errorf("parse skill: %w", err)
	}

	skill.Source = SkillSource{
		Kind: "github",
		URL:  fmt.Sprintf("https://github.com/%s/%s", user, repo),
		Ref:  ref,
	}

	// 获取 commit hash
	cmd = exec.Command("git", "-C", destDir, "rev-parse", "HEAD")
	if out, err := cmd.Output(); err == nil {
		skill.Source.Commit = strings.TrimSpace(string(out))
	}

	return skill, nil
}

// installFromLocal 从本地路径安装 (创建符号链接)
func (l *Loader) installFromLocal(source string) (*Skill, error) {
	// 展开 ~
	if strings.HasPrefix(source, "~") {
		home, _ := os.UserHomeDir()
		source = filepath.Join(home, source[1:])
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(source)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	// 检查 SKILL.md 存在
	skillFile := filepath.Join(absPath, "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		return nil, fmt.Errorf("SKILL.md not found in %s", absPath)
	}

	// 解析技能
	skill, err := ParseSKILLMD(skillFile)
	if err != nil {
		return nil, fmt.Errorf("parse skill: %w", err)
	}

	// 创建符号链接
	linkPath := filepath.Join(l.skillDirs[0], skill.ID)
	if _, err := os.Lstat(linkPath); err == nil {
		os.Remove(linkPath)
	}

	if err := os.Symlink(absPath, linkPath); err != nil {
		return nil, fmt.Errorf("create symlink: %w", err)
	}

	skill.Source = SkillSource{
		Kind: "local",
		URL:  absPath,
	}

	return skill, nil
}

// Uninstall 卸载技能
func (l *Loader) Uninstall(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	skill, ok := l.skills[id]
	if !ok {
		return fmt.Errorf("skill not found: %s", id)
	}

	// 删除目录
	if err := os.RemoveAll(skill.Path); err != nil {
		return fmt.Errorf("remove skill directory: %w", err)
	}

	delete(l.skills, id)
	l.saveState()

	log.Info().Str("id", id).Msg("Uninstalled skill")
	return nil
}

// Update 更新技能
func (l *Loader) Update(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	skill, ok := l.skills[id]
	if !ok {
		return fmt.Errorf("skill not found: %s", id)
	}

	if skill.Source.Kind != "github" {
		return fmt.Errorf("can only update github skills")
	}

	// git pull
	cmd := exec.Command("git", "-C", skill.Path, "pull", "--ff-only")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	// 重新解析
	skillFile := filepath.Join(skill.Path, "SKILL.md")
	newSkill, err := ParseSKILLMD(skillFile)
	if err != nil {
		return fmt.Errorf("parse updated skill: %w", err)
	}

	// 保留原有状态
	newSkill.Source = skill.Source
	newSkill.Enabled = skill.Enabled

	// 更新 commit
	cmd = exec.Command("git", "-C", skill.Path, "rev-parse", "HEAD")
	if out, err := cmd.Output(); err == nil {
		newSkill.Source.Commit = strings.TrimSpace(string(out))
	}

	l.skills[id] = newSkill
	l.saveState()

	log.Info().Str("id", id).Msg("Updated skill")
	return nil
}

// GetBinaries 获取所有技能的二进制文件路径
func (l *Loader) GetBinaries() map[string]string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	bins := make(map[string]string)
	for _, skill := range l.skills {
		if !skill.Enabled {
			continue
		}
		for _, tool := range skill.Tools {
			if tool.Binary != "" {
				binPath, err := skill.GetBinary(tool.Name)
				if err == nil {
					bins[tool.Name] = binPath
				}
			}
		}
	}
	return bins
}

// Status 获取加载器状态
type Status struct {
	TotalSkills   int    `json:"totalSkills"`
	EnabledSkills int    `json:"enabledSkills"`
	TotalTools    int    `json:"totalTools"`
	SkillDirs     []string `json:"skillDirs"`
}

func (l *Loader) Status() Status {
	l.mu.RLock()
	defer l.mu.RUnlock()

	status := Status{
		TotalSkills: len(l.skills),
		SkillDirs:   l.skillDirs,
	}

	for _, s := range l.skills {
		if s.Enabled {
			status.EnabledSkills++
		}
		status.TotalTools += len(s.Tools)
	}

	return status
}

// ============================================================================
// State persistence
// ============================================================================

type skillState struct {
	ID        string    `json:"id"`
	Enabled   bool      `json:"enabled"`
	Source    SkillSource `json:"source"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type stateFile struct {
	Skills []skillState `json:"skills"`
}

func (l *Loader) loadState() {
	data, err := os.ReadFile(l.stateFile)
	if err != nil {
		return
	}

	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}

	for _, s := range state.Skills {
		l.skills[s.ID] = &Skill{
			ID:      s.ID,
			Enabled: s.Enabled,
			Source:  s.Source,
		}
	}
}

func (l *Loader) saveState() {
	states := make([]skillState, 0, len(l.skills))
	for _, s := range l.skills {
		states = append(states, skillState{
			ID:        s.ID,
			Enabled:   s.Enabled,
			Source:    s.Source,
			UpdatedAt: time.Now(),
		})
	}

	state := stateFile{Skills: states}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}

	os.MkdirAll(filepath.Dir(l.stateFile), 0755)
	os.WriteFile(l.stateFile, data, 0644)
}
