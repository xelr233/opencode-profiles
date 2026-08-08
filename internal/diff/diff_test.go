package diff

import (
	"os"
	"path/filepath"
	"testing"

	"opencode-profiles/internal/paths"
	"opencode-profiles/internal/skills"
)

func TestDiffBasic(t *testing.T) {
	base := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	p := paths.New(base, filepath.Join(t.TempDir(), "skills"))

	mkProfile(t, p, "a", `{"provider":{"x":{},"shared":{}},"mcp":{"m1":{}},"plugin":["p1@1","shared@1"]}`, []string{"skillA", "sharedSkill"})
	mkProfile(t, p, "b", `{"provider":{"y":{},"shared":{}},"mcp":{"m2":{}},"plugin":["p2@2","shared@1"]}`, []string{"skillB", "sharedSkill"})

	r, err := Diff(p, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if r.A != "a" || r.B != "b" {
		t.Fatalf("A=%s B=%s", r.A, r.B)
	}
	checkChange(t, "provider", r.Providers, []string{"y"}, []string{"x"})
	checkChange(t, "mcp", r.MCP, []string{"m2"}, []string{"m1"})
	checkChange(t, "plugin", r.Plugins, []string{"p2@2"}, []string{"p1@1"})
	checkChange(t, "skill", r.Skills, []string{"skillB"}, []string{"skillA"})
}

func TestDiffIdentical(t *testing.T) {
	base := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	p := paths.New(base, filepath.Join(t.TempDir(), "skills"))
	mkProfile(t, p, "a", `{"provider":{"x":{}},"plugin":["p1"]}`, []string{"s1"})
	mkProfile(t, p, "b", `{"provider":{"x":{}},"plugin":["p1"]}`, []string{"s1"})

	r, err := Diff(p, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Providers.Added)+len(r.Providers.Removed) != 0 {
		t.Fatalf("providers = %+v", r.Providers)
	}
	if len(r.Skills.Added)+len(r.Skills.Removed) != 0 {
		t.Fatalf("skills = %+v", r.Skills)
	}
}

func TestDiffMissingKeys(t *testing.T) {
	base := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	p := paths.New(base, filepath.Join(t.TempDir(), "skills"))
	mkProfile(t, p, "a", `{"shell":"bash"}`, nil) // 无 provider/mcp/plugin，无 skills.yml
	mkProfile(t, p, "b", `{"shell":"zsh"}`, nil)

	r, err := Diff(p, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Providers.Added)+len(r.Providers.Removed) != 0 {
		t.Fatalf("providers = %+v", r.Providers)
	}
	if len(r.MCP.Added)+len(r.MCP.Removed) != 0 {
		t.Fatalf("mcp = %+v", r.MCP)
	}
	if len(r.Skills.Added)+len(r.Skills.Removed) != 0 {
		t.Fatalf("skills = %+v", r.Skills)
	}
}

func TestDiffMissingProfile(t *testing.T) {
	base := filepath.Join(t.TempDir(), "opencode")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	p := paths.New(base, filepath.Join(t.TempDir(), "skills"))
	mkProfile(t, p, "a", `{}`, nil)
	if _, err := Diff(p, "a", "nope"); err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func mkProfile(t *testing.T, p *paths.Paths, name, config string, skillsList []string) {
	t.Helper()
	dir := p.ProfileDir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfileConfig(name), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if skillsList != nil {
		if err := skills.WriteSkillsYML(p, name, skillsList); err != nil {
			t.Fatal(err)
		}
	}
}

func checkChange(t *testing.T, label string, got Change, wantAdded, wantRemoved []string) {
	t.Helper()
	if !equalStrings(got.Added, wantAdded) {
		t.Fatalf("%s Added = %v, want %v", label, got.Added, wantAdded)
	}
	if !equalStrings(got.Removed, wantRemoved) {
		t.Fatalf("%s Removed = %v, want %v", label, got.Removed, wantRemoved)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
