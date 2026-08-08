# Config Diff 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 新增 `-d` 命令显示 profile 间配置差异（provider/mcp/plugin/skill），并让 `-s` 切换前打印差异。

**架构：** 新建 `internal/diff` 包负责纯解析与集合对比，返回结构化 `Result` 并由 `Render` 分节打印。`SwitchDB` 增加 `out io.Writer` 参数，在切换前计算并渲染 `from → name` 的差异；`Switch` 保留旧签名（`io.Discard`）。`main.go` 暴露 `-d` 命令并更新 `-s` 分支。

**技术栈：** Go 1.25+，标准库 `encoding/json`/`flag`，现有 `internal/skills.ReadSkillsYML`。

**规格：** `docs/superpowers/specs/2026-08-09-config-diff-design.md`

---

## 文件结构

| 文件 | 职责 |
|------|------|
| `internal/diff/diff.go`（创建） | `Change`/`Result` 类型、`Diff()` 解析对比、`Render()` 分节打印 |
| `internal/diff/diff_test.go`（创建） | diff 包单测 |
| `internal/ops/ops.go`（修改） | `SwitchDB` 增加 `out` 参数并打印 diff；`Switch` 兼容 |
| `internal/ops/ops_test.go`（修改） | 更新 `SwitchDB` 调用签名，新增 diff 输出断言 |
| `cmd/opencode-profiles/main.go`（修改） | `-d` flag、位置参数处理、`-s` 传 writer |
| `cmd/opencode-profiles/main_test.go`（修改） | `-d` 命令测试、`-s` diff 输出测试、现有调用签名更新 |
| `README.md`（修改） | 新增 `-d` 用法示例 |

---

### 任务 1：`internal/diff` 包类型与 `Diff()`

**文件：**
- 创建：`internal/diff/diff.go`
- 测试：`internal/diff/diff_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
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
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/diff/ -v`
预期：编译失败，报错 `undefined: diff.Diff` / `cannot find package` 等（包尚不存在）。

- [ ] **步骤 3：创建包并实现最少代码**

创建 `internal/diff/diff.go`：

```go
package diff

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"opencode-profiles/internal/paths"
	"opencode-profiles/internal/skills"
)

// Change 表示某个维度的增删集合，均已排序。
type Change struct {
	Added   []string
	Removed []string
}

// Result 表示 profile A 到 B 的四维度差异。
type Result struct {
	A, B      string
	Providers Change
	MCP       Change
	Plugins   Change
	Skills    Change
}

// Diff 读取 profile a 与 b 的 opencode.json 与 skills.yml，
// 返回四维度键/项集合差异。
func Diff(p *paths.Paths, a, b string) (*Result, error) {
	cfgA, err := readConfig(p.ProfileConfig(a))
	if err != nil {
		return nil, err
	}
	cfgB, err := readConfig(p.ProfileConfig(b))
	if err != nil {
		return nil, err
	}
	skillsA, err := skills.ReadSkillsYML(p, a)
	if err != nil {
		return nil, err
	}
	skillsB, err := skills.ReadSkillsYML(p, b)
	if err != nil {
		return nil, err
	}
	return &Result{
		A:         a,
		B:         b,
		Providers: setDiff(mapKeys(cfgA["provider"]), mapKeys(cfgB["provider"])),
		MCP:       setDiff(mapKeys(cfgA["mcp"]), mapKeys(cfgB["mcp"])),
		Plugins:   setDiff(listItems(cfgA["plugin"]), listItems(cfgB["plugin"])),
		Skills:    setDiff(skillsA, skillsB),
	}, nil
}

func readConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

func mapKeys(v any) []string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func listItems(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// setDiff 返回 target 相对 current 的 (新增, 移除)，均排序。
func setDiff(current, target []string) Change {
	cur := make(map[string]struct{}, len(current))
	for _, s := range current {
		cur[s] = struct{}{}
	}
	tgt := make(map[string]struct{}, len(target))
	for _, s := range target {
		tgt[s] = struct{}{}
	}
	var c Change
	for s := range tgt {
		if _, ok := cur[s]; !ok {
			c.Added = append(c.Added, s)
		}
	}
	for s := range cur {
		if _, ok := tgt[s]; !ok {
			c.Removed = append(c.Removed, s)
		}
	}
	sort.Strings(c.Added)
	sort.Strings(c.Removed)
	return c
}
```

注意 `strings` 未使用故不引入；`encoding/json`/`fmt`/`os`/`sort` 均被下方代码使用。

- [ ] **步骤 5：运行测试验证通过**

运行：`go test ./internal/diff/ -v`
预期：PASS，所有 4 个测试通过。

- [ ] **步骤 6：Commit**

```bash
git add internal/diff/diff.go internal/diff/diff_test.go
git commit -m "feat: add diff package for config comparison"
```

---

### 任务 2：`Render()` 分节打印

**文件：**
- 修改：`internal/diff/diff.go`
- 测试：`internal/diff/diff_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
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
```

在测试文件 import 中加入 `bytes` 和 `strings`。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/diff/ -run TestRender -v`
预期：FAIL，报错 `undefined: Render`。

- [ ] **步骤 3：实现 `Render`**

在 `internal/diff/diff.go` 的 import 块中加入 `"io"`，并在文件末尾追加：

```go
// Render 将 Result 分节打印到 w。无变化的维度省略；
// 全部无差异时输出 No differences 行。
func Render(w io.Writer, r *Result) {
	if r.Empty() {
		fmt.Fprintf(w, "No differences between '%s' and '%s'\n", r.A, r.B)
		return
	}
	fmt.Fprintf(w, "Diff: %s -> %s\n", r.A, r.B)
	renderSection(w, "provider", r.Providers)
	renderSection(w, "mcp", r.MCP)
	renderSection(w, "plugin", r.Plugins)
	renderSection(w, "skill", r.Skills)
}

// Empty 报告四维度是否均无差异。
func (r *Result) Empty() bool {
	empty := func(c Change) bool { return len(c.Added) == 0 && len(c.Removed) == 0 }
	return empty(r.Providers) && empty(r.MCP) && empty(r.Plugins) && empty(r.Skills)
}

func renderSection(w io.Writer, name string, c Change) {
	if len(c.Added) == 0 && len(c.Removed) == 0 {
		return
	}
	fmt.Fprintf(w, "\n[%s]\n", name)
	for _, s := range c.Removed {
		fmt.Fprintf(w, "  - %s\n", s)
	}
	for _, s := range c.Added {
		fmt.Fprintf(w, "  + %s\n", s)
	}
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/diff/ -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/diff/diff.go internal/diff/diff_test.go
git commit -m "feat: add diff Render for sectioned output"
```

---

### 任务 3：`SwitchDB` 打印切换差异

**文件：**
- 修改：`internal/ops/ops.go:390-453`
- 测试：`internal/ops/ops_test.go`

- [ ] **步骤 1：编写失败的测试**

在 `internal/ops/ops_test.go` 新增（先写，编译失败验证）：

```go
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

func TestSwitchCompatibleNoOut(t *testing.T) {
	p, db := newEnv(t)
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
```

在测试文件 import 中加入 `bytes`。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/ops/ -run 'TestSwitch' -v`
预期：FAIL/编译失败（`SwitchDB` 参数不匹配、`Switch` 不存在）。

- [ ] **步骤 3：实现 `SwitchDB` 签名变更与 diff 打印**

修改 `internal/ops/ops.go`：

在 import 块增加：

```go
	"io"

	"opencode-profiles/internal/diff"
```

修改 `SwitchDB` 签名与函数体（在 `EnsureInitialized` 后、`target` 校验前打印 diff）：

```go
// SwitchDB 切换 symlink 指向目标 profile 并同步技能（dbPath 注入用）。
// out 接收切换前后差异输出。
func SwitchDB(p *paths.Paths, name, dbPath string, out io.Writer) error {
	if err := EnsureInitialized(p); err != nil {
		return err
	}

	from := GetActive(p)
	if from != "" {
		if result, err := diff.Diff(p, from, name); err != nil {
			return err
		} else {
			diff.Render(out, result)
		}
	}

	target := p.ProfileConfig(name)
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			available := ListProfiles(p)
			return fmt.Errorf("Profile '%s' not found. Available: %s", name, pythonList(available))
		}
		return err
	}
	// ... 其余切换逻辑保持不变（symlink、tui.json、SyncSkills）
```

修改 `Switch` 保留旧签名：

```go
// Switch 切换 symlink 指向目标 profile（使用默认 db 路径，丢弃 diff 输出）。
func Switch(p *paths.Paths, name string) error {
	return SwitchDB(p, name, "", io.Discard)
}
```

- [ ] **步骤 4：更新既有 `SwitchDB` 测试调用签名**

在 `internal/ops/ops_test.go` 中所有 `SwitchDB(p, name, db)` 调用改为 `SwitchDB(p, name, db, io.Discard)`：
- `TestSwitchAndActive`（约 273 行）
- `TestSwitchWithTUI`（约 298 行）
- `TestSwitchSyncsSkills`（约 523 行）

在 import 中加入 `io`。

- [ ] **步骤 5：运行测试验证通过**

运行：`go test ./internal/ops/ -v`
预期：PASS，包括新增的 `TestSwitchPrintsDiff` 与 `TestSwitchCompatibleNoOut`。

- [ ] **步骤 6：Commit**

```bash
git add internal/ops/ops.go internal/ops/ops_test.go
git commit -m "feat: print config diff before switch"
```

---

### 任务 4：`main.go` 新增 `-d` 命令

**文件：**
- 修改：`cmd/opencode-profiles/main.go`

- [ ] **步骤 1：编写失败的测试**

在 `cmd/opencode-profiles/main_test.go` 新增：

```go
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
	if res.code == 0 {
		t.Fatalf("expected failure")
	}
}

func TestDiffTooManyArgs(t *testing.T) {
	p, db, out, errOut := newCLIEnv(t)
	res := invoke(t, p, db, out, errOut, "-d", "a", "b", "c")
	if res.code == 0 || !strings.Contains(res.stderr, "-d") {
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
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./cmd/opencode-profiles/ -run 'TestDiff' -v`
预期：FAIL（`-d` 被当作未知 flag 解析失败，返回 2）。

- [ ] **步骤 3：实现 `-d` 命令与位置参数处理**

修改 `cmd/opencode-profiles/main.go`：

在 var 块新增：

```go
		diffFlag        bool
```

注册 flag：

```go
	fs.BoolVar(&diffFlag, "d", false, "显示当前与目标 profile 的配置差异（可加 1-2 个 profile 名）")
	fs.BoolVar(&diffFlag, "diff", false, "显示配置差异")
```

修改 NArg 检查（允许多个位置参数以支持 `-d A B`）：

```go
	if !diffFlag && fs.NArg() > 0 {
		fmt.Fprintln(stderr, "Unexpected argument: "+fs.Arg(0))
		return 2
	}
	if diffFlag {
		if fs.NArg() == 0 {
			fmt.Fprintln(stderr, "Error: -d requires at least one profile name")
			return 1
		}
		if fs.NArg() > 2 {
			fmt.Fprintln(stderr, "Error: -d accepts at most two profile names")
			return 1
		}
		if backupFlag || createName != "" || emptyName != "" || switchName != "" || addSkillName != "" || removeSkillName != "" || listFlag {
			fmt.Fprintln(stderr, "Error: -d cannot be combined with other commands")
			return 1
		}
	}
```

在 `-s` 分支前新增 `-d` 分支：

```go
	} else if diffFlag {
		var a, b string
		if fs.NArg() == 1 {
			a = ops.GetActive(p)
			if a == "" {
				fmt.Fprintln(stderr, "Error: no active profile to diff against")
				return 1
			}
			b = fs.Arg(0)
		} else {
			a, b = fs.Arg(0), fs.Arg(1)
		}
		result, err := diff.Diff(p, a, b)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		diff.Render(stdout, result)
	} else if switchName != "" {
		if err := ops.SwitchDB(p, switchName, dbPath, stdout); err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Switched to '%s'\n", switchName)
	}
```

在 import 块增加：

```go
	"opencode-profiles/internal/diff"
```

- [ ] **步骤 4：更新 `TestSwitchCommand` 断言 diff 输出**

现有 `TestSwitchCommand`（约 88 行）只断言 "Switched to 'work'"。新增对 diff 输出的断言：

```go
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
```

- [ ] **步骤 5：运行测试验证通过**

运行：`go test ./cmd/opencode-profiles/ -v`
预期：PASS。

- [ ] **步骤 6：Commit**

```bash
git add cmd/opencode-profiles/main.go cmd/opencode-profiles/main_test.go
git commit -m "feat: add -d command for config diff"
```

---

### 任务 5：README 文档

**文件：**
- 修改：`README.md`

- [ ] **步骤 1：在"切换 profile"后新增 diff 用法**

在 `README.md` 的"切换 profile"小节后添加：

```markdown
### 显示配置差异

```bash
# 比较当前激活 profile 与 work
opencode-profiles -d work

# 比较两个 profile
opencode-profiles -d work personal
```

输出按 provider / mcp / plugin / skill 四个维度分组，`-` 表示移除，`+` 表示新增。

切换 profile 时也会先显示差异：

```bash
opencode-profiles -s work
```
```

- [ ] **步骤 2：Commit**

```bash
git add README.md
git commit -m "docs: document -d diff command"
```

---

### 任务 6：整体验证

**文件：** 无代码变更

- [ ] **步骤 1：运行全部测试、vet、格式化检查**

```bash
go test ./...
go vet ./...
gofmt -l .
go build -trimpath -ldflags="-s -w" -o opencode-profiles ./cmd/opencode-profiles
```

预期：全部 PASS；`gofmt -l .` 无输出；构建成功生成 `opencode-profiles` 二进制。

- [ ] **步骤 2：手动冒烟验证（真实配置，只读操作）**

```bash
./opencode-profiles -d default
./opencode-profiles -d default nopower
./opencode-profiles -l
```

预期：`-d default` 输出当前激活（若为 default 则 `No differences`）；`-d default nopower` 输出两 profile 差异；`-l` 正常列出。

- [ ] **步骤 3：Commit（如冒烟发现问题则修复后 commit）**

```bash
git add -A
git commit -m "chore: verify config-diff feature" || true
```

（无变更时跳过 commit。）

---

## 自检

- **规格覆盖度：** `internal/diff` 包（任务 1-2）覆盖规格"Diff/Render"节；`SwitchDB` out 参数与打印（任务 3）覆盖"SwitchDB 签名变化"节；`-d` 命令与参数规则、互斥、无参数/超参报错（任务 4）覆盖"CLI"节；README（任务 5）覆盖"CLI"文档。测试矩阵覆盖规格"Testing"节全部条目与"Edge Cases"全部场景。
- **占位符扫描：** 无 "TODO"/"待定"；所有步骤含完整代码与命令。
- **类型一致性：** `diff.Result`/`diff.Change`/`diff.Diff`/`diff.Render` 在各任务间签名一致；`SwitchDB(p, name, dbPath, out)` 与 `Switch(p, name)` 定义与调用一致；`diffFlag`/`fs.NArg()`/`fs.Arg(i)` 使用一致。
