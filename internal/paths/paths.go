package paths

import (
	"os"
	"path/filepath"
)

// Paths 管理 opencode 配置路径常量。
type Paths struct {
	baseDir         string
	skillSourcesDir string
}

// New 返回基于指定目录的 Paths 实例。baseDir 或 skillSourcesDir 为空时回退到默认值：
//
//	baseDir          = ~/.config/opencode
//	skillSourcesDir  = ~/.cc-switch/skills
func New(baseDir, skillSourcesDir string) *Paths {
	if baseDir == "" {
		home, _ := os.UserHomeDir()
		baseDir = filepath.Join(home, ".config", "opencode")
	}
	if skillSourcesDir == "" {
		home, _ := os.UserHomeDir()
		skillSourcesDir = filepath.Join(home, ".cc-switch", "skills")
	}
	return &Paths{baseDir: baseDir, skillSourcesDir: skillSourcesDir}
}

func (p *Paths) BaseDir() string {
	return p.baseDir
}

func (p *Paths) SkillSourcesDir() string {
	return p.skillSourcesDir
}

func (p *Paths) ConfigFile() string {
	return filepath.Join(p.baseDir, "opencode.json")
}

func (p *Paths) ProfilesDir() string {
	return filepath.Join(p.baseDir, "profiles")
}

func (p *Paths) TUIConfigFile() string {
	return filepath.Join(p.baseDir, "tui.json")
}

func (p *Paths) ProfileDir(name string) string {
	return filepath.Join(p.baseDir, "profiles", name)
}

func (p *Paths) ProfileConfig(name string) string {
	return filepath.Join(p.ProfileDir(name), "opencode.json")
}

func (p *Paths) ProfileTUIConfig(name string) string {
	return filepath.Join(p.ProfileDir(name), "tui.json")
}

func (p *Paths) ProfileSkills(name string) string {
	return filepath.Join(p.ProfileDir(name), "skills")
}

func (p *Paths) ProfileSkillsYML(name string) string {
	return filepath.Join(p.ProfileDir(name), "skills.yml")
}

func (p *Paths) SkillSource(name string) string {
	return filepath.Join(p.skillSourcesDir, name)
}

// RelativeTarget 返回指向指定 profile 配置文件的相对路径（用于 symlink）。
func (p *Paths) RelativeTarget(name string) string {
	return filepath.Join("profiles", name, "opencode.json")
}

// RelativeTargetTUI 返回指向指定 profile tui.json 的相对路径（用于 symlink）。
func (p *Paths) RelativeTargetTUI(name string) string {
	return filepath.Join("profiles", name, "tui.json")
}
