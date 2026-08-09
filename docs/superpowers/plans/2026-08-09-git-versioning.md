# Profile Git 版本管理实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为 profile 添加可选的 git 版本管理：每个 profile 独立仓库，支持 init/commit/log/soft-rollback 四个 CLI 子命令。

**架构：** 新增 `internal/git` 包封装对 git 二进制的调用（`exec.Command`，零新依赖）。CLI 在 `main.go` 新增 `--git-init`/`--git-commit`/`--git-log`/`--git-rollback` 四个 flag，复用现有 `run()` 可注入测试接口。

**技术栈：** Go 1.25+，stdlib `os/exec`，系统 git 二进制。

---

## 文件结构

- 创建：`internal/git/git.go` — git 封装：Available/IsRepo/Init/Commit/Log/Rollback 及内部 run/status 辅助
- 创建：`internal/git/git_test.go` — 单元测试（git 未安装时 `t.Skip`）
- 修改：`cmd/opencode-profiles/main.go` — 四个新 flag + 互斥校验 + 分发逻辑
- 修改：`cmd/opencode-profiles/main_test.go` — CLI 集成测试
- 修改：`README.md` — 文档新增命令示例
- 修改：`docs/superpowers/specs/2026-08-09-git-versioning-design.md` —（已存在，作为规格依据）

---

### 任务 1：`internal/git` 包 — run/status 与 Available/IsRepo

**文件：**
- 创建：`internal/git/git.go`
- 测试：`internal/git/git_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
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
	if err := run(p, name, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if !IsRepo(p, name) {
		t.Fatal("expected repo after git init")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/git/ -v`
预期：FAIL，报错 "undefined: Available" / "undefined: run" / "undefined: IsRepo"

- [ ] **步骤 3：编写最少实现代码**

```go
package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"opencode-profiles/internal/paths"
)

// ErrGitNotFound 表示系统未安装 git。
var ErrGitNotFound = errors.New("git is not installed; please install git first")

// gitExec 缓存 exec.LookPath 结果，便于测试覆盖。
var gitExec = func() string {
	path, err := exec.LookPath("git")
	if err != nil {
		return ""
	}
	return path
}()

// Available 报告系统中是否安装了 git。
func Available() bool {
	return gitExec != ""
}

// run 在 profile 目录内执行 git 命令。返回 stdout、stderr。
func run(p *paths.Paths, name string, args ...string) (string, string, error) {
	if !Available() {
		return "", "", ErrGitNotFound
	}
	cmd := exec.Command(gitExec, args...)
	cmd.Dir = p.ProfileDir(name)
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if err != nil {
		return out.String(), errOut.String(), err
	}
	return out.String(), errOut.String(), nil
}

// IsRepo 判断 profile 目录是否已初始化 git 仓库（存在 .git 目录）。
func IsRepo(p *paths.Paths, name string) bool {
	_, err := os.Stat(filepath.Join(p.ProfileDir(name), ".git"))
	return err == nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/git/ -v`
预期：PASS，`TestIsRepoFalse` 与 `TestIsRepoTrue` 均通过

- [ ] **步骤 5：Commit**

```bash
git add internal/git/git.go internal/git/git_test.go
git commit -m "feat: add git run/IsRepo primitives"
```

---

### 任务 2：`internal/git` — Init（git init + .gitignore + 初始 commit）

**文件：**
- 修改：`internal/git/git.go`
- 测试：`internal/git/git_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
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
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/git/ -run TestInit -v`
预期：FAIL，报错 "undefined: Init"

- [ ] **步骤 3：编写最少实现代码**（追加到 git.go）

```go
// trackedFiles 返回 profile 中被 git 跟踪的三个文件，均相对 profile 目录。
var trackedFiles = []string{"opencode.json", "tui.json", "skills.yml"}

// Init 在 profile 目录初始化 git 仓库并做首次提交。
func Init(p *paths.Paths, name string) error {
	if !Available() {
		return ErrGitNotFound
	}
	if IsRepo(p, name) {
		return errors.New("profile '" + name + "' is already under version control")
	}
	gitignore := filepath.Join(p.ProfileDir(name), ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		if err := os.WriteFile(gitignore, []byte("skills/\n"), 0o644); err != nil {
			return err
		}
	}
	if _, _, err := run(p, name, "init", "-q"); err != nil {
		return err
	}
	// --ignore-missing：tui.json/skills.yml 可能不存在（如 -e 创建的 profile）
	if _, _, err := run(p, name, "add", "--ignore-missing", "--", "opencode.json", "tui.json", "skills.yml", ".gitignore"); err != nil {
		return err
	}
	if _, _, err := run(p, name, "commit", "-m", "chore: initial commit for profile "+name); err != nil {
		return err
	}
	return nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/git/ -run TestInit -v`
预期：PASS，两个测试均通过；`.gitignore` 内容为 `skills/\n`

- [ ] **步骤 5：Commit**

```bash
git add internal/git/git.go internal/git/git_test.go
git commit -m "feat: add git Init with first commit"
```

---

### 任务 3：`internal/git` — Commit 与 Log

**文件：**
- 修改：`internal/git/git.go`
- 测试：`internal/git/git_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
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
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/git/ -run TestCommitAndLog -v`
预期：FAIL，报错 "undefined: Commit" / "undefined: Log"

- [ ] **步骤 3：编写最少实现代码**（追加到 git.go）

```go
// errNotRepo 表示 profile 未启用版本管理。
func errNotRepo(name string) error {
	return errors.New("profile '" + name + "' is not under version control; run --git-init first")
}

// ensureRepo 返回 nil 或 errNotRepo。
func ensureRepo(p *paths.Paths, name string) error {
	if !Available() {
		return ErrGitNotFound
	}
	if !IsRepo(p, name) {
		return errNotRepo(name)
	}
	return nil
}

// Commit 提交 profile 的三个被跟踪文件，消息缺省时用自动消息。
func Commit(p *paths.Paths, name, message string) error {
	if err := ensureRepo(p, name); err != nil {
		return err
	}
	if message == "" {
		message = "chore: update profile " + name
	}
	if _, _, err := run(p, name, "add", "--ignore-missing", "--", "opencode.json", "tui.json", "skills.yml"); err != nil {
		return err
	}
	if _, _, err := run(p, name, "commit", "-m", message); err != nil {
		return err
	}
	return nil
}

// Log 返回 git log --oneline 输出（空仓库时返回 nil）。
func Log(p *paths.Paths, name string) (string, error) {
	if err := ensureRepo(p, name); err != nil {
		return "", err
	}
	out, _, err := run(p, name, "log", "--oneline")
	return out, err
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/git/ -run TestCommitAndLog -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/git/git.go internal/git/git_test.go
git commit -m "feat: add git Commit and Log"
```

---

### 任务 4：`internal/git` — Rollback（软回滚，脏状态拒绝）

**文件：**
- 修改：`internal/git/git.go`
- 测试：`internal/git/git_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
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
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/git/ -run TestRollback -v`
预期：FAIL，报错 "undefined: Rollback"

- [ ] **步骤 3：编写最少实现代码**（追加到 git.go）

```go
// isClean 报告 profile 工作区是否有未提交改动。
func isClean(p *paths.Paths, name string) bool {
	out, _, err := run(p, name, "status", "--porcelain")
	return err == nil && strings.TrimSpace(out) == ""
}

// Rollback 将工作区三个文件恢复到指定 commit，保留提交历史。
// 工作区存在未提交改动时直接拒绝。
func Rollback(p *paths.Paths, name, commit string) error {
	if err := ensureRepo(p, name); err != nil {
		return err
	}
	if !isClean(p, name) {
		return errors.New("working tree has uncommitted changes; commit or stash first")
	}
	args := append([]string{"checkout", commit, "--"}, trackedFiles...)
	if _, _, err := run(p, name, args...); err != nil {
		return err
	}
	return nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/git/ -run TestRollback -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/git/git.go internal/git/git_test.go
git commit -m "feat: add git Rollback with dirty-tree guard"
```

---

### 任务 5：CLI — 四个子命令 flag 与互斥校验

**文件：**
- 修改：`cmd/opencode-profiles/main.go`
- 测试：`cmd/opencode-profiles/main_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
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
```

（需在 main_test.go 顶部 import 增加 `"os/exec"`。）

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./cmd/opencode-profiles/ -run TestGit -v`
预期：FAIL，`--git-init` 未被解析（flag provided but not defined）

- [ ] **步骤 3：实现代码**（main.go 修改）

在 `run` 的 var 块中新增（`main.go:26-38` 区域）：

```go
	var (
		// ...已有 flag...
		gitInitName     string
		gitCommitName   string
		gitLogName      string
		gitRollbackName string
		commitMessage   string
	)
```

注册 flag（放在 `fs.StringVar(&profileName, ...)` 之后）：

```go
	fs.StringVar(&gitInitName, "git-init", "", "Enable git version control for a profile")
	fs.StringVar(&gitCommitName, "git-commit", "", "Commit changes for a profile")
	fs.StringVar(&gitLogName, "git-log", "", "Show commit history for a profile")
	fs.StringVar(&gitRollbackName, "git-rollback", "", "Roll back a profile to a commit (soft)")
	fs.StringVar(&commitMessage, "m", "", "Commit message for --git-commit")
```

互斥校验（放在 `if diffFlag { ... }` 块之后）：

```go
	gitCmd := gitInitName != "" || gitCommitName != "" || gitLogName != "" || gitRollbackName != ""
	if gitCmd && (backupFlag || diffFlag || createName != "" || emptyName != "" || switchName != "" ||
		addSkillName != "" || removeSkillName != "" || listFlag || fromCurrent || fromProfile != "") {
		fmt.Fprintln(stderr, "Error: git commands cannot be combined with other commands")
		return 1
	}
	if gitRollbackName != "" && fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Error: --git-rollback requires exactly one commit reference")
		return 1
	}
	if commitMessage != "" && gitCommitName == "" {
		fmt.Fprintln(stderr, "Error: -m can only be used with --git-commit")
		return 1
	}
```

关键修改——放宽 `fs.NArg() > 0` 拒绝逻辑（`main.go:60-63`），放行 `--git-rollback` 的位置参数：

```go
	if !diffFlag && gitRollbackName == "" && fs.NArg() > 0 {
		fmt.Fprintln(stderr, "Unexpected argument: "+fs.Arg(0))
		return 2
	}
```

分发逻辑（放在 `switchName` 分支之前）：

```go
	} else if gitInitName != "" {
		if err := gitpkg.Init(p, gitInitName); err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Version control enabled for '%s'\n", gitInitName)
	} else if gitCommitName != "" {
		if err := gitpkg.Commit(p, gitCommitName, commitMessage); err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Committed changes for '%s'\n", gitCommitName)
	} else if gitLogName != "" {
		log, err := gitpkg.Log(p, gitLogName)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintln(stdout, log)
	} else if gitRollbackName != "" {
		if err := gitpkg.Rollback(p, gitRollbackName, fs.Arg(0)); err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Rolled back '%s' to %s\n", gitRollbackName, fs.Arg(0))
	}
```

import 修改（`main.go:3-14`）：新增 `gitpkg "opencode-profiles/internal/git"`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./cmd/opencode-profiles/ -run TestGit -v`
预期：PASS；`go test ./...` 全绿

- [ ] **步骤 5：Commit**

```bash
git add cmd/opencode-profiles/main.go cmd/opencode-profiles/main_test.go
git commit -m "feat: add git versioning CLI commands"
```

---

### 任务 6：README 文档

**文件：**
- 修改：`README.md`

- [ ] **步骤 1：在"从 profile 移除技能"节后新增"Profile 版本管理"节**

```markdown
### Profile 版本管理

为 profile 启用 git 版本管理（需系统已安装 git，默认关闭）：

```bash
# 为 profile 启用版本管理（首次提交）
opencode-profiles --git-init work

# 提交改动
opencode-profiles --git-commit work -m "switch to zsh"

# 查看提交历史
opencode-profiles --git-log work

# 软回滚到历史版本（保留提交历史）
opencode-profiles --git-rollback work HEAD~1
```

每个 profile 独立 git 仓库，跟踪 `opencode.json`、`tui.json`、`skills.yml` 三个文件，`skills/` 目录自动排除。回滚前工作区有未提交改动时会拒绝。
```

- [ ] **步骤 2：验证渲染**

运行：`go test ./...`
预期：不受影响，全绿

- [ ] **步骤 3：Commit**

```bash
git add README.md
git commit -m "docs: document git versioning commands"
```

---

### 任务 7：全量验证与收尾

- [ ] **步骤 1：运行全量测试、vet、gofmt**

运行：
```bash
go test ./...
go vet ./...
gofmt -l .
```
预期：全部通过，gofmt 无输出

- [ ] **步骤 2：交叉编译验证**

运行：
```bash
GOOS=linux GOARCH=arm64 go build ./cmd/opencode-profiles
GOOS=darwin GOARCH=arm64 go build ./cmd/opencode-profiles
```
预期：构建成功

- [ ] **步骤 3：手动端到端冒烟测试**

运行：
```bash
go run ./cmd/opencode-profiles -e smoke 2>/dev/null || true
```
（对隔离环境；验证 `--git-init`/`--git-log` 输出符合预期，测试已覆盖主流程，此处可跳过）

- [ ] **步骤 4：确认无遗留**

运行：`git status`
预期：工作区干净，仅本次功能相关 commit
