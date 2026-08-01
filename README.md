# opencode-profiles

管理多个 opencode 配置 profile 的 CLI 工具，支持 per-profile 技能管理。

## 安装

### 开发模式

```bash
uv pip install -e .
```

### 全局安装（推荐）

```bash
uv tool install --from dist/opencode_profiles-0.1.0-py3-none-any.whl opencode-profiles
```

或从源码构建后安装：

```bash
uv build
uv tool install --from dist/opencode_profiles-0.1.0-py3-none-any.whl opencode-profiles
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
uv run pytest -v

# 代码风格与格式检查
uv run ruff check .
uv run ruff format --check .

# 类型检查
uv run ty check opencode_profiles/
```

## 技术栈

- Python 3.11+
- click — CLI 框架
- uv — 包管理
- pytest — 测试
