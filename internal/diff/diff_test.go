package diff

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func TestRenderSections(t *testing.T) {
	var buf bytes.Buffer
	r := &Result{
		A: "default", B: "work",
		Providers: Change{Added: []string{"deepseek"}, Removed: []string{"meituan"}},
		MCP:       Change{Added: []string{"codegraph"}},
		Skills:    Change{},
	}
	Render(&buf, r)
	got := buf.String()

	if !strings.Contains(got, "Diff: default -> work") {
		t.Fatalf("missing header: %q", got)
	}
	if !strings.Contains(got, "[provider]") {
		t.Fatalf("missing provider section: %q", got)
	}
	if !strings.Contains(got, "  - meituan") || !strings.Contains(got, "  + deepseek") {
		t.Fatalf("provider changes missing: %q", got)
	}
	if !strings.Contains(got, "[mcp]") || !strings.Contains(got, "  + codegraph") {
		t.Fatalf("mcp section missing: %q", got)
	}
	if strings.Contains(got, "[skill]") {
		t.Fatalf("skill section should be omitted when empty: %q", got)
	}
	// 顺序：Removed 在前，Added 在后
	if idx := strings.Index(got, "- meituan"); idx > strings.Index(got, "+ deepseek") {
		t.Fatalf("removed should print before added: %q", got)
	}
}

func TestRenderNoDiff(t *testing.T) {
	var buf bytes.Buffer
	r := &Result{A: "default", B: "work"}
	Render(&buf, r)
	if got := buf.String(); got != "No differences between 'default' and 'work'\n" {
		t.Fatalf("got %q", got)
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
