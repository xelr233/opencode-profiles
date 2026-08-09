# Profile Git 版本管理设计

日期：2026-08-09

## 目标

为 profile 添加可选的 git 版本管理能力。每个 profile 使用独立的 git 仓库，支持手动创建 commit、查看历史、软回滚。该功能默认关闭，创建 profile 时自动生成的目录不含 git 仓库；用户可随时对已存在的 profile 启用。

## 约束

- 依赖系统 git 二进制；git 未安装时 `--git-init` 直接报错并提示安装。
- 零新增外部 Go 依赖（调用 `git` 可执行文件，不引入 go-git）。
- 沿用现有架构：路径解析在 `internal/paths`，业务逻辑在 `internal/ops`，CLI 在 `cmd/opencode-profiles/main.go`。

## 设计

### 1. 新包 `internal/git`

封装对 git 二进制的调用。职责单一：在指定 profile 目录内执行 git 命令，并区分「git 未安装」与「git 操作失败」。

```go
// Available 检测系统中是否安装了 git。
func Available() bool

// run 在 profile 目录内执行 git 命令，返回 stdout/stderr。
func run(p *paths.Paths, name string, args ...string) (string, string, error)

// IsRepo 判断 profile 是否已初始化 git 仓库（存在 .git 目录）。
func IsRepo(p *paths.Paths, name string) bool

// Init 初始化仓库：git init、写 .gitignore、git add 三文件、初始 commit。
func Init(p *paths.Paths, name string) error

// Commit 提交当前改动：git add 三文件、git commit -m。
func Commit(p *paths.Paths, name, message string) error

// Log 返回 git log --oneline 输出。
func Log(p *paths.Paths, name string) (string, error)

// Rollback 将工作区文件恢复到指定 commit：git checkout <commit> -- <files>。
func Rollback(p *paths.Paths, name, commit string) error
```

- `.gitignore` 内容固定为一行 `skills/`，排除 profile 目录下的技能 symlink 子目录。
- 跟踪文件：`opencode.json`、`tui.json`、`skills.yml`（显式 `git add` 这三个文件，避免 `.gitignore` 意外放行其他内容）。
- 通过 `exec.Command` 在 `p.ProfileDir(name)` 目录执行；git 未安装时返回带提示的错误。

### 2. 错误与前置校验

- `Init`：`!git.Available()` → 报错「git 未安装」；`IsRepo` 已存在 → 报错「已初始化」。
- `Commit` / `Log` / `Rollback`：`!IsRepo` → 报错「未初始化，先 --git-init」。
- `Rollback`：先跑 `git status --porcelain`，若工作区有未提交改动则直接拒绝并提示先 commit 或 stash。
- `git` 命令失败时透传其 stderr 到错误信息。

### 3. CLI 子命令（`main.go`）

新增 flag，沿用现有 `-c` / `-e` / `-s` 单字符+长名称风格：

| Flag | 参数 | 说明 |
|---|---|---|
| `--git-init <name>` | profile 名 | 启用版本管理，初始化仓库并做首次提交 |
| `--git-commit <name>` | profile 名 | 提交当前改动；配合 `-m` 指定消息，缺省为自动消息 `chore: update profile <name>` |
| `--git-log <name>` | profile 名 | 打印 `git log --oneline` |
| `--git-rollback <name> <commit>` | profile 名 + commit 引用 | 软回滚工作区文件到指定 commit |

`-m <msg>` 与 `--git-commit` 组合使用；`--git-rollback` 通过 `fs.Args()` 取 commit 引用。

命令互斥校验：git 系列命令不能与 `-b`/`-d`/`-c`/`-e`/`-s`/`--add-skill`/`--remove-skill`/`-l` 组合，否则报错退出码 1。

### 4. 测试

- `internal/git/git_test.go`：用 `t.TempDir()` 构造隔离 `Paths`，通过真实 git 二进制跑 init → commit → log → rollback 全流程。
- 若系统未安装 git，`git.Available()` 为 false 时测试 `t.Skip()`。
- `cmd/opencode-profiles/main_test.go`：新增 `TestGitInit/Commit/Log/Rollback`，断言退出码与 stdout 内容。
- 复用 `run(args, &buf, &errBuf, paths, dbPath)` 可注入接口，不触碰真实 `~/.config/opencode`。

## 未纳入范围（YAGNI）

- 不提供 `--git-init` 之外的对 git 的交互式确认（`--force` 等）。
- 不做 push/远程仓库集成。
- 不在 `-c`/`-e` 创建 profile 时自动启用 git（保持默认关闭）。
