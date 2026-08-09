# Profile 导出导入设计规格

日期：2026-08-09
状态：已批准

## 目标

为 opencode-profiles 添加 profile 导出与导入能力：

- **导出**：将 profile 打包为 zip，导出的 `opencode.json` **不包含任何 provider**。
- **可选**：导出源 skills 压缩包（zip），默认不开启。
- **导入**：将 zip 还原为 profile，支持指定新名字；可携带 skills 一并还原。

## 需求

### 导出 `export`

```
opencode-profiles export <name> [--with-skills] [--out <dir>]
```

- 默认生成 **1 个 zip**：`<name>.zip`。
- `--with-skills` 时额外生成 **1 个 zip**：`<name>-skills.zip`。
- `--out <dir>` 指定输出目录（默认当前目录）。
- profile 不存在时报错。
- `--out` 目录不可写时报错。

**`<name>.zip` 内部结构**（平铺）：

| 文件 | 说明 |
|------|------|
| `opencode.json` | 原配置 **删除 `provider` 键**，其余字段原样保留 |
| `skills.yml` | 存在才包含 |
| `tui.json` | 存在才包含 |

**`<name>-skills.zip` 内部结构**：

- skills.yml 中引用的每个技能一个子目录 `<skill>/`，打包其源目录（`~/.cc-switch/skills/<skill>/`）全部内容。
- 本地源缺失的技能**跳过并打印 warning**，不中断导出。

### 导入 `import`

```
opencode-profiles import <file.zip> [--name <new>] [--skills <file.zip>]
```

- **默认 profile 名**：zip 文件名去 `.zip` 后缀。
- `--name <new>`：指定导入名。
- 目标 profile 已存在 → **报错退出**，提示用 `--name` 指定新名（不覆盖）。
- **仅导入不切换**当前激活 profile。
- `--skills <file.zip>`：显式指定 skills zip。
- 未显式指定时，若同目录存在 `<name>-skills.zip`（与 profile zip 同 basename），**自动关联导入**。

**导入步骤**：

1. 校验 zip 内 `opencode.json` 存在，否则报错"无效的 profile zip"。
2. 目标 profile 已存在则报错（见上）。
3. 解压到 `profiles/<name>/`：`opencode.json`、`skills.yml`、`tui.json` 写回。
4. `skills.yml` 原样保留；对本地 `~/.cc-switch/skills/` 中不存在的技能打印 warning，**不创建软链接**。
5. skills zip（自动或显式）：
   - 解压到 `~/.cc-switch/skills/<skill>/`，**同名技能源已存在则跳过**，不覆盖。
   - 对存在的技能在 `~/.config/opencode/skills/<skill>` 创建软链接，**同名已存在则跳过**。

## 架构

### 新包 `internal/export`

- `Export(p *paths.Paths, name, outDir string, withSkills bool, warn io.Writer) error`
  - 生成 `<name>.zip`（删除 provider 后的配置 + skills.yml + tui.json）。
  - `withSkills` 时生成 `<name>-skills.zip`。
  - warning 写入 `warn`（调用方注入，通常 stderr）。
- `Import(p *paths.Paths, zipPath, name, skillsZipPath string, warn io.Writer) error`
  - 校验、解压、写回；`skillsZipPath` 为空时尝试同目录 `<name>-skills.zip` 自动关联。
  - warning 写入 `warn`。

### 删除 provider

- 读取 `opencode.json` 为 `map[string]any`，`delete(data, "provider")`，重新 marshal。
- 使用 `json.MarshalIndent(data, "", "  ")` 与现有写入格式保持一致。
- 配置非 JSON 或读失败时报错。

### zip 读写

- 使用标准库 `archive/zip`，无需新依赖。
- 导出：`zip.NewWriter` + `Create` + `io.Copy`。
- 导入：`zip.OpenReader` + 逐条目解压（限制路径穿越：拒绝 `..` 与绝对路径条目）。

### CLI 接线（`cmd/opencode-profiles/main.go`）

- 新增 `export` / `import` 两个子命令，通过**首个位置参数**识别。
- 与现有 flag 风格共存：`fs.Arg(0)` 为 `export`/`import` 时走子命令分支。
- 子命令 flag：`--with-skills`、`--out`、`--name`、`--skills`（解析时移除首位置参数）。
- 与其他命令互斥。

## 错误处理

| 场景 | 行为 |
|------|------|
| 导出不存在的 profile | 报错，退出码 1 |
| `--out` 目录不可写 | 报错，退出码 1 |
| 导入 zip 无 `opencode.json` | 报错"无效的 profile zip"，退出码 1 |
| 目标 profile 已存在 | 报错并提示 `--name`，退出码 1 |
| 源 skills 缺失（导出） | warning 跳过，不中断 |
| 本地技能缺失（导入） | warning，不创建软链接 |
| skills 同名已存在（导入） | 跳过，不覆盖 |

## 测试

- 单元测试（`internal/export/export_test.go`）用 `t.TempDir()` 构造隔离的 `Paths`：
  - 导出：构造 profile（含 provider、skills.yml、tui.json）→ 导出 → 断言 zip 内容（无 provider 键、含 skills.yml/tui.json、`--with-skills` 时生成第二个 zip）。
  - 导入：构造 zip → 导入 → 断言 profile 文件内容、同名跳过逻辑、warning 输出。
  - 无效 zip（无 opencode.json）报错。
  - 目标已存在报错。
- CLI 测试（`cmd/opencode-profiles/main_test.go`）：`run(args, ...)` 断言导出/导入退出码与输出。
- `--with-skills` 缺源 warning 路径。
- `--skills` 显式与自动关联两条路径。

## 不做的事（YAGNI）

- 不自动切换激活 profile。
- 不做加密、版本化导出格式。
- 不导出 `skills/` 目录本身（symlink 由 skills.yml 驱动重建）。
- 不导入 git 历史（`.git` 目录不导出）。
