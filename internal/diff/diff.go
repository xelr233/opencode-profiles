package diff

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"opencode-profiles/internal/paths"
	"opencode-profiles/internal/skills"
)

// Change 表示某个维度的增删集合，均已排序。
type Change struct {
	Added   []string
	Removed []string
}

// Result 表示 profile A 到 B 的四维度差异。
type Result struct {
	A, B      string
	Providers Change
	MCP       Change
	Plugins   Change
	Skills    Change
}

// Diff 读取 profile a 与 b 的 opencode.json 与 skills.yml，
// 返回四维度键/项集合差异。
func Diff(p *paths.Paths, a, b string) (*Result, error) {
	cfgA, err := readConfig(p.ProfileConfig(a))
	if err != nil {
		return nil, err
	}
	cfgB, err := readConfig(p.ProfileConfig(b))
	if err != nil {
		return nil, err
	}
	skillsA, err := skills.ReadSkillsYML(p, a)
	if err != nil {
		return nil, err
	}
	skillsB, err := skills.ReadSkillsYML(p, b)
	if err != nil {
		return nil, err
	}
	return &Result{
		A:         a,
		B:         b,
		Providers: setDiff(mapKeys(cfgA["provider"]), mapKeys(cfgB["provider"])),
		MCP:       setDiff(mapKeys(cfgA["mcp"]), mapKeys(cfgB["mcp"])),
		Plugins:   setDiff(listItems(cfgA["plugin"]), listItems(cfgB["plugin"])),
		Skills:    setDiff(skillsA, skillsB),
	}, nil
}

func readConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

func mapKeys(v any) []string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func listItems(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// setDiff 返回 target 相对 current 的 (新增, 移除)，均排序。
func setDiff(current, target []string) Change {
	cur := make(map[string]struct{}, len(current))
	for _, s := range current {
		cur[s] = struct{}{}
	}
	tgt := make(map[string]struct{}, len(target))
	for _, s := range target {
		tgt[s] = struct{}{}
	}
	var c Change
	for s := range tgt {
		if _, ok := cur[s]; !ok {
			c.Added = append(c.Added, s)
		}
	}
	for s := range cur {
		if _, ok := tgt[s]; !ok {
			c.Removed = append(c.Removed, s)
		}
	}
	sort.Strings(c.Added)
	sort.Strings(c.Removed)
	return c
}
