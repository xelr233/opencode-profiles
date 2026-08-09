# Profile 导出导入实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为 opencode-profiles 添加 profile 的导出/导入：导出为 zip（opencode.json 删除 provider），可选导出源 skills zip；导入还原 profile（可指定新名字、携带 skills）。

**架构：** 新包 `internal/export` 负责 zip 读写（标准库 `archive/zip`）。`Export` 生成 `<name>.zip`（去 provider 的配置 + skills.yml + tui.json）与可选 `<name>-skills.zip`（技能源目录）。`Import` 校验并解压 profile zip 到 `profiles/<name>/`，可选导入 skills zip 到 `~/.cc-switch/skills/<skill>/` 并为存在的技能建 symlink。CLI 通过首个位置参数识别 `export`/`import` 子命令，与现有 flag 风格共存。

**技术栈：** Go 1.25+，标准库 `archive/zip`、`encoding/json`、`gopkg.in/yaml.v3`（已有）。无新依赖。

---

## 文件结构

- **创建 `internal/export/export.go`** — `Export`/`Import` 函数 + `stripProviders`/`extractFile`/`importSkillsZip`/`exportSkillsZip` 内部辅助。全部 warning 写入注入的 `io.Writer`。
- **创建 `internal/export/export_test.go`** — 单元测试（`t.TempDir()` 隔离）。
- **修改 `cmd/opencode-profiles/main.go`** — 子命令分发 + `runExport`/`runImport`。
- **修改 `cmd/opencode-profiles/main_test.go`** — CLI 测试。
- **修改 `README.md`** — 文档。

---

### 任务 1：`internal/export` 骨架与 `stripProviders`

**文件：**
- 创建：`internal/export/export.go`
- 测试：`internal/export/export_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
package export

import (
	"encoding/json"
	"testing"
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
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/export/ -run TestStripProviders -v`
预期：FAIL，报编译错误（包不存在 / stripProviders not defined）

- [ ] **步骤 3：编写最少实现代码**

```go
// Package export 提供 profile 的导出与导入（zip 格式）。
package export

import (
	"encoding/json"
)

// stripProviders 读取 opencode.json 内容并删除 provider 键，其余字段原样保留。
func stripProviders(raw []byte) ([]byte, error) {
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	delete(cfg, "provider")
	return json.MarshalIndent(cfg, "", "  ")
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/export/ -run TestStripProviders -v`
预期：3 个测试全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/export/export.go internal/export/export_test.go
git commit -m "feat: add export package with provider stripping"
```

---

### 任务 2：`Export` 生成 profile zip

**文件：**
- 修改：`internal/export/export.go`
- 修改：`internal/export/export_test.go`

- [ ] **步骤 1：编写失败的测试**

在 `export_test.go` 追加测试辅助与导出测试。辅助函数：

```go
package export

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"opencode-profiles/internal/ops"
	"opencode-profiles/internal/paths"
)

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
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/export/ -run TestExport -v`
预期：FAIL（Export not defined）

- [ ] **步骤 3：编写最少实现代码**

在 `export.go` 添加导入与 `Export`：

```go
import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"opencode-profiles/internal/ops"
	"opencode-profiles/internal/paths"
)

// Export 将 profile 导出为 zip。withSkills 时额外生成 <name>-skills.zip。
// warning 写入 warn。
func Export(p *paths.Paths, name, outDir string, withSkills bool, warn io.Writer) error {
	if err := ops.EnsureInitialized(p); err != nil {
		return err
	}
	configPath := p.ProfileConfig(name)
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("Profile '%s' not found", name)
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	stripped, err := stripProviders(raw)
	if err != nil {
		return err
	}

	zipPath := filepath.Join(outDir, name+".zip")
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)

	if err := writeZipBytes(zw, "opencode.json", stripped); err != nil {
		zw.Close()
		f.Close()
		return err
	}
	if _, err := os.Stat(p.ProfileSkillsYML(name)); err == nil {
		yml, err := os.ReadFile(p.ProfileSkillsYML(name))
		if err != nil {
			zw.Close()
			f.Close()
			return err
		}
		if err := writeZipBytes(zw, "skills.yml", yml); err != nil {
			zw.Close()
			f.Close()
			return err
		}
	}
	if _, err := os.Stat(p.ProfileTUIConfig(name)); err == nil {
		tui, err := os.ReadFile(p.ProfileTUIConfig(name))
		if err != nil {
			zw.Close()
			f.Close()
			return err
		}
		if err := writeZipBytes(zw, "tui.json", tui); err != nil {
			zw.Close()
			f.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	if withSkills {
		if err := exportSkillsZip(p, name, outDir, warn); err != nil {
			return err
		}
	}
	return nil
}

// writeZipBytes 向 zip writer 写入一个条目。
func writeZipBytes(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
```

`exportSkillsZip` 当前留空实现（任务 3 填充）：

```go
// exportSkillsZip 导出 skills.yml 中引用的技能源为 <name>-skills.zip。缺失源跳过并 warning。
func exportSkillsZip(p *paths.Paths, name, outDir string, warn io.Writer) error {
	return nil
}
```

注意：任务 2 末尾 `TestExportProfileZip` 断言 `work-skills.zip` 不存在——因为 `withSkills=false`，所以该断言不受 `exportSkillsZip` 空实现影响。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/export/ -run TestExport -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/export/export.go internal/export/export_test.go
git commit -m "feat: export profile as zip without providers"
```

---

### 任务 3：`--with-skills` 导出 skills zip

**文件：**
- 修改：`internal/export/export.go`
- 修改：`internal/export/export_test.go`

- [ ] **步骤 1：编写失败的测试**

在 `export_test.go` 追加：

```go
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
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/export/ -run TestExportWithSkills -v`
预期：FAIL（skills zip 为空）

- [ ] **步骤 3：编写实现代码**

替换 `exportSkillsZip` 空实现（此时需在 `export.go` 的 import 块中加入 `"opencode-profiles/internal/skills"`）：

```go
// exportSkillsZip 导出 skills.yml 中引用的技能源为 <name>-skills.zip。缺失源跳过并 warning。
func exportSkillsZip(p *paths.Paths, name, outDir string, warn io.Writer) error {
	skillsList, err := skills.ReadSkillsYML(p, name)
	if err != nil {
		return err
	}
	zipPath := filepath.Join(outDir, name+"-skills.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)

	for _, skill := range skillsList {
		src := p.SkillSource(skill)
		if _, err := os.Stat(src); err != nil {
			fmt.Fprintf(warn, "Warning: skill source '%s' not found, skipped\n", skill)
			continue
		}
		err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			entry := skill + "/" + rel
			if d.IsDir() {
				_, err := zw.Create(entry + "/")
				return err
			}
			return addZipFile(zw, entry, path)
		})
		if err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}

// addZipFile 将文件内容写入 zip 的指定条目。
func addZipFile(zw *zip.Writer, entry, path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(fi)
	if err != nil {
		return err
	}
	hdr.Name = entry
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	rc, err := os.Open(path)
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(w, rc)
	return err
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/export/ -run TestExport -v`
预期：PASS（含 with-skills 两个新测试）

- [ ] **步骤 5：Commit**

```bash
git add internal/export/export.go internal/export/export_test.go
git commit -m "feat: export source skills as zip when --with-skills"
```

---

### 任务 4：`Import` 还原 profile zip

**文件：**
- 修改：`internal/export/export.go`
- 修改：`internal/export/export_test.go`

- [ ] **步骤 1：编写失败的测试**

在 `export_test.go` 追加：

```go
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
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/export/ -run TestImport -v`
预期：FAIL（Import not defined）

- [ ] **步骤 3：编写实现代码**

在 `export.go` 追加 `Import` 与 `extractFile`：

```go
// Import 从 zip 还原 profile 到 profiles/<name>/。skillsZipPath 为空时尝试
// 同目录 <basename>-skills.zip 自动关联。warning 写入 warn。
func Import(p *paths.Paths, zipPath, name, skillsZipPath string, warn io.Writer) error {
	if err := ops.EnsureInitialized(p); err != nil {
		return err
	}
	profileDir := p.ProfileDir(name)
	if _, err := os.Stat(profileDir); err == nil {
		return fmt.Errorf("Profile '%s' already exists, use --name to import under a different name", name)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	hasConfig := false
	for _, f := range zr.File {
		if f.Name == "opencode.json" {
			hasConfig = true
			break
		}
	}
	if !hasConfig {
		return fmt.Errorf("invalid profile zip '%s': missing opencode.json", zipPath)
	}

	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return err
	}
	for _, f := range zr.File {
		switch f.Name {
		case "opencode.json", "skills.yml", "tui.json":
			if err := extractFile(f, profileDir); err != nil {
				return err
			}
		}
	}

	// skills zip：显式指定或自动关联
	skillsPath := skillsZipPath
	if skillsPath == "" {
		base := strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
		candidate := filepath.Join(filepath.Dir(zipPath), base+"-skills.zip")
		if _, err := os.Stat(candidate); err == nil {
			skillsPath = candidate
		}
	}
	if skillsPath != "" {
		if err := importSkillsZip(p, skillsPath, warn); err != nil {
			return err
		}
	}

	// skills.yml 中引用的技能：本地缺失的 warning，存在的确保 symlink
	return linkExistingSkills(p, name, warn)
}

// extractFile 将 zip 条目解压到 dstDir，拒绝路径穿越。
func extractFile(f *zip.File, dstDir string) error {
	if strings.Contains(f.Name, "..") || filepath.IsAbs(f.Name) {
		return fmt.Errorf("unsafe zip entry: %s", f.Name)
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	dst := filepath.Join(dstDir, f.Name)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, rc); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// importSkillsZip 与 linkExistingSkills 在任务 5 实现。
func importSkillsZip(p *paths.Paths, zipPath string, warn io.Writer) error {
	return nil
}

func linkExistingSkills(p *paths.Paths, name string, warn io.Writer) error {
	return nil
}
```

需要 `"strings"` 导入。`linkExistingSkills` 空实现会让 `TestImportProfile` 通过（skills.yml 本地源不存在 → 不建 symlink），`TestImport*` 系列先绿。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/export/ -run TestImport -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/export/export.go internal/export/export_test.go
git commit -m "feat: import profile zip with name conflict detection"
```

---

### 任务 5：skills zip 导入与 symlink 同步

**文件：**
- 修改：`internal/export/export.go`
- 修改：`internal/export/export_test.go`

- [ ] **步骤 1：编写失败的测试**

在 `export_test.go` 追加：

```go
func TestImportWithSkills(t *testing.T) {
	p, outDir, warn := makeExportEnv(t)
	makeSkillSource(t, p, "brainstorming")
	makeSkillSource(t, p, "rtk")
	if err := Export(p, "work", outDir, true, warn); err != nil {
		t.Fatal(err)
	}
	// 导入到另一个独立环境（干净的 skill_sources 与 base）
	base2 := filepath.Join(t.TempDir(), "opencode")
	src2 := filepath.Join(t.TempDir(), "skills")
	p2 := paths.New(base2, src2)
	if err := Import(p2, filepath.Join(outDir, "work.zip"), "work", "", warn); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p2.SkillSource("brainstorming")); err != nil {
		t.Fatalf("skill source not imported: %v", err)
	}
	link := filepath.Join(p2.BaseDir(), "skills", "brainstorming")
	resolved, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if resolved != p2.SkillSource("brainstorming") {
		t.Fatalf("symlink points to %q, want %q", resolved, p2.SkillSource("brainstorming"))
	}
}

func TestImportWithSkillsSkipsExistingSource(t *testing.T) {
	p, outDir, warn := makeExportEnv(t)
	makeSkillSource(t, p, "brainstorming")
	makeSkillSource(t, p, "rtk")
	if err := Export(p, "work", outDir, true, warn); err != nil {
		t.Fatal(err)
	}
	// 目标环境已存在 brainstorming 源（旧版本）
	p2 := paths.New(filepath.Join(t.TempDir(), "opencode"), filepath.Join(t.TempDir(), "skills"))
	if err := os.MkdirAll(p2.SkillSource("brainstorming"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p2.SkillSource("brainstorming"), "SKILL.md"), []byte("# old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	warn.Reset()
	if err := Import(p2, filepath.Join(outDir, "work.zip"), "work", "", warn); err != nil {
		t.Fatal(err)
	}
	old, _ := os.ReadFile(filepath.Join(p2.SkillSource("brainstorming"), "SKILL.md"))
	if string(old) != "# old\n" {
		t.Fatalf("existing source overwritten: %s", old)
	}
	if !strings.Contains(warn.String(), "brainstorming") {
		t.Fatalf("expected skip warning for brainstorming, got: %q", warn.String())
	}
}

func TestImportMissingSkillWarnsNoLink(t *testing.T) {
	p, outDir, warn := makeExportEnv(t)
	if err := Export(p, "work", outDir, false, warn); err != nil {
		t.Fatal(err)
	}
	p2 := paths.New(filepath.Join(t.TempDir(), "opencode"), filepath.Join(t.TempDir(), "skills"))
	warn.Reset()
	if err := Import(p2, filepath.Join(outDir, "work.zip"), "work", "", warn); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn.String(), "brainstorming") {
		t.Fatalf("expected missing-skill warning, got: %q", warn.String())
	}
	link := filepath.Join(p2.BaseDir(), "skills", "brainstorming")
	if _, err := os.Lstat(link); err == nil {
		t.Fatal("symlink should not be created for missing skill")
	}
}

func TestImportExplicitSkillsZip(t *testing.T) {
	p, outDir, warn := makeExportEnv(t)
	makeSkillSource(t, p, "brainstorming")
	makeSkillSource(t, p, "rtk")
	if err := Export(p, "work", outDir, true, warn); err != nil {
		t.Fatal(err)
	}
	// 将 skills zip 移到独立目录，验证显式 --skills 路径（避免自动关联）
	skillsDir := t.TempDir()
	skillsZip := filepath.Join(skillsDir, "moved-skills.zip")
	data, err := os.ReadFile(filepath.Join(outDir, "work-skills.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillsZip, data, 0o644); err != nil {
		t.Fatal(err)
	}
	p2 := paths.New(filepath.Join(t.TempDir(), "opencode"), filepath.Join(t.TempDir(), "skills"))
	warn.Reset()
	if err := Import(p2, filepath.Join(outDir, "work.zip"), "work", skillsZip, warn); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p2.SkillSource("brainstorming")); err != nil {
		t.Fatalf("skill source not imported via explicit --skills: %v", err)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/export/ -run TestImportWith -v`
预期：FAIL（skills zip 未导入 / symlink 未创建）

- [ ] **步骤 3：编写实现代码**

替换 `importSkillsZip` 与 `linkExistingSkills` 空实现：

```go
// importSkillsZip 解压 skills zip 到 skill_sources_dir。<skill> 源已存在时跳过该技能并 warning。
func importSkillsZip(p *paths.Paths, zipPath string, warn io.Writer) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) < 2 || parts[0] == "" {
			continue
		}
		skill := parts[0]
		src := p.SkillSource(skill)
		if _, err := os.Stat(src); err == nil {
			fmt.Fprintf(warn, "Warning: skill '%s' already exists locally, skipped\n", skill)
			continue
		}
		if strings.Contains(f.Name, "..") || filepath.IsAbs(f.Name) {
			return fmt.Errorf("unsafe zip entry: %s", f.Name)
		}
		rel := strings.TrimSuffix(parts[1], "/")
		if rel == "" {
			continue
		}
		dst := filepath.Join(src, rel)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := extractFile(f, src); err != nil {
			return err
		}
	}
	return nil
}

// linkExistingSkills 为 skills.yml 中本地存在的技能建立 base/skills/<skill> symlink。
// 本地缺失的技能打印 warning，不建 symlink；同名 symlink 已存在则跳过。
func linkExistingSkills(p *paths.Paths, name string, warn io.Writer) error {
	skillsList, err := skills.ReadSkillsYML(p, name)
	if err != nil {
		return err
	}
	skillsDir := filepath.Join(p.BaseDir(), "skills")
	for _, skill := range skillsList {
		if _, err := os.Stat(p.SkillSource(skill)); err != nil {
			fmt.Fprintf(warn, "Warning: skill '%s' not found locally, symlink not created\n", skill)
			continue
		}
		link := filepath.Join(skillsDir, skill)
		if _, err := os.Lstat(link); err == nil {
			continue
		}
		if err := os.MkdirAll(skillsDir, 0o755); err != nil {
			return err
		}
		if err := os.Symlink(p.SkillSource(skill), link); err != nil {
			return err
		}
	}
	return nil
}
```

注意：`extractFile(f, src)` 的 dst 是 `src/rel`，与 `importSkillsZip` 中计算的一致，且 `extractFile` 已含 `..` 校验（双保险）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/export/ -v`
预期：全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/export/export.go internal/export/export_test.go
git commit -m "feat: import skills zip and sync symlinks"
```

---

### 任务 6：CLI 子命令接线

**文件：**
- 修改：`cmd/opencode-profiles/main.go`
- 修改：`cmd/opencode-profiles/main_test.go`

- [ ] **步骤 1：编写失败的测试**

在 `cmd/opencode-profiles/main_test.go` 追加：

```go
func TestExportCommand(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	if err := ops.EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	// 构造 work profile
	if err := ops.CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, p.ProfileConfig("work"), `{"provider": {"deepseek": {}}, "shell": "zsh"}`)

	outDir := t.TempDir()
	code := run([]string{"export", "work", "--out", outDir}, out, errOut, p, db)
	if code != 0 {
		t.Fatalf("export failed code=%d stderr=%q", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(outDir, "work.zip")); err != nil {
		t.Fatalf("zip not created: %v", err)
	}
}

func TestExportCommandMissingProfile(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	code := run([]string{"export", "nope"}, out, errOut, p, db)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "not found") {
		t.Fatalf("stderr=%q", errOut.String())
	}
}

func TestImportCommand(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	if err := ops.EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	// 先导出再导入
	srcP, srcDB, _, _ := newCLIEnv(t)
	if err := ops.EnsureInitialized(srcP); err != nil {
		t.Fatal(err)
	}
	if err := ops.CreateEmpty(srcP, "work", ""); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, srcP.ProfileConfig("work"), `{"provider": {"deepseek": {}}}`)
	outDir := t.TempDir()
	if code := run([]string{"export", "work", "--out", outDir}, &bytes.Buffer{}, &bytes.Buffer{}, srcP, srcDB); code != 0 {
		t.Fatalf("export failed code=%d", code)
	}
	code := run([]string{"import", filepath.Join(outDir, "work.zip")}, out, errOut, p, db)
	if code != 0 {
		t.Fatalf("import failed code=%d stderr=%q", code, errOut.String())
	}
	if _, err := os.Stat(p.ProfileConfig("work")); err != nil {
		t.Fatalf("work profile not created: %v", err)
	}
}

func TestImportCommandNameConflict(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	if err := ops.EnsureInitialized(p); err != nil {
		t.Fatal(err)
	}
	if err := ops.CreateEmpty(p, "work", ""); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	srcP, srcDB, _, _ := newCLIEnv(t)
	if err := ops.EnsureInitialized(srcP); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, srcP.ProfileConfig("default"), `{"provider": {"x": {}}}`)
	if code := run([]string{"export", "default", "--out", outDir}, &bytes.Buffer{}, &bytes.Buffer{}, srcP, srcDB); code != 0 {
		t.Fatalf("export failed code=%d", code)
	}
	code := run([]string{"import", filepath.Join(outDir, "default.zip"), "--name", "work"}, out, errOut, p, db)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "already exists") {
		t.Fatalf("stderr=%q", errOut.String())
	}
}
```

注意：`TestImportCommand` 与 `TestImportCommandNameConflict` 需要 `bytes` 已有导入（`newCLIEnv` 已返回 `*bytes.Buffer`）。`TestImportCommand` 中用 `outDir := t.TempDir()` 但随后未用作 `--out` 之外——已用于导出。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./cmd/opencode-profiles/ -run TestExportCommand -v`
预期：FAIL（export 子命令未接线 → 走 `-help` 分支返回 0 或报错）

- [ ] **步骤 3：编写实现代码**

在 `main.go` 修改 `run` 开头添加子命令分发：

```go
func run(args []string, stdout, stderr io.Writer, p *paths.Paths, dbPath string) int {
	// 子命令：export / import（首个位置参数）
	if len(args) > 0 {
		switch args[0] {
		case "export":
			return runExport(args[1:], stdout, stderr, p)
		case "import":
			return runImport(args[1:], stdout, stderr, p)
		}
	}
	// ...原有 flag 解析代码不变
```

在 `run` 函数之后（或文件尾部）添加：

```go
// runExport 处理 export 子命令：export <name> [--with-skills] [--out <dir>]。
func runExport(args []string, stdout, stderr io.Writer, p *paths.Paths) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	withSkills := fs.Bool("with-skills", false, "也导出源 skills 压缩包")
	outDir := fs.String("out", ".", "输出目录（默认当前目录）")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Error: export requires exactly one profile name")
		return 1
	}
	name := fs.Arg(0)
	if err := exportpkg.Export(p, name, *outDir, *withSkills, stderr); err != nil {
		fmt.Fprintf(stderr, "Error: %s\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Exported profile '%s' to %s\n", name, filepath.Join(*outDir, name+".zip"))
	return 0
}

// runImport 处理 import 子命令：import <file.zip> [--name <new>] [--skills <file.zip>]。
func runImport(args []string, stdout, stderr io.Writer, p *paths.Paths) int {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "导入后的 profile 名（默认取 zip 文件名）")
	skillsZip := fs.String("skills", "", "显式指定 skills zip")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Error: import requires exactly one zip file")
		return 1
	}
	zipPath := fs.Arg(0)
	importName := *name
	if importName == "" {
		importName = strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
	}
	if err := exportpkg.Import(p, zipPath, importName, *skillsZip, stderr); err != nil {
		fmt.Fprintf(stderr, "Error: %s\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Imported profile '%s' from %s\n", importName, zipPath)
	return 0
}
```

导入 `exportpkg "opencode-profiles/internal/export"` 与 `path/filepath`：

```go
import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"opencode-profiles/internal/diff"
	exportpkg "opencode-profiles/internal/export"
	gitpkg "opencode-profiles/internal/git"
	"opencode-profiles/internal/ops"
	"opencode-profiles/internal/paths"
	"opencode-profiles/internal/skills"
)
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./cmd/opencode-profiles/ -run TestExportCommand -v && go test ./cmd/opencode-profiles/ -run TestImportCommand -v`
预期：全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add cmd/opencode-profiles/main.go cmd/opencode-profiles/main_test.go
git commit -m "feat: add export/import CLI subcommands"
```

---

### 任务 7：README 文档

**文件：**
- 修改：`README.md`

- [ ] **步骤 1：在 README 追加导出导入章节**

在 `### Profile 版本管理` 之后追加：

```markdown
### 导出 / 导入 profile

导出 profile（导出的 `opencode.json` 不含任何 provider，避免泄露密钥）：

```bash
# 导出为 ./work.zip（含 opencode.json、skills.yml、tui.json）
opencode-profiles export work

# 同时导出源 skills 压缩包（默认不开启）
opencode-profiles export work --with-skills   # 生成 work.zip + work-skills.zip

# 指定输出目录
opencode-profiles export work --out /path/to/backup
```

导入 zip：

```bash
# 默认 profile 名 = zip 文件名（去 .zip）
opencode-profiles import work.zip

# 指定导入名（目标已存在时用 --name 避免覆盖冲突）
opencode-profiles import work.zip --name work-backup

# 显式指定 skills zip（未指定时自动关联同目录的 <name>-skills.zip）
opencode-profiles import work.zip --skills work-skills.zip
```

导入说明：

- 仅导入 profile 文件，不切换当前激活 profile，不导入 git 历史。
- 目标 profile 已存在时报错，用 `--name` 指定新名字。
- `skills.yml` 原样保留；本地缺失的技能源打印 warning 且不创建软链接。
- skills zip 导入到 `~/.cc-switch/skills/<skill>/`，同名源已存在时跳过不覆盖。
```

- [ ] **步骤 2：验证渲染**

运行：`grep -n "导出 / 导入 profile" README.md`
预期：找到章节标题

- [ ] **步骤 3：Commit**

```bash
git add README.md
git commit -m "docs: document export/import commands"
```

---

### 任务 8：全量验证

**文件：** 无

- [ ] **步骤 1：运行全量测试**

运行：`go test ./...`
预期：全部 PASS（此前 90 个 + 新增 export/CLI 测试）

- [ ] **步骤 2：静态检查与格式**

运行：`go vet ./... && gofmt -l .`
预期：vet 无输出，gofmt 无文件列出

- [ ] **步骤 3：交叉编译验证**

运行：
```bash
GOOS=linux GOARCH=arm64 go build ./cmd/opencode-profiles
GOOS=darwin GOARCH=arm64 go build ./cmd/opencode-profiles
GOOS=windows GOARCH=amd64 go build ./cmd/opencode-profiles
```
预期：全部成功，无输出

- [ ] **步骤 4：真实环境冒烟测试**

用临时 HOME 验证端到端（`git` 已安装）：
```bash
TMP=$(mktemp -d)
HOME="$TMP" go run ./cmd/opencode-profiles export default --out "$TMP" 2>&1 || true
ls -la "$TMP"
rm -rf "$TMP"
```
预期：`default.zip` 生成，`unzip -l` 可见 `opencode.json`、`skills.yml`。注：全新环境 `export default` 可能因 profile 无内容仍成功（EnsureInitialized 后 default 存在）。若有 provider 则断言 zip 内无 `provider` 键。

- [ ] **步骤 5：更新 `.superpowers/sdd/progress.md` 记录账本**

```bash
mkdir -p .superpowers/sdd
cat >> .superpowers/sdd/progress.md << 'EOF'
Export/import feature (branch feat/profile-export-import):
- Task 1: export package skeleton + stripProviders
- Task 2: Export profile zip
- Task 3: --with-skills skills zip
- Task 4: Import profile zip
- Task 5: skills zip import + symlink sync
- Task 6: CLI subcommands
- Task 7: README docs
- Task 8: full verification
EOF
git add .superpowers/sdd/progress.md
git commit -m "docs: record export/import implementation ledger"
```

---

## 自检

**1. 规格覆盖度：**
- 导出默认 1 个 zip → 任务 2（`<name>.zip`，去 provider、含 skills.yml/tui.json）
- `--with-skills` 额外 skills zip → 任务 3（`<name>-skills.zip`，子目录 `<skill>/`，缺源跳过+warning）
- `--out` 输出目录 → 任务 2/6（`Export` 参数 `outDir`，CLI `--out`）
- 导入校验 opencode.json → 任务 4（`hasConfig` 检查）
- 默认名 = 去 `.zip`、`--name` 重命名 → 任务 6（`runImport`）
- 目标已存在报错 → 任务 4（`TestImportExistingProfileRejected`）
- 仅导入不切换 → 任务 4/6（Import 不调用 Switch）
- skills.yml 原样保留 + 缺失技能 warning 不建软链接 → 任务 5（`linkExistingSkills`）
- skills zip 同名跳过不覆盖 → 任务 5（`importSkillsZip` 的 `os.Stat(src)` 检查 + `TestImportWithSkillsSkipsExistingSource`）
- 显式 `--skills` 与自动关联 → 任务 4/6（`skillsZipPath` 参数 + 同目录 `<base>-skills.zip` 探测）

**2. 占位符扫描：** 无 TODO/待定；每步含完整代码与预期输出。任务 2/4 的临时空实现（`exportSkillsZip`、`importSkillsZip`、`linkExistingSkills`）均有明确的下游任务填充步骤，且不阻塞各自任务测试通过。

**3. 类型一致性：**
- `Export(p *paths.Paths, name, outDir string, withSkills bool, warn io.Writer) error` → 任务 2 定义，任务 6 调用一致。
- `Import(p *paths.Paths, zipPath, name, skillsZipPath string, warn io.Writer) error` → 任务 4 定义，任务 6 调用一致。
- `stripProviders(raw []byte) ([]byte, error)` → 任务 1 定义，任务 2 调用一致。
- `readZipEntry(t, zipPath, name)`、`makeZip`、`makeExportEnv` 测试辅助在任务 2 定义，后续任务复用。
- `importSkillsZip` / `linkExistingSkills` 签名任务 4 声明、任务 5 实现，一致。
- `Export`/`Import` 开头调用 `ops.EnsureInitialized`（与 AGENTS.md 架构约定一致）——`internal/export` 依赖 `internal/ops`，`ops` 不依赖 `export`，无循环依赖（已核实）。
