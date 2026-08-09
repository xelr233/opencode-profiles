package git

import (
	"os"
	"path/filepath"
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
