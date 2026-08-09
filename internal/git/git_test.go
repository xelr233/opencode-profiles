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
