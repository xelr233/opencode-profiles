# opencode-profiles

管理多个 opencode 配置 profile 的 CLI 工具，支持 per-profile 技能管理。用 Go 实现，编译为单个静态二进制，无运行时依赖。

## 安装

### 从源码构建

```bash
go build -trimpath -ldflags="-s -w" -o opencode-profiles ./cmd/opencode-profiles
sudo install -m 0755 opencode-profiles /usr/local/bin/
```

### 直接运行

```bash
go run ./cmd/opencode-profiles -h
```

## 使用方法

### 列出所有 profile

```bash
opencode-profiles -l
```

### 从当前配置创建新 profile

```bash
opencode-profiles -c work
```

### 创建空 profile

```bash
opencode-profiles -e minimal
```

### 切换 profile

```bash
opencode-profiles -s work
```

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

### 备份当前配置

```bash
opencode-profiles -b
```

### 为 profile 添加技能

```bash
opencode-profiles --add-skill brainstorming --profile work
```

### 从 profile 移除技能

```bash
opencode-profiles --remove-skill brainstorming --profile work
```

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

## 目录结构

```
~/.config/opencode/
├── opencode.json -> profiles/personal/opencode.json  (symlink)
├── skills/                                      (技能 symlinks)
│   ├── brainstorming -> ~/.cc-switch/skills/brainstorming
│   └── rtk -> ~/.cc-switch/skills/rtk
└── profiles/
    ├── personal/
    │   ├── opencode.json
    │   └── skills.yml                         (技能列表)
    └── work/
        ├── opencode.json
        └── skills.yml
```

- `~/.config/opencode/opencode.json` 始终是指向当前激活 profile 的符号链接
- 每个 profile 的 `skills.yml` 记录激活的技能列表
- 切换 profile 时，`~/.config/opencode/skills/` 下的 symlinks 会同步更新
- 技能源目录默认位于 `~/.cc-switch/skills/`

## 开发

```bash
# 运行测试
go test ./...

# 代码风格与静态检查
go fmt ./...
go vet ./...

# 构建
go build -trimpath -ldflags="-s -w" -o opencode-profiles ./cmd/opencode-profiles
```

## 技术栈

- Go 1.25+
- gopkg.in/yaml.v3 — YAML 解析
- modernc.org/sqlite — 纯 Go SQLite 驱动（cc-switch.db 集成）
