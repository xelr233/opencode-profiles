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

func makeSkillSource(t *testing.T, p *paths.Paths, skill string) {
	t.Helper()
	dir := filepath.Join(p.SkillSourcesDir(), skill)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+skill+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.txt"), []byte("data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExportWithSkills(t *testing.T) {
	p, outDir, warn := makeExportEnv(t)
	makeSkillSource(t, p, "brainstorming")
	makeSkillSource(t, p, "rtk")
	if err := Export(p, "work", outDir, true, warn); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(outDir, "work-skills.zip")
	sk, ok := readZipEntry(t, zipPath, "brainstorming/SKILL.md")
	if !ok {
		t.Fatal("missing brainstorming/SKILL.md")
	}
	if !strings.Contains(string(sk), "# brainstorming") {
		t.Fatalf("bad content: %s", sk)
	}
	if _, ok := readZipEntry(t, zipPath, "rtk/data.txt"); !ok {
		t.Fatal("missing rtk/data.txt")
	}
	if _, ok := readZipEntry(t, zipPath, "missing-skill/SKILL.md"); ok {
		t.Fatal("missing-skill should not be exported")
	}
}

func TestExportWithSkillsMissingSourceWarns(t *testing.T) {
	p, outDir, warn := makeExportEnv(t)
	makeSkillSource(t, p, "brainstorming")
	// rtk 源不存在，skills.yml 里却有
	if err := Export(p, "work", outDir, true, warn); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn.String(), "rtk") {
		t.Fatalf("expected warning mentioning rtk, got: %q", warn.String())
	}
	if _, ok := readZipEntry(t, filepath.Join(outDir, "work-skills.zip"), "rtk/SKILL.md"); ok {
		t.Fatal("rtk should be skipped")
	}
}

func makeZip(t *testing.T, zipPath string, files map[string]string) {
	t.Helper()
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestImportProfile(t *testing.T) {
	p, outDir, warn := makeExportEnv(t)
	if err := Export(p, "work", outDir, false, warn); err != nil {
		t.Fatal(err)
	}
	// 导入到独立环境 p2（保证 work 不存在）
	p2 := paths.New(filepath.Join(t.TempDir(), "opencode"), filepath.Join(t.TempDir(), "skills"))
	if err := Import(p2, filepath.Join(outDir, "work.zip"), "work", "", warn); err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile(p2.ProfileConfig("work"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cfg), "provider") {
		t.Fatalf("provider should be stripped on import too: %s", cfg)
	}
	if _, err := os.Stat(p2.ProfileSkillsYML("work")); err != nil {
		t.Fatalf("skills.yml not restored: %v", err)
	}
	if _, err := os.Stat(p2.ProfileTUIConfig("work")); err != nil {
		t.Fatalf("tui.json not restored: %v", err)
	}
}

func TestImportInvalidZip(t *testing.T) {
	p, outDir, warn := makeExportEnv(t)
	badZip := filepath.Join(outDir, "bad.zip")
	makeZip(t, badZip, map[string]string{"skills.yml": "- foo\n"})
	if err := Import(p, badZip, "bad", "", warn); err == nil {
		t.Fatal("expected error for zip without opencode.json")
	}
}

func TestImportExistingProfileRejected(t *testing.T) {
	p, outDir, warn := makeExportEnv(t)
	if err := Export(p, "work", outDir, false, warn); err != nil {
		t.Fatal(err)
	}
	p2 := paths.New(filepath.Join(t.TempDir(), "opencode"), filepath.Join(t.TempDir(), "skills"))
	if err := Import(p2, filepath.Join(outDir, "work.zip"), "work", "", warn); err != nil {
		t.Fatal(err)
	}
	if err := Import(p2, filepath.Join(outDir, "work.zip"), "work", "", warn); err == nil {
		t.Fatal("expected error importing over existing profile")
	}
}

func TestImportWithNewName(t *testing.T) {
	p, outDir, warn := makeExportEnv(t)
	if err := Export(p, "work", outDir, false, warn); err != nil {
		t.Fatal(err)
	}
	// custom 在 p 中不存在，直接导入到 p
	if err := Import(p, filepath.Join(outDir, "work.zip"), "custom", "", warn); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.ProfileConfig("custom")); err != nil {
		t.Fatalf("custom profile not created: %v", err)
	}
}
