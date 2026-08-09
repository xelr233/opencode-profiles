# opencode-profiles

管理多个 opencode 配置 profile 的 CLI 工具，支持 per-profile 技能管理。用 Go 实现，编译为单个静态二进制，无运行时依赖。

## 安装

### 从源码构建

```bash
# 构建（含版本号注入；未指定 version 时输出 "dev"）
go build -trimpath -ldflags="-s -w -X main.version=v0.3.0" -o opencode-profiles ./cmd/opencode-profiles
sudo install -m 0755 opencode-profiles /usr/local/bin/
```

### 直接运行

```bash
go run ./cmd/opencode-profiles -h
go run ./cmd/opencode-profiles --version
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
