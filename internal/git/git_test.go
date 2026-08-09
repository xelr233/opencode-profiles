package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"opencode-profiles/internal/paths"
)

func makeRepo(t *testing.T) (*paths.Paths, string) {
	t.Helper()
	p := paths.New(filepath.Join(t.TempDir(), "opencode"), filepath.Join(t.TempDir(), "skills"))
	dir := p.ProfileDir("work")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return p, "work"
}

func TestIsRepoFalse(t *testing.T) {
	if !Available() {
		t.Skip("git not installed")
	}
	p, name := makeRepo(t)
	if IsRepo(p, name) {
		t.Fatal("expected no repo before git init")
	}
}

func TestIsRepoTrue(t *testing.T) {
	if !Available() {
		t.Skip("git not installed")
	}
	p, name := makeRepo(t)
	if _, _, err := run(p, name, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if !IsRepo(p, name) {
		t.Fatal("expected repo after git init")
	}
}

func TestInitCreatesRepoAndFirstCommit(t *testing.T) {
	if !Available() {
		t.Skip("git not installed")
	}
	p, name := makeRepo(t)
	writeProfileFiles(t, p, name, `{"shell":"bash"}`)

	if err := Init(p, name); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if !IsRepo(p, name) {
		t.Fatal("expected .git after Init")
	}
	log, _, err := run(p, name, "log", "--oneline")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log, "initial") {
		t.Fatalf("expected initial commit, got %q", log)
	}
	gitignore := filepath.Join(p.ProfileDir(name), ".gitignore")
	data, err := os.ReadFile(gitignore)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "skills/\n" {
		t.Fatalf("unexpected .gitignore content: %q", data)
	}
	files, _, err := run(p, name, "ls-files")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(files, "skills/dummy") {
		t.Fatalf("expected skills/dummy to be ignored, got %q", files)
	}
	if !strings.Contains(files, ".gitignore") {
		t.Fatalf("expected .gitignore to be tracked, got %q", files)
	}
}

func TestInitRejectsExistingRepo(t *testing.T) {
	if !Available() {
		t.Skip("git not installed")
	}
	p, name := makeRepo(t)
	writeProfileFiles(t, p, name, `{"shell":"bash"}`)
	if err := Init(p, name); err != nil {
		t.Fatal(err)
	}
	if err := Init(p, name); err == nil {
		t.Fatal("expected error when repo already initialized")
	}
}

// writeProfileFiles 写入三个被跟踪文件。
func writeProfileFiles(t *testing.T, p *paths.Paths, name, config string) {
	t.Helper()
	if err := os.WriteFile(p.ProfileConfig(name), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfileTUIConfig(name), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfileSkillsYML(name), []byte("- rtk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(p.ProfileDir(name), "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.ProfileDir(name), "skills", "dummy"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCommitAndLog(t *testing.T) {
	if !Available() {
		t.Skip("git not installed")
	}
	p, name := makeRepo(t)
	writeProfileFiles(t, p, name, `{"shell":"bash"}`)
	if err := Init(p, name); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(p.ProfileConfig(name), []byte(`{"shell":"zsh"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Commit(p, name, "switch shell"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	log, err := Log(p, name)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(log), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 commits, got %d: %q", len(lines), log)
	}
	if !strings.Contains(log, "switch shell") {
		t.Fatalf("missing commit message: %q", log)
	}
}

func TestCommitOnUninitializedRepo(t *testing.T) {
	if !Available() {
		t.Skip("git not installed")
	}
	p, name := makeRepo(t)
	writeProfileFiles(t, p, name, `{"shell":"bash"}`)
	if err := Commit(p, name, "msg"); err == nil {
		t.Fatal("expected error committing to uninitialized repo")
	}
}

func TestLogOnUninitializedRepo(t *testing.T) {
	if !Available() {
		t.Skip("git not installed")
	}
	p, name := makeRepo(t)
	if _, err := Log(p, name); err == nil {
		t.Fatal("expected error logging uninitialized repo")
	}
}

func TestRollbackRestoresFileKeepsHistory(t *testing.T) {
	if !Available() {
		t.Skip("git not installed")
	}
	p, name := makeRepo(t)
	writeProfileFiles(t, p, name, `{"shell":"bash"}`)
	if err := Init(p, name); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfileConfig(name), []byte(`{"shell":"zsh"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Commit(p, name, "switch shell"); err != nil {
		t.Fatal(err)
	}

	if err := Rollback(p, name, "HEAD~1"); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
	data, err := os.ReadFile(p.ProfileConfig(name))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"shell":"bash"}` {
		t.Fatalf("expected restored content, got %q", data)
	}
	log, err := Log(p, name)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(strings.Split(strings.TrimSpace(log), "\n")); n != 2 {
		t.Fatalf("expected history preserved (2 commits), got %d", n)
	}
}

func TestRollbackSkipsUntrackedFiles(t *testing.T) {
	if !Available() {
		t.Skip("git not installed")
	}
	p, name := makeRepo(t)
	// 只写 opencode.json，模拟 -e 创建的 profile（无 tui.json/skills.yml）
	if err := os.WriteFile(p.ProfileConfig(name), []byte(`{"shell":"bash"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(p, name); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfileConfig(name), []byte(`{"shell":"zsh"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Commit(p, name, "switch shell"); err != nil {
		t.Fatal(err)
	}
	if err := Rollback(p, name, "HEAD~1"); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
	data, err := os.ReadFile(p.ProfileConfig(name))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"shell":"bash"}` {
		t.Fatalf("expected restored content, got %q", data)
	}
}

func TestRollbackRejectsDirtyWorkingTree(t *testing.T) {
	if !Available() {
		t.Skip("git not installed")
	}
	p, name := makeRepo(t)
	writeProfileFiles(t, p, name, `{"shell":"bash"}`)
	if err := Init(p, name); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfileConfig(name), []byte(`{"shell":"fish"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Rollback(p, name, "HEAD"); err == nil {
		t.Fatal("expected error when working tree is dirty")
	}
}

func TestRollbackOnUninitializedRepo(t *testing.T) {
	if !Available() {
		t.Skip("git not installed")
	}
	p, name := makeRepo(t)
	if err := Rollback(p, name, "HEAD"); err == nil {
		t.Fatal("expected error rolling back uninitialized repo")
	}
}

func TestRunErrorIncludesStderr(t *testing.T) {
	if !Available() {
		t.Skip("git not installed")
	}
	p, name := makeRepo(t)
	// 在非仓库目录执行 git 命令触发失败，stderr 应透传到错误信息
	_, _, err := run(p, name, "status", "--porcelain")
	if err == nil {
		t.Fatal("expected error running git in non-repo dir")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("expected git stderr in error message, got: %v", err)
	}
}
