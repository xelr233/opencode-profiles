# opencode-profiles 设计文档

## 概述

opencode-profiles 是一个 CLI 工具，用于管理多个 opencode 配置（profile）。通过符号链接机制实现配置间的快速切换，每个 profile 以独立目录形式存储。

## 目录结构

```
~/.config/opencode/
├── opencode.json -> profiles/personal/opencode.json  (symlink，指向当前激活的 profile)
└── profiles/
    ├── personal/
    │   ├── opencode.json
    │   └── skills/              (预留，暂不管理)
    └── work/
        ├── opencode.json
        └── skills/              (预留，暂不管理)
```

- 别名（alias）即为 profile 目录名（如 `personal`、`work`）
- 每个 profile 目录包含 `opencode.json` 和预留的 `skills/` 子目录
- `~/.config/opencode/opencode.json` 始终是 symlink，指向当前激活 profile 的配置文件

## CLI 命令

采用简洁风格（短标志位）：

| 命令 | 说明 |
|------|------|
| `opencode-profiles -b` | 备份当前配置到 `profiles/backup_<timestamp>/` |
| `opencode-profiles -c <name>` | 从当前 `opencode.json` 内容创建新 profile |
| `opencode-profiles -e <name>` | 创建空 profile（最小合法 JSON `{}`） |
| `opencode-profiles -s <name>` | 切换 symlink 指向目标 profile |
| `opencode-profiles -l` | 列出所有 profile，标注当前激活项 |

## 核心流程

### 初始化（首次运行）

如果 `~/.config/opencode/opencode.json` 不是 symlink：
1. 创建 `profiles/` 目录
2. 将当前配置复制到 `profiles/default/opencode.json`
3. 创建预留子目录 `profiles/default/skills/`
4. 备份原文件为 `opencode.json.bak`（安全保留）
5. 将 `opencode.json` 替换为 symlink → `profiles/default/opencode.json`

### 备份（-b）

1. 读取当前 `opencode.json` 实际内容（通过 symlink 解析或当前激活 profile）
2. 创建 `profiles/backup_<YYYYMMDD_HHMMSS>/opencode.json`
3. 创建预留 `skills/` 子目录

### 从当前配置创建 profile（-c <name>）

1. 检查 `profiles/<name>/` 是否已存在，存在则报错
2. 复制当前激活 profile 的 `opencode.json` 到 `profiles/<name>/opencode.json`
3. 创建预留 `skills/` 子目录

### 创建空 profile（-e <name>）

1. 检查 `profiles/<name>/` 是否已存在，存在则报错
2. 写入 `{}` 到 `profiles/<name>/opencode.json`
3. 创建预留 `skills/` 子目录

### 切换 profile（-s <name>）

1. 检查 `profiles/<name>/opencode.json` 是否存在
2. 更新 symlink：`ln -sfn profiles/<name>/opencode.json ~/.config/opencode/opencode.json`
3. 输出切换结果

### 列出 profile（-l）

1. 遍历 `profiles/` 下所有子目录
2. 解析当前 symlink 目标，确定激活项
3. 输出列表，激活项标记 `*`

## 技术选型

| 方面 | 选择 | 理由 |
|------|------|------|
| CLI 框架 | click | 轻量、声明式、Python CLI 主流选择 |
| 文件操作 | pathlib + os + shutil | 标准库，无需额外依赖 |
| 配置存储 | JSON 文件 | 与 opencode 配置格式一致 |
| 切换机制 | symlink | 零冗余、即时生效 |

## 入口点配置

`pyproject.toml` 中配置：
```toml
[project.scripts]
opencode-profiles = "opencode_profiles.cli:main"
```

## 错误处理

- profile 已存在时：友好报错，不覆盖
- profile 不存在时（切换）：提示可用 profile 列表
- symlink 操作失败：保留原状态，输出错误信息
- `~/.config/opencode/` 不存在：提示先安装/运行 opencode

## 安全考虑

- 替换原文件前创建 `.bak` 备份
- 不删除任何 profile，除非显式命令
- symlink 使用相对路径，保证目录移动后仍可用
