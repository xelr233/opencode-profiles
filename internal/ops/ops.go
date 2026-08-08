package ops

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"opencode-profiles/internal/paths"
	"opencode-profiles/internal/skills"
)

// copyFile 复制文件内容、权限与时间戳（近似 shutil.copy2）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, fi.ModTime(), fi.ModTime())
}

func isSymlink(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

// pythonList 将 Go 切片渲染为 Python repr 风格的列表（用于错误消息）。
func pythonList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = "'" + s + "'"
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// EnsureInitialized 确保 opencode 配置目录已初始化。
//
// 如果 opencode.json 不是 symlink，将其内容存入 default profile 并替换为 symlink。
// 如果已经是有效 symlink（目标存在），不做任何操作。
// 如果 symlink 悬空（目标不存在），清理后重新初始化。
func EnsureInitialized(p *paths.Paths) error {
	if err := os.MkdirAll(p.ProfilesDir(), 0o755); err != nil {
		return err
	}

	config := p.ConfigFile()

	// Valid symlink: target exists
	if isSymlink(config) {
		if _, err := os.Stat(config); err == nil {
			if _, err := os.Stat(p.ProfileSkillsYML("default")); os.IsNotExist(err) {
				current, err := skills.ScanCurrentSkills(p)
				if err != nil {
					return err
				}
				if err := skills.WriteSkillsYML(p, "default", current); err != nil {
					return err
				}
			}
			return nil
		}
	}

	// Clean up dangling symlink
	if isSymlink(config) {
		if err := os.Remove(config); err != nil {
			return err
		}
	}

	defaultDir := p.ProfileDir("default")
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		return err
	}
	defaultConfig := p.ProfileConfig("default")
	defaultTUIConfig := p.ProfileTUIConfig("default")
	skillsDir := p.ProfileSkills("default")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return err
	}

	if _, err := os.Stat(config); err == nil {
		backupPath := filepath.Join(p.BaseDir(), "opencode.json.bak")
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			if err := copyFile(config, backupPath); err != nil {
				return err
			}
		}
		if err := copyFile(config, defaultConfig); err != nil {
			return err
		}
		if err := os.Remove(config); err != nil {
			return err
		}
	} else {
		// Config doesn't exist (dangling symlink removed or fresh install)
		backupPath := filepath.Join(p.BaseDir(), "opencode.json.bak")
		if _, err := os.Stat(backupPath); err == nil {
			if err := copyFile(backupPath, defaultConfig); err != nil {
				return err
			}
		} else {
			if err := os.WriteFile(defaultConfig, []byte("{}"), 0o644); err != nil {
				return err
			}
		}
	}

	if err := os.Symlink(p.RelativeTarget("default"), config); err != nil {
		return err
	}

	// Handle tui.json (also check for dangling symlink)
	tuiConfig := p.TUIConfigFile()
	tuiWasDangling := isSymlink(tuiConfig)
	if tuiWasDangling {
		if _, err := os.Stat(tuiConfig); err == nil {
			tuiWasDangling = false
		}
	}
	if tuiWasDangling {
		if err := os.Remove(tuiConfig); err != nil {
			return err
		}
	}

	if _, err := os.Stat(tuiConfig); err == nil && !isSymlink(tuiConfig) {
		if err := copyFile(tuiConfig, defaultTUIConfig); err != nil {
			return err
		}
		if err := os.Remove(tuiConfig); err != nil {
			return err
		}
		if err := os.Symlink(p.RelativeTargetTUI("default"), tuiConfig); err != nil {
			return err
		}
	} else if tuiWasDangling {
		// tui.json was a dangling symlink — recreate symlink pointing
		// to default profile; create empty tui.json in default if needed.
		if _, err := os.Stat(defaultTUIConfig); os.IsNotExist(err) {
			if err := os.WriteFile(defaultTUIConfig, []byte("{}"), 0o644); err != nil {
				return err
			}
		}
		if err := os.Symlink(p.RelativeTargetTUI("default"), tuiConfig); err != nil {
			return err
		}
	}

	// Write skills.yml for default profile
	if _, err := os.Stat(p.ProfileSkillsYML("default")); os.IsNotExist(err) {
		current, err := skills.ScanCurrentSkills(p)
		if err != nil {
			return err
		}
		if err := skills.WriteSkillsYML(p, "default", current); err != nil {
			return err
		}
	}

	return nil
}

// Backup 备份当前激活的 profile 配置。返回备份目录名称。
func Backup(p *paths.Paths) (string, error) {
	if err := EnsureInitialized(p); err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102_150405")
	backupName := "backup_" + timestamp
	backupDir := p.ProfileDir(backupName)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}

	currentConfig := p.ConfigFile()
	if !isSymlink(currentConfig) {
		return "", fmt.Errorf("config_file is not a symlink after init")
	}
	target, err := filepath.EvalSymlinks(currentConfig)
	if err != nil {
		return "", err
	}
	if err := copyFile(target, p.ProfileConfig(backupName)); err != nil {
		return "", err
	}

	currentTUI := p.TUIConfigFile()
	if isSymlink(currentTUI) {
		tuiTarget, err := filepath.EvalSymlinks(currentTUI)
		if err != nil {
			return "", err
		}
		if err := copyFile(tuiTarget, p.ProfileTUIConfig(backupName)); err != nil {
			return "", err
		}
	}

	if err := os.MkdirAll(p.ProfileSkills(backupName), 0o755); err != nil {
		return "", err
	}
	return backupName, nil
}

// CreateFromCurrent 从当前激活的 profile 创建新 profile。
func CreateFromCurrent(p *paths.Paths, name string) error {
	if err := EnsureInitialized(p); err != nil {
		return err
	}

	currentConfig := p.ConfigFile()
	if !isSymlink(currentConfig) {
		return fmt.Errorf("config_file is not a symlink after init")
	}
	target, err := filepath.EvalSymlinks(currentConfig)
	if err != nil {
		return err
	}

	// 读取 tui.json 内容（在 rmtree 之前，防止覆盖激活 profile 时源文件被删）
	var tuiContent *string
	if active := GetActive(p); active != "" {
		currentTUI := p.ProfileTUIConfig(active)
		if data, err := os.ReadFile(currentTUI); err == nil {
			s := string(data)
			tuiContent = &s
		}
	}

	profileDir := p.ProfileDir(name)
	if _, err := os.Stat(profileDir); err == nil {
		// 先复制到临时位置（保持元数据），再 rmtree，再 move 回来——
		// 因为当覆盖当前激活的 profile 时，源和目标是同一个文件，
		// 必须先保存内容再删除目录。
		tmpPath := filepath.Join(p.BaseDir(), ".opencode.json.tmp."+name)
		if err := copyFile(target, tmpPath); err != nil {
			return err
		}
		if err := os.RemoveAll(profileDir); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		if err := os.MkdirAll(profileDir, 0o755); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		if err := os.MkdirAll(p.ProfileSkills(name), 0o755); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		if err := os.Rename(tmpPath, p.ProfileConfig(name)); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(profileDir, 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(p.ProfileSkills(name), 0o755); err != nil {
			return err
		}
		if err := copyFile(target, p.ProfileConfig(name)); err != nil {
			return err
		}
	} else {
		return err
	}

	// 写入 tui.json（使用提前读取的内容）
	if tuiContent != nil {
		if err := os.WriteFile(p.ProfileTUIConfig(name), []byte(*tuiContent), 0o644); err != nil {
			return err
		}
	}

	// Write skills.yml for new profile
	current, err := skills.ScanCurrentSkills(p)
	if err != nil {
		return err
	}
	return skills.WriteSkillsYML(p, name, current)
}

// LoadProviders 从源配置读取 provider dict。source 为 "current" 或 profile 名。
func LoadProviders(p *paths.Paths, source string) (map[string]any, error) {
	var data map[string]any
	if source == "current" {
		config := p.ConfigFile()
		if !isSymlink(config) {
			return nil, fmt.Errorf("Current config is not a symlink")
		}
		target, err := filepath.EvalSymlinks(config)
		if err != nil {
			return nil, err
		}
		raw, err := os.ReadFile(target)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
	} else {
		configPath := p.ProfileConfig(source)
		if _, err := os.Stat(configPath); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("Source profile '%s' not found", source)
			}
			return nil, err
		}
		raw, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
	}

	providers, _ := data["provider"].(map[string]any)
	if len(providers) == 0 {
		return nil, fmt.Errorf("Source config has no providers to import")
	}
	return providers, nil
}

// CreateEmpty 创建空 profile，可选从源配置导入 provider。
// source 为空字符串表示空配置；"current" 表示从当前激活配置导入；
// 其他字符串表示从指定 profile 名称导入。
func CreateEmpty(p *paths.Paths, name, source string) error {
	if err := EnsureInitialized(p); err != nil {
		return err
	}

	profileDir := p.ProfileDir(name)
	if _, err := os.Stat(profileDir); err == nil {
		if err := os.RemoveAll(profileDir); err != nil {
			return err
		}
	}

	var providers map[string]any
	if source != "" {
		var err error
		providers, err = LoadProviders(p, source)
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(p.ProfileSkills(name), 0o755); err != nil {
		return err
	}

	if providers == nil {
		return os.WriteFile(p.ProfileConfig(name), []byte("{}"), 0o644)
	}
	data, err := json.MarshalIndent(map[string]any{"provider": providers}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.ProfileConfig(name), data, 0o644)
}

// SwitchDB 切换 symlink 指向目标 profile 并同步技能（dbPath 注入用）。
func SwitchDB(p *paths.Paths, name, dbPath string) error {
	if err := EnsureInitialized(p); err != nil {
		return err
	}

	target := p.ProfileConfig(name)
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			available := ListProfiles(p)
			return fmt.Errorf("Profile '%s' not found. Available: %s", name, pythonList(available))
		}
		return err
	}

	config := p.ConfigFile()
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		return err
	}

	if isSymlink(config) {
		if err := os.Remove(config); err != nil {
			return err
		}
	} else if _, err := os.Stat(config); err == nil {
		if err := os.Remove(config); err != nil {
			return err
		}
	}

	if err := os.Symlink(p.RelativeTarget(name), config); err != nil {
		return err
	}

	// 管理 tui.json symlink
	tuiConfig := p.TUIConfigFile()
	tuiTarget := p.ProfileTUIConfig(name)

	if _, err := os.Stat(tuiTarget); err == nil {
		if isSymlink(tuiConfig) {
			if err := os.Remove(tuiConfig); err != nil {
				return err
			}
		} else if _, err := os.Stat(tuiConfig); err == nil {
			if err := os.Remove(tuiConfig); err != nil {
				return err
			}
		}
		if err := os.Symlink(p.RelativeTargetTUI(name), tuiConfig); err != nil {
			return err
		}
	} else if isSymlink(tuiConfig) {
		if err := os.Remove(tuiConfig); err != nil {
			return err
		}
	}

	// Sync skills symlinks
	return skills.SyncSkills(p, name, dbPath)
}

// Switch 切换 symlink 指向目标 profile（使用默认 db 路径）。
func Switch(p *paths.Paths, name string) error {
	return SwitchDB(p, name, "")
}

// ListProfiles 列出所有 profile 名称。
func ListProfiles(p *paths.Paths) []string {
	entries, err := os.ReadDir(p.ProfilesDir())
	if err != nil {
		return nil
	}
	var profiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		configPath := filepath.Join(p.ProfileDir(entry.Name()), "opencode.json")
		if _, err := os.Stat(configPath); err == nil {
			profiles = append(profiles, entry.Name())
		}
	}
	sort.Strings(profiles)
	return profiles
}

// GetActive 获取当前激活的 profile 名称；非 symlink 时返回空字符串。
func GetActive(p *paths.Paths) string {
	config := p.ConfigFile()
	if !isSymlink(config) {
		return ""
	}

	target, err := filepath.EvalSymlinks(config)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(p.BaseDir(), target)
	if err != nil {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) >= 2 && parts[0] == "profiles" {
		return parts[1]
	}
	return ""
}
