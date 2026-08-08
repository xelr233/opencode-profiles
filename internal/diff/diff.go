package diff

import (
	"encoding/json"
	"fmt"
	"io"
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
	if _, err := os.Stat(p.ProfileConfig(a)); err != nil {
		return nil, fmt.Errorf("profile '%s' not found", a)
	}
	if _, err := os.Stat(p.ProfileConfig(b)); err != nil {
		return nil, fmt.Errorf("profile '%s' not found", b)
	}
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

// Render 将 Result 分节打印到 w。无变化的维度省略；
// 全部无差异时输出 No differences 行。
func Render(w io.Writer, r *Result) {
	if r.Empty() {
		fmt.Fprintf(w, "No differences between '%s' and '%s'\n", r.A, r.B)
		return
	}
	fmt.Fprintf(w, "Diff: %s -> %s\n", r.A, r.B)
	renderSection(w, "provider", r.Providers)
	renderSection(w, "mcp", r.MCP)
	renderSection(w, "plugin", r.Plugins)
	renderSection(w, "skill", r.Skills)
}

// Empty 报告四维度是否均无差异。
func (r *Result) Empty() bool {
	empty := func(c Change) bool { return len(c.Added) == 0 && len(c.Removed) == 0 }
	return empty(r.Providers) && empty(r.MCP) && empty(r.Plugins) && empty(r.Skills)
}

func renderSection(w io.Writer, name string, c Change) {
	if len(c.Added) == 0 && len(c.Removed) == 0 {
		return
	}
	fmt.Fprintf(w, "\n[%s]\n", name)
	for _, s := range c.Removed {
		fmt.Fprintf(w, "  - %s\n", s)
	}
	for _, s := range c.Added {
		fmt.Fprintf(w, "  + %s\n", s)
	}
}
