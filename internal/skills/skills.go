package skills

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite"

	"gopkg.in/yaml.v3"

	"opencode-profiles/internal/paths"
)

// ReadSkillsYML 读取 profile 的 skills.yml。文件不存在时返回 nil。
func ReadSkillsYML(p *paths.Paths, name string) ([]string, error) {
	ymlPath := p.ProfileSkillsYML(name)
	data, err := os.ReadFile(ymlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var skills []string
	if err := yaml.Unmarshal(data, &skills); err != nil {
		return nil, err
	}
	return skills, nil
}

// WriteSkillsYML 将技能列表写入 profile 的 skills.yml（block 风格逐行 `- name`）。
func WriteSkillsYML(p *paths.Paths, name string, skills []string) error {
	ymlPath := p.ProfileSkillsYML(name)
	if err := os.MkdirAll(filepath.Dir(ymlPath), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(skills)
	if err != nil {
		return err
	}
	return os.WriteFile(ymlPath, data, 0o644)
}

// ScanCurrentSkills 扫描 base/skills 下指向 skill_sources_dir 的 symlink，按名字排序。
func ScanCurrentSkills(p *paths.Paths) ([]string, error) {
	skillsDir := filepath.Join(p.BaseDir(), "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []string
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		link := filepath.Join(skillsDir, entry.Name())
		resolved, err := filepath.EvalSymlinks(link)
		if err != nil {
			continue
		}
		if filepath.Dir(resolved) == p.SkillSourcesDir() {
			result = append(result, entry.Name())
		}
	}
	sort.Strings(result)
	return result, nil
}

// ComputeDiff 基于集合差返回 (toAdd, toRemove)，均排序。
func ComputeDiff(current, target []string) (toAdd, toRemove []string) {
	cur := make(map[string]struct{}, len(current))
	for _, s := range current {
		cur[s] = struct{}{}
	}
	tgt := make(map[string]struct{}, len(target))
	for _, s := range target {
		tgt[s] = struct{}{}
	}
	for s := range tgt {
		if _, ok := cur[s]; !ok {
			toAdd = append(toAdd, s)
		}
	}
	for s := range cur {
		if _, ok := tgt[s]; !ok {
			toRemove = append(toRemove, s)
		}
	}
	sort.Strings(toAdd)
	sort.Strings(toRemove)
	return toAdd, toRemove
}

// AddSkill 向 profile 的 skills.yml 添加技能。源不存在时返回错误。
func AddSkill(p *paths.Paths, name, skill string) error {
	source := p.SkillSource(skill)
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Skill source '%s' not found at %s", skill, source)
		}
		return err
	}
	skills, err := ReadSkillsYML(p, name)
	if err != nil {
		return err
	}
	found := false
	for _, s := range skills {
		if s == skill {
			found = true
			break
		}
	}
	if !found {
		skills = append(skills, skill)
	}
	return WriteSkillsYML(p, name, skills)
}

// RemoveSkill 从 profile 的 skills.yml 移除技能。
func RemoveSkill(p *paths.Paths, name, skill string) error {
	skills, err := ReadSkillsYML(p, name)
	if err != nil {
		return err
	}
	out := skills[:0]
	for _, s := range skills {
		if s != skill {
			out = append(out, s)
		}
	}
	return WriteSkillsYML(p, name, out)
}

// SyncSkills 同步 base/skills symlink 使其匹配目标 profile 的 skills.yml。
// 先校验所有目标源存在再修改，成功后更新 cc-switch.db。
func SyncSkills(p *paths.Paths, targetName, dbPath string) error {
	targetSkills, err := ReadSkillsYML(p, targetName)
	if err != nil {
		return err
	}

	skillsDir := filepath.Join(p.BaseDir(), "skills")
	var currentSkills []string
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, entry := range entries {
			currentSkills = append(currentSkills, entry.Name())
		}
		sort.Strings(currentSkills)
	}

	for _, skill := range targetSkills {
		if _, err := os.Stat(p.SkillSource(skill)); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("Skill source '%s' not found at %s", skill, p.SkillSource(skill))
			}
			return err
		}
	}

	toAdd, toRemove := ComputeDiff(currentSkills, targetSkills)

	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return err
	}

	for _, skill := range toRemove {
		link := filepath.Join(skillsDir, skill)
		if err := removeEntry(link); err != nil {
			return err
		}
	}

	for _, skill := range toAdd {
		link := filepath.Join(skillsDir, skill)
		if err := removeEntry(link); err != nil {
			return err
		}
		if err := os.Symlink(p.SkillSource(skill), link); err != nil {
			return err
		}
	}

	return UpdateDB(dbPath, targetSkills)
}

// removeEntry 移除 entry：symlink 或文件用 os.Remove，目录用 os.RemoveAll。
func removeEntry(link string) error {
	fi, err := os.Lstat(link)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.IsDir() && fi.Mode()&os.ModeSymlink == 0 {
		return os.RemoveAll(link)
	}
	return os.Remove(link)
}

// UpdateDB 基于激活技能更新 cc-switch.db 的 enabled_opencode。
// db 不存在或发生任何 sqlite 错误时静默忽略。
func UpdateDB(dbPath string, activeSkills []string) error {
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".cc-switch", "cc-switch.db")
	}
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return nil
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()
	if _, err := db.Exec("UPDATE skills SET enabled_opencode = 0"); err != nil {
		return nil
	}
	stmt, err := db.Prepare("UPDATE skills SET enabled_opencode = 1 WHERE name = ?")
	if err != nil {
		return nil
	}
	defer stmt.Close()
	for _, skill := range activeSkills {
		stmt.Exec(skill)
	}
	return nil
}
