# 空配置 Provider 导入设计

日期：2026-07-25

## 背景

`opencode-profiles -e <name>` 当前创建仅包含 `{}` 的空配置。用户希望在创建空配置时，能够从当前激活的配置或指定 profile 中导入 `provider` 字段，避免手动重新配置 provider。

## 需求

1. `-e <name>` 行为不变（向后兼容）
2. `-e <name> --from-current`：从当前激活的 profile 复制全部 provider
3. `-e <name> --from-profile <source>`：从指定 profile 复制全部 provider
4. 源配置无 provider 或 provider 为空时报错
5. `--from-current` 与 `--from-profile` 互斥
6. 二者仅在配合 `-e` 时使用，单独使用报错

## 方案设计

### ops.py

扩展 `create_empty` 签名：

```python
def create_empty(paths: OpenCodePaths, name: str, source: str | None = None) -> None:
```

- `source=None`：写入 `{}`（现有行为）
- `source="current"`：从当前激活 profile 读取 provider
- `source="<profile_name>"`：从指定 profile 读取 provider
- 写入 `{"provider": <extracted>}`

新增内部辅助函数：

```python
def _load_providers(paths: OpenCodePaths, source: str) -> dict:
    """从源配置读取 provider dict。source 为 'current' 或 profile 名。
    
    Raises:
        FileNotFoundError: 源配置不存在
        ValueError: 源配置无 provider 或 provider 为空
    """
```

读取逻辑：
- `source="current"`：解析 symlink → 读取目标 `opencode.json`
- 其他值：读取 `paths.profile_config(source)`
- JSON 解析后检查 `provider` 键是否存在且非空 dict

### cli.py

新增两个全局选项：

```python
@click.option("--from-current", is_flag=True, help="从当前配置导入 provider（配合 -e 使用）")
@click.option("--from-profile", type=str, help="从指定 profile 导入 provider（配合 -e 使用）")
```

验证规则（在 `main` 函数入口）：
- `--from-current` 和 `--from-profile` 同时存在 → `ClickException`
- `--from-current` 或 `--from-profile` 存在但无 `-e` → `ClickException`

在 `elif empty:` 分支：
- 仅有 `-e` → `create_empty(paths, name)`
- `-e` + `--from-current` → `create_empty(paths, name, source="current")`
- `-e` + `--from-profile <src>` → `create_empty(paths, name, source=src)`

### 错误处理

| 场景 | 错误消息 |
|------|---------|
| `--from-profile` 指定的 profile 不存在 | `Source profile 'xxx' not found` |
| 源配置无 provider 或为空 | `Source config has no providers to import` |
| `--from-current`/`--from-profile` 无 `-e` | `--from-current/--from-profile can only be used with -e` |
| 同时传 `--from-current` 和 `--from-profile` | `--from-current and --from-profile are mutually exclusive` |
| profile 已存在 | 保持现有行为（`FileExistsError`） |

### 输出示例

```bash
$ opencode-profiles -e work --from-current
Created profile 'work' with providers from current config

$ opencode-profiles -e work --from-profile personal
Created profile 'work' with providers from 'personal'

$ opencode-profiles -e minimal
Created empty profile 'minimal'
```

## 测试计划

1. `-e <name> --from-current`：验证新 profile 的 provider 与当前配置一致
2. `-e <name> --from-profile <src>`：验证从指定 profile 复制 provider
3. 源配置无 provider → 抛出 ValueError
4. 源 profile 不存在 → 抛出 FileNotFoundError
5. `--from-current` 与 `--from-profile` 同时使用 → ClickException
6. `--from-current` / `--from-profile` 无 `-e` → ClickException
7. `-e` 已有 profile → 保持 FileExistsError
8. 纯 `-e`（无 source 参数）→ 向后兼容，写入 `{}`

## 影响范围

- `opencode_profiles/ops.py`：修改 `create_empty`，新增 `_load_providers`
- `opencode_profiles/cli.py`：新增两个 option，增加验证逻辑
- `tests/test_ops.py`：新增测试用例
- `tests/test_cli.py`：新增 CLI 集成测试
