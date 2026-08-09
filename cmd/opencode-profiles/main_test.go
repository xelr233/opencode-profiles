package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"opencode-profiles/internal/ops"
	"opencode-profiles/internal/paths"
	"opencode-profiles/internal/skills"
)

type cliResult struct {
	code   int
	stdout string
	stderr string
}

func newCLIEnv(t *testing.T) (*paths.Paths, string, *bytes.Buffer, *bytes.Buffer) {
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
	writeJSONFile(t, p.ConfigFile(), `{"shell": "bash"}`)
	db := filepath.Join(t.TempDir(), "nonexistent.db")
	var out, errOut bytes.Buffer
	return p, db, &out, &errOut
}

func writeJSONFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func invoke(t *testing.T, p *paths.Paths, db string, out, errOut *bytes.Buffer, args ...string) cliResult {
	t.Helper()
	code := run(args, out, errOut, p, db)
	return cliResult{code: code, stdout: out.String(), stderr: errOut.String()}
}

func TestListCommand(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	if err := ops.EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	res := invoke(t, p, db, out, errOut, "-l")
	if res.code != 0 || !strings.Contains(res.stdout, "default") {
		t.Fatalf("code=%d stdout=%q", res.code, res.stdout)
	}
	if !strings.Contains(res.stdout, "Active: default") {
		t.Fatalf("missing active marker: %q", res.stdout)
	}
}

func TestCreateCommand(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "-c", "work")
	if res.code != 0 || !strings.Contains(res.stdout, "Created profile 'work' from current config") {
		t.Fatalf("code=%d stdout=%q", res.code, res.stdout)
	}
}

func TestCreateEmptyCommand(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "-e", "minimal")
	if res.code != 0 || !strings.Contains(res.stdout, "Created empty profile 'minimal'") {
		t.Fatalf("code=%d stdout=%q", res.code, res.stdout)
	}
}

func TestSwitchCommand(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	if res := invoke(t, p, db, out, errOut, "-c", "work"); res.code != 0 {
		t.Fatalf("create failed: %q", res.stderr)
	}
	out.Reset()
	res := invoke(t, p, db, out, errOut, "-s", "work")
	if res.code != 0 || !strings.Contains(res.stdout, "Switched to 'work'") {
		t.Fatalf("code=%d stdout=%q stderr=%q", res.code, res.stdout, res.stderr)
	}
	// 同内容 profile：应打印 No differences 行
	if !strings.Contains(res.stdout, "No differences between 'default' and 'work'") {
		t.Fatalf("missing no-diff line: %q", res.stdout)
	}
}

func TestDiffCommand(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	if err := ops.EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := ops.CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, p.ProfileConfig("work"), `{"provider": {"deepseek": {}}}`)
	res := invoke(t, p, db, out, errOut, "-d", "work")
	if res.code != 0 {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "Diff: default -> work") {
		t.Fatalf("stdout=%q", res.stdout)
	}
	if !strings.Contains(res.stdout, "[provider]") || !strings.Contains(res.stdout, "  + deepseek") {
		t.Fatalf("stdout=%q", res.stdout)
	}
}

func TestDiffTwoArgs(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	if err := ops.EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := ops.CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, p.ProfileConfig("work"), `{"mcp": {"srv": {}}}`)
	if err := ops.CreateEmpty(p, "personal", ""); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, p.ProfileConfig("personal"), `{"mcp": {"srv": {}, "extra": {}}}`)
	res := invoke(t, p, db, out, errOut, "-d", "work", "personal")
	if res.code != 0 {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "Diff: work -> personal") {
		t.Fatalf("stdout=%q", res.stdout)
	}
	if !strings.Contains(res.stdout, "  + extra") {
		t.Fatalf("stdout=%q", res.stdout)
	}
}

func TestDiffNoArgs(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "-d")
	if res.code == 0 || !strings.Contains(res.stderr, "requires at least one profile name") {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
}

func TestDiffTooManyArgs(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "-d", "a", "b", "c")
	if res.code == 0 || !strings.Contains(res.stderr, "accepts at most two profile names") {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
}

func TestDiffMutuallyExclusive(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "-d", "work", "-s", "work")
	if res.code == 0 || !strings.Contains(res.stderr, "cannot be combined") {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
}

func TestBackupCommand(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "-b")
	if res.code != 0 || !strings.Contains(res.stdout, "Backed up to 'backup_") {
		t.Fatalf("code=%d stdout=%q", res.code, res.stdout)
	}
}

func TestEmptyFromCurrent(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	writeJSONFile(t, p.ConfigFile(), `{"provider": {"test": {"name": "Test"}}}`)
	res := invoke(t, p, db, out, errOut, "-e", "work", "--from-current")
	if res.code != 0 || !strings.Contains(res.stdout, "with providers from current config") {
		t.Fatalf("code=%d stdout=%q stderr=%q", res.code, res.stdout, res.stderr)
	}
	var cfg map[string]any
	raw, _ := os.ReadFile(p.ProfileConfig("work"))
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["provider"] == nil {
		t.Fatal("providers not imported")
	}
}

func TestEmptyFromProfile(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	writeJSONFile(t, p.ConfigFile(), `{"provider": {"test": {"name": "Test"}}}`)
	if res := invoke(t, p, db, out, errOut, "-c", "personal"); res.code != 0 {
		t.Fatalf("create personal failed: %q", res.stderr)
	}
	out.Reset()
	res := invoke(t, p, db, out, errOut, "-e", "work", "--from-profile", "personal")
	if res.code != 0 || !strings.Contains(res.stdout, "with providers from 'personal'") {
		t.Fatalf("code=%d stdout=%q stderr=%q", res.code, res.stdout, res.stderr)
	}
}

func TestFromCurrentWithoutEmpty(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "--from-current")
	if res.code == 0 || !strings.Contains(res.stderr, "can only be used with -e") {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
}

func TestFromProfileWithoutEmpty(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "--from-profile", "personal")
	if res.code == 0 || !strings.Contains(res.stderr, "can only be used with -e") {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
}

func TestMutuallyExclusive(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "-e", "work", "--from-current", "--from-profile", "x")
	if res.code == 0 || !strings.Contains(res.stderr, "mutually exclusive") {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
}

func TestEmptyFromCurrentNoProvider(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	if err := ops.EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p.ProfileDir("no_prov"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, p.ProfileConfig("no_prov"), `{"shell": "bash"}`)
	if err := os.MkdirAll(p.ProfileSkills("no_prov"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p.ConfigFile()); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(p.RelativeTarget("no_prov"), p.ConfigFile()); err != nil {
		t.Fatal(err)
	}
	res := invoke(t, p, db, out, errOut, "-e", "work", "--from-current")
	if res.code == 0 || !strings.Contains(strings.ToLower(res.stderr), "no providers") {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
}

func TestAddSkillCommand(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	if err := ops.EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := ops.CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	res := invoke(t, p, db, out, errOut, "--add-skill", "rtk", "--profile", "work")
	if res.code != 0 || !strings.Contains(res.stdout, "Added skill 'rtk' to profile 'work'") {
		t.Fatalf("code=%d stdout=%q stderr=%q", res.code, res.stdout, res.stderr)
	}
	got, _ := skills.ReadSkillsYML(p, "work")
	if len(got) != 1 || got[0] != "rtk" {
		t.Fatalf("skills = %v", got)
	}
}

func TestRemoveSkillCommand(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	if err := ops.EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := ops.CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	if err := skills.WriteSkillsYML(p, "work", []string{"rtk"}); err != nil {
		t.Fatal(err)
	}
	res := invoke(t, p, db, out, errOut, "--remove-skill", "rtk", "--profile", "work")
	if res.code != 0 || !strings.Contains(res.stdout, "Removed skill 'rtk' from profile 'work'") {
		t.Fatalf("code=%d stdout=%q stderr=%q", res.code, res.stdout, res.stderr)
	}
	got, _ := skills.ReadSkillsYML(p, "work")
	if len(got) != 0 {
		t.Fatalf("skills = %v", got)
	}
}

func TestAddSkillRequiresProfile(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "--add-skill", "rtk")
	if res.code == 0 || !strings.Contains(res.stderr, "requires --profile") {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
}

func TestRemoveSkillRequiresProfile(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "--remove-skill", "rtk")
	if res.code == 0 || !strings.Contains(res.stderr, "requires --profile") {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
}

func TestNoArgs(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut)
	if res.code != 0 || !strings.Contains(res.stdout, "Use --help for available commands.") {
		t.Fatalf("code=%d stdout=%q", res.code, res.stdout)
	}
}

func TestGitInitCommand(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	p, db, out, errOut := newCLIEnv(t)
	invoke(t, p, db, out, errOut, "-e", "work")
	res := invoke(t, p, db, out, errOut, "--git-init", "work")
	if res.code != 0 || !strings.Contains(res.stdout, "Version control enabled for 'work'") {
		t.Fatalf("code=%d stdout=%q stderr=%q", res.code, res.stdout, res.stderr)
	}
}

func TestGitInitRejectsExisting(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	p, db, out, errOut := newCLIEnv(t)
	invoke(t, p, db, out, errOut, "-e", "work")
	invoke(t, p, db, out, errOut, "--git-init", "work")
	res := invoke(t, p, db, out, errOut, "--git-init", "work")
	if res.code != 1 || !strings.Contains(res.stderr, "already under version control") {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
}

func TestGitCommitAndLogCommand(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	p, db, out, errOut := newCLIEnv(t)
	invoke(t, p, db, out, errOut, "-e", "work")
	invoke(t, p, db, out, errOut, "--git-init", "work")
	writeJSONFile(t, p.ProfileConfig("work"), `{"shell":"zsh"}`)
	res := invoke(t, p, db, out, errOut, "--git-commit", "work", "-m", "update")
	if res.code != 0 || !strings.Contains(res.stdout, "Committed changes for 'work'") {
		t.Fatalf("code=%d stdout=%q stderr=%q", res.code, res.stdout, res.stderr)
	}
	res = invoke(t, p, db, out, errOut, "--git-log", "work")
	if res.code != 0 || !strings.Contains(res.stdout, "update") {
		t.Fatalf("code=%d stdout=%q", res.code, res.stdout)
	}
}

func TestGitRollbackCommand(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	p, db, out, errOut := newCLIEnv(t)
	invoke(t, p, db, out, errOut, "-e", "work")
	invoke(t, p, db, out, errOut, "--git-init", "work")
	writeJSONFile(t, p.ProfileConfig("work"), `{"shell":"zsh"}`)
	invoke(t, p, db, out, errOut, "--git-commit", "work", "-m", "first")
	res := invoke(t, p, db, out, errOut, "--git-rollback", "work", "HEAD~1")
	if res.code != 0 || !strings.Contains(res.stdout, "Rolled back 'work' to HEAD~1") {
		t.Fatalf("code=%d stdout=%q stderr=%q", res.code, res.stdout, res.stderr)
	}
	got, _ := os.ReadFile(p.ProfileConfig("work"))
	if string(got) != `{}` {
		t.Fatalf("expected rollback to empty config, got %q", got)
	}
}

func TestGitCommandWithoutGitInstalled(t *testing.T) {
	if gitAvailable() {
		t.Skip("git installed; this test needs a machine without git")
	}
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "--git-init", "work")
	if res.code != 1 || !strings.Contains(res.stderr, "git is not installed") {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
}

func TestGitCommandMutualExclusion(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "--git-init", "work", "-l")
	if res.code != 1 || !strings.Contains(res.stderr, "cannot be combined") {
		t.Fatalf("code=%d stderr=%q", res.code, res.stderr)
	}
}

// gitAvailable 供 main_test.go 内的跳过判断。
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}
