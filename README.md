# opencode-profiles

管理多个 opencode 配置 profile 的 CLI 工具。

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

## 目录结构

```
~/.config/opencode/
├── opencode.json -> profiles/personal/opencode.json  (symlink)
└── profiles/
    ├── personal/
    │   ├── opencode.json
    │   └── skills/              (预留)
    └── work/
        ├── opencode.json
        └── skills/              (预留)
```

- `~/.config/opencode/opencode.json` 始终是指向当前激活 profile 的符号链接
- 每个 profile 是独立目录，包含 `opencode.json` 和预留的 `skills/` 子目录

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
