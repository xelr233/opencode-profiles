package ops

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"opencode-profiles/internal/paths"
	"opencode-profiles/internal/skills"
)

func sampleConfig() map[string]any {
	return map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"provider": map[string]any{
			"test": map[string]any{
				"name":    "Test",
				"npm":     "@ai-sdk/openai-compatible",
				"options": map[string]any{"apiKey": "test-key", "baseURL": "https://test.example.com/v1"},
			},
		},
		"shell": "bash",
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJSONFileRaw(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readConfig(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func newEnv(t *testing.T) (*paths.Paths, string) {
	t.Helper()
	src := filepath.Join(t.TempDir(), "skill-sources")
	for _, name := range []string{"brainstorming", "rtk", "mavenbuild"} {
		dir := filepath.Join(src, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	p := paths.New(base, src)
	writeJSON(t, p.ConfigFile(), sampleConfig())
	return p, filepath.Join(t.TempDir(), "nonexistent.db")
}

func TestInitBasics(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.ProfileConfig("default")); err != nil {
		t.Fatalf("default config missing: %v", err)
	}
	if !isSymlink(p.ConfigFile()) {
		t.Fatal("config_file should be a symlink")
	}
	target, _ := filepath.EvalSymlinks(p.ConfigFile())
	want, _ := filepath.EvalSymlinks(p.ProfileConfig("default"))
	if target != want {
		t.Fatalf("symlink target %q != %q", target, want)
	}
	bak := filepath.Join(p.BaseDir(), "opencode.json.bak")
	if _, err := os.Stat(bak); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if readConfig(t, bak)["shell"] != "bash" {
		t.Fatal("backup content mismatch")
	}
	if fi, err := os.Stat(p.ProfileSkills("default")); err != nil || !fi.IsDir() {
		t.Fatal("default skills dir missing")
	}
	if _, err := os.Stat(p.ProfileSkillsYML("default")); err != nil {
		t.Fatal("default skills.yml missing")
	}
	// idempotent
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if !isSymlink(p.ConfigFile()) {
		t.Fatal("still should be a symlink")
	}
}

func TestInitEmptyConfig(t *testing.T) {
	base := filepath.Join(t.TempDir(), "opencode")
	p := paths.New(base, "")
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if got := readConfig(t, p.ProfileConfig("default")); len(got) != 0 {
		t.Fatalf("default config = %v, want {}", got)
	}
}

func TestInitTUI(t *testing.T) {
	base := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	p := paths.New(base, "")
	writeJSON(t, p.ConfigFile(), sampleConfig())
	writeJSON(t, p.TUIConfigFile(), map[string]any{"theme": "dark", "fontSize": 14})

	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if !isSymlink(p.TUIConfigFile()) {
		t.Fatal("tui.json should be a symlink")
	}
	target, _ := filepath.EvalSymlinks(p.TUIConfigFile())
	want, _ := filepath.EvalSymlinks(p.ProfileTUIConfig("default"))
	if target != want {
		t.Fatalf("tui target %q != %q", target, want)
	}
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if !isSymlink(p.TUIConfigFile()) {
		t.Fatal("tui.json should stay symlink")
	}
}

func TestInitNoTUI(t *testing.T) {
	base := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	p := paths.New(base, "")
	writeJSON(t, p.ConfigFile(), sampleConfig())
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.TUIConfigFile()); !os.IsNotExist(err) {
		t.Fatal("tui.json should not exist")
	}
}

func TestBackup(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	name, err := Backup(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "backup_") {
		t.Fatalf("name = %q", name)
	}
	if readConfig(t, p.ProfileConfig(name))["shell"] != "bash" {
		t.Fatal("backup content mismatch")
	}
	if fi, err := os.Stat(p.ProfileSkills(name)); err != nil || !fi.IsDir() {
		t.Fatal("backup skills dir missing")
	}
}

func TestBackupWithTUI(t *testing.T) {
	base := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	p := paths.New(base, "")
	writeJSON(t, p.ConfigFile(), sampleConfig())
	writeJSON(t, p.TUIConfigFile(), map[string]any{"theme": "dark"})
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	name, err := Backup(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.ProfileTUIConfig(name)); err != nil {
		t.Fatal("backup tui missing")
	}
}

func TestCreateFromCurrent(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateFromCurrent(p, "work"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, name := range ListProfiles(p) {
		if name == "work" {
			found = true
		}
	}
	if !found {
		t.Fatal("work not listed")
	}
	if readConfig(t, p.ProfileConfig("work"))["shell"] != "bash" {
		t.Fatal("work config mismatch")
	}
	if fi, err := os.Stat(p.ProfileSkills("work")); err != nil || !fi.IsDir() {
		t.Fatal("work skills dir missing")
	}
}

func TestCreateFromCurrentWithTUI(t *testing.T) {
	base := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	p := paths.New(base, "")
	writeJSON(t, p.ConfigFile(), sampleConfig())
	writeJSON(t, p.TUIConfigFile(), map[string]any{"theme": "dark"})
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateFromCurrent(p, "work"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.ProfileTUIConfig("work")); err != nil {
		t.Fatal("work tui missing")
	}
}

func TestCreateEmpty(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateEmpty(p, "empty", ""); err != nil {
		t.Fatal(err)
	}
	if got := readConfig(t, p.ProfileConfig("empty")); len(got) != 0 {
		t.Fatalf("empty = %v", got)
	}
}

func TestSwitchAndActive(t *testing.T) {
	p, db := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if GetActive(p) != "default" {
		t.Fatalf("active = %q", GetActive(p))
	}
	if err := CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	if err := SwitchDB(p, "work", db, io.Discard); err != nil {
		t.Fatal(err)
	}
	if GetActive(p) != "work" {
		t.Fatalf("active = %q", GetActive(p))
	}
	if err := SwitchDB(p, "nonexistent", db, io.Discard); err == nil {
		t.Fatal("expected not found error")
	}
}

func TestSwitchWithTUI(t *testing.T) {
	base := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	p := paths.New(base, "")
	writeJSON(t, p.ConfigFile(), sampleConfig())
	writeJSON(t, p.TUIConfigFile(), map[string]any{"theme": "dark"})
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateFromCurrent(p, "work"); err != nil {
		t.Fatal(err)
	}
	if err := SwitchDB(p, "work", filepath.Join(t.TempDir(), "nonexistent.db"), io.Discard); err != nil {
		t.Fatal(err)
	}
	if !isSymlink(p.TUIConfigFile()) {
		t.Fatal("tui should be symlink")
	}
	target, _ := filepath.EvalSymlinks(p.TUIConfigFile())
	want, _ := filepath.EvalSymlinks(p.ProfileTUIConfig("work"))
	if target != want {
		t.Fatalf("tui target %q != %q", target, want)
	}
}

func TestListProfilesEmpty(t *testing.T) {
	p, _ := newEnv(t)
	if got := ListProfiles(p); len(got) != 0 {
		t.Fatalf("empty list = %v", got)
	}
}

func TestGetActiveNotSymlink(t *testing.T) {
	p, _ := newEnv(t)
	if GetActive(p) != "" {
		t.Fatal("should be empty")
	}
}

func TestCreateEmptyWithSource(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateEmpty(p, "work", "current"); err != nil {
		t.Fatal(err)
	}
	got := readConfig(t, p.ProfileConfig("work"))
	prov, ok := got["provider"].(map[string]any)
	if !ok || prov["test"] == nil {
		t.Fatalf("work providers = %v", got)
	}
	if _, err := os.Stat(p.ProfileTUIConfig("work")); !os.IsNotExist(err) {
		t.Fatal("create_empty should not import tui")
	}
	if s, err := skills.ReadSkillsYML(p, "work"); err != nil || len(s) != 0 {
		t.Fatalf("create_empty should not import skills: %v", s)
	}
}

func TestCreateEmptySourceErrors(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateEmpty(p, "work", "nonexistent"); err == nil {
		t.Fatal("expected not found error")
	}
	if err := os.MkdirAll(p.ProfileDir("no_provider"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, p.ProfileConfig("no_provider"), map[string]any{"shell": "bash"})
	if err := os.MkdirAll(p.ProfileSkills("no_provider"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := CreateEmpty(p, "work", "no_provider")
	if err == nil || !strings.Contains(err.Error(), "no providers") {
		t.Fatalf("expected no-providers error, got %v", err)
	}
}

// redirectStderr 将 os.Stderr 重定向到临时文件；返回恢复 os.Stderr 并返回捕获内容
// 的函数（包内测试串行执行，无并行风险）。
func redirectStderr(t *testing.T) func() string {
	t.Helper()
	old := os.Stderr
	f, err := os.Create(filepath.Join(t.TempDir(), "stderr.txt"))
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = f
	return func() string {
		os.Stderr = old
		f.Close()
		data, err := os.ReadFile(f.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
}

func TestCreateEmptyWarnsGitHistory(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(p.ProfileDir("work"), ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := redirectStderr(t)
	if err := CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	got := restore()
	if !strings.Contains(got, "has git history that will be deleted") {
		t.Fatalf("expected git-history warning, got %q", got)
	}
}

func TestCreateFromCurrentWarnsGitHistory(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateFromCurrent(p, "work"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(p.ProfileDir("work"), ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := redirectStderr(t)
	if err := CreateFromCurrent(p, "work"); err != nil {
		t.Fatal(err)
	}
	got := restore()
	if !strings.Contains(got, "has git history that will be deleted") {
		t.Fatalf("expected git-history warning, got %q", got)
	}
}

func TestCreateOverwriteExisting(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateFromCurrent(p, "work"); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(p.ProfileConfig("work"))

	active := GetActive(p)
	if active == "" {
		t.Fatal("expected active profile")
	}
	current, _ := filepath.EvalSymlinks(p.ConfigFile())
	cfg := readConfig(t, current)
	prov := cfg["provider"].(map[string]any)
	prov["new_provider"] = map[string]any{"name": "New"}
	writeJSON(t, current, cfg)

	if err := CreateFromCurrent(p, "work"); err != nil {
		t.Fatal(err)
	}
	now, _ := os.ReadFile(p.ProfileConfig("work"))
	if string(now) == string(original) {
		t.Fatal("work config should be overwritten")
	}

	if err := CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	if got := readConfig(t, p.ProfileConfig("work")); len(got) != 0 {
		t.Fatalf("empty overwrite = %v", got)
	}
}

func TestRmRFThenCreateDefault(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(p.ProfilesDir())
	for _, e := range entries {
		if e.IsDir() {
			if err := os.RemoveAll(filepath.Join(p.ProfilesDir(), e.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := CreateFromCurrent(p, "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.ProfileConfig("default")); err != nil {
		t.Fatal(err)
	}
	if !isSymlink(p.ConfigFile()) {
		t.Fatal("config should be symlink")
	}
	if _, err := os.Stat(p.ConfigFile()); err != nil {
		t.Fatal("config symlink should resolve")
	}
	if readConfig(t, p.ProfileConfig("default"))["shell"] != "bash" {
		t.Fatal("content not preserved")
	}
}

func TestDanglingSymlinkRecovery(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(p.ProfilesDir())
	for _, e := range entries {
		if e.IsDir() {
			if err := os.RemoveAll(filepath.Join(p.ProfilesDir(), e.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !isSymlink(p.ConfigFile()) {
		t.Fatal("config should be dangling symlink now")
	}
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.ProfileConfig("default")); err != nil {
		t.Fatal("default restored")
	}
	if _, err := os.Stat(p.ConfigFile()); err != nil {
		t.Fatal("symlink resolves after recovery")
	}
}

func TestNoBakCreatesEmptyConfig(t *testing.T) {
	base := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	p := paths.New(base, "")
	if err := os.Symlink(filepath.Join("profiles", "default", "opencode.json"), p.ConfigFile()); err != nil {
		t.Fatal(err)
	}
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p.ProfileConfig("default"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{}" {
		t.Fatalf("default = %q", string(data))
	}
}

func TestCreateFromCurrentSyncsSkills(t *testing.T) {
	p, _ := newEnv(t)
	skillsDir := filepath.Join(p.BaseDir(), "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(p.SkillSource("rtk"), filepath.Join(skillsDir, "rtk")); err != nil {
		t.Fatal(err)
	}
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateFromCurrent(p, "work"); err != nil {
		t.Fatal(err)
	}
	got, _ := skills.ReadSkillsYML(p, "work")
	if len(got) != 1 || got[0] != "rtk" {
		t.Fatalf("work skills = %v", got)
	}
}

func TestSwitchSyncsSkills(t *testing.T) {
	p, db := newEnv(t)
	skillsDir := filepath.Join(p.BaseDir(), "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(p.SkillSource("rtk"), filepath.Join(skillsDir, "rtk")); err != nil {
		t.Fatal(err)
	}
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	if err := skills.AddSkill(p, "work", "mavenbuild"); err != nil {
		t.Fatal(err)
	}
	if err := skills.RemoveSkill(p, "work", "rtk"); err != nil {
		t.Fatal(err)
	}
	if err := SwitchDB(p, "work", db, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(skillsDir, "mavenbuild")); err != nil {
		t.Fatal("mavenbuild symlink missing")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "rtk")); !os.IsNotExist(err) {
		t.Fatal("rtk should be removed")
	}
}

func TestSwitchPrintsDiff(t *testing.T) {
	p, db := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, p.ProfileConfig("work"), map[string]any{"provider": map[string]any{"deepseek": map[string]any{}}})
	if err := skills.AddSkill(p, "work", "mavenbuild"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := SwitchDB(p, "work", db, &buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "Diff: default -> work") {
		t.Fatalf("missing diff header: %q", got)
	}
	if !strings.Contains(got, "[provider]") || !strings.Contains(got, "  + deepseek") {
		t.Fatalf("provider section missing: %q", got)
	}
	if !strings.Contains(got, "[skill]") || !strings.Contains(got, "  + mavenbuild") {
		t.Fatalf("skill section missing: %q", got)
	}
}

func TestSwitchWarnAndContinue(t *testing.T) {
	p, db := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	writeJSONFileRaw(t, p.ProfileConfig("work"), "{ not json")
	var buf bytes.Buffer
	if err := SwitchDB(p, "work", db, &buf); err != nil {
		t.Fatalf("switch should continue despite malformed config: %v", err)
	}
	if !strings.Contains(buf.String(), "Warning: could not diff profiles") {
		t.Fatalf("missing warning: %q", buf.String())
	}
	if GetActive(p) != "work" {
		t.Fatalf("active = %q", GetActive(p))
	}
}

func TestSwitchCompatibleNoOut(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	if err := Switch(p, "work"); err != nil {
		t.Fatal(err)
	}
	if GetActive(p) != "work" {
		t.Fatalf("active = %q", GetActive(p))
	}
}

func TestEnsureInitializedExistingSetup(t *testing.T) {
	p, _ := newEnv(t)
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p.ProfileSkillsYML("default")); err != nil {
		t.Fatal(err)
	}
	skillsDir := filepath.Join(p.BaseDir(), "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(p.SkillSource("rtk"), filepath.Join(skillsDir, "rtk")); err != nil {
		t.Fatal(err)
	}
	if err := EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	got, _ := skills.ReadSkillsYML(p, "default")
	if len(got) != 1 || got[0] != "rtk" {
		t.Fatalf("default skills = %v", got)
	}
}
