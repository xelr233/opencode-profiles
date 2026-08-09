package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"opencode-profiles/internal/ops"
	"opencode-profiles/internal/paths"
)

func TestStripProviders(t *testing.T) {
	raw := []byte(`{"shell": "bash", "provider": {"deepseek": {"apiKey": "x"}}, "plugin": ["p"]}`)
	out, err := stripProviders(raw)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["provider"]; ok {
		t.Fatalf("provider key still present: %s", out)
	}
	if got["shell"] != "bash" {
		t.Fatalf("non-provider keys changed: %s", out)
	}
}

func TestStripProvidersNoProvider(t *testing.T) {
	raw := []byte(`{"mcp": {"a": {"url": "u"}}}`)
	out, err := stripProviders(raw)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["mcp"]; !ok {
		t.Fatalf("mcp lost: %s", out)
	}
}

func TestStripProvidersInvalidJSON(t *testing.T) {
	if _, err := stripProviders([]byte(`{not json`)); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func makeExportEnv(t *testing.T) (*paths.Paths, string, *bytes.Buffer) {
	t.Helper()
	base := filepath.Join(t.TempDir(), "opencode")
	src := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	p := paths.New(base, src)
	if err := ops.EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	// 构造带 provider、skills.yml、tui.json 的 profile
	if err := os.MkdirAll(p.ProfileDir("work"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfileConfig("work"), []byte(`{"provider": {"deepseek": {"apiKey": "x"}}, "shell": "zsh"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfileSkillsYML("work"), []byte("- brainstorming\n- rtk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfileTUIConfig("work"), []byte(`{"theme": "dark"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	var warn bytes.Buffer
	return p, outDir, &warn
}

func readZipEntry(t *testing.T, zipPath, name string) ([]byte, bool) {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(rc); err != nil {
				t.Fatal(err)
			}
			return buf.Bytes(), true
		}
	}
	return nil, false
}

func TestExportProfileZip(t *testing.T) {
	p, outDir, warn := makeExportEnv(t)
	if err := Export(p, "work", outDir, false, warn); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(outDir, "work.zip")
	cfg, ok := readZipEntry(t, zipPath, "opencode.json")
	if !ok {
		t.Fatal("missing opencode.json in zip")
	}
	if strings.Contains(string(cfg), "provider") {
		t.Fatalf("provider should be stripped: %s", cfg)
	}
	if _, ok := readZipEntry(t, zipPath, "skills.yml"); !ok {
		t.Fatal("missing skills.yml in zip")
	}
	if _, ok := readZipEntry(t, zipPath, "tui.json"); !ok {
		t.Fatal("missing tui.json in zip")
	}
	if _, ok := readZipEntry(t, zipPath, "work-skills.zip"); ok {
		t.Fatal("skills zip should not exist without --with-skills")
	}
}

func TestExportProfileMissing(t *testing.T) {
	p, outDir, warn := makeExportEnv(t)
	if err := Export(p, "nope", outDir, false, warn); err == nil {
		t.Fatal("expected error for missing profile")
	}
}
