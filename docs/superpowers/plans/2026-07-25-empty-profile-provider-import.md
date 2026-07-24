# 空配置 Provider 导入 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 扩展 `-e/--empty` 命令，支持 `--from-current` 和 `--from-profile` 参数从源配置导入 provider

**架构：** 修改 `create_empty` 增加 `source` 参数，新增内部辅助 `_load_providers` 读取源配置的 provider 字段；CLI 新增两个全局选项并在入口验证互斥

**技术栈：** Python 3.11+, click, pytest

---

## 文件结构

- `opencode_profiles/ops.py` — 修改 `create_empty`，新增 `_load_providers`
- `opencode_profiles/cli.py` — 新增 `--from-current` / `--from-profile` 选项及验证逻辑
- `tests/test_ops.py` — 新增 ops 层测试
- `tests/test_cli.py` — 新增 CLI 集成测试

---

## 任务

### 任务 1：添加 `_load_providers` 辅助函数

**文件：**
- 修改：`opencode_profiles/ops.py:78`
- 测试：`tests/test_ops.py`

- [ ] **步骤 1：编写失败的测试**

在 `tests/test_ops.py` 末尾添加：

```python
import json
from opencode_profiles.ops import create_empty


class TestCreateEmptyWithSource:
    """测试 create_empty 的 source 参数功能。"""

    def test_create_empty_with_from_current(self, paths, existing_config, sample_config):
        """从当前配置导入 provider 创建新 profile。"""
        from opencode_profiles.ops import ensure_initialized
        ensure_initialized(paths)
        create_empty(paths, "work", source="current")
        content = json.loads(paths.profile_config("work").read_text())
        assert content == {"provider": sample_config["provider"]}

    def test_create_empty_with_from_profile(self, paths, existing_config, sample_config):
        """从指定 profile 导入 provider 创建新 profile。"""
        from opencode_profiles.ops import ensure_initialized, create_from_current
        ensure_initialized(paths)
        create_from_current(paths, "personal")
        create_empty(paths, "work", source="personal")
        content = json.loads(paths.profile_config("work").read_text())
        assert content == {"provider": sample_config["provider"]}

    def test_create_empty_source_not_found(self, paths, existing_config):
        """源 profile 不存在时报错。"""
        from opencode_profiles.ops import ensure_initialized
        ensure_initialized(paths)
        with pytest.raises(FileNotFoundError):
            create_empty(paths, "work", source="nonexistent")

    def test_create_empty_source_no_provider(self, paths, existing_config, tmp_path):
        """源配置无 provider 时报错。"""
        from opencode_profiles.ops import ensure_initialized
        ensure_initialized(paths)
        # 创建一个无 provider 的 profile
        no_provider_dir = paths.profile_dir("no_provider")
        no_provider_dir.mkdir(parents=True)
        paths.profile_config("no_provider").write_text('{"shell": "bash"}')
        paths.profile_skills("no_provider").mkdir(exist_ok=True)
        with pytest.raises(ValueError, match="no providers"):
            create_empty(paths, "work", source="no_provider")

    def test_create_empty_backward_compatible(self, paths, existing_config):
        """不传 source 时行为不变（写入 {}）。"""
        from opencode_profiles.ops import ensure_initialized
        ensure_initialized(paths)
        create_empty(paths, "empty")
        content = json.loads(paths.profile_config("empty").read_text())
        assert content == {}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`pytest tests/test_ops.py::TestCreateEmptyWithSource -v`
预期：FAIL，报错 `unexpected keyword argument 'source'` 或 `_load_providers` 相关

- [ ] **步骤 3：修改 `create_empty` 并新增 `_load_providers`**

将 `opencode_profiles/ops.py` 中 `create_empty` 函数替换为：

```python
def _load_providers(paths: OpenCodePaths, source: str) -> dict:
    """从源配置读取 provider dict。source 为 'current' 或 profile 名。

    Raises:
        FileNotFoundError: 源配置不存在
        ValueError: 源配置无 provider 或 provider 为空
    """
    if source == "current":
        config = paths.config_file
        if not config.is_symlink():
            raise FileNotFoundError("Current config is not a symlink")
        target = config.resolve()
        data = json.loads(target.read_text())
    else:
        config_path = paths.profile_config(source)
        if not config_path.exists():
            raise FileNotFoundError(f"Source profile '{source}' not found")
        data = json.loads(config_path.read_text())

    providers = data.get("provider")
    if not providers:
        raise ValueError(f"Source config has no providers to import")
    return providers


def create_empty(paths: OpenCodePaths, name: str, source: str | None = None) -> None:
    """创建空 profile，可选从源配置导入 provider。

    Args:
        paths: 路径管理实例
        name: 新 profile 名称
        source: None 表示空配置；"current" 表示从当前激活配置导入；
                其他字符串表示从指定 profile 名称导入
    """
    ensure_initialized(paths)

    profile_dir = paths.profile_dir(name)
    if profile_dir.exists():
        raise FileExistsError(f"Profile '{name}' already exists")

    profile_dir.mkdir(parents=True)
    paths.profile_skills(name).mkdir(exist_ok=True)

    if source is None:
        paths.profile_config(name).write_text("{}")
    else:
        providers = _load_providers(paths, source)
        paths.profile_config(name).write_text(
            json.dumps({"provider": providers}, indent=2)
        )
```

- [ ] **步骤 4：运行测试验证通过**

运行：`pytest tests/test_ops.py::TestCreateEmptyWithSource -v`
预期：全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add opencode_profiles/ops.py tests/test_ops.py
git commit -m "feat(ops): support importing providers in create_empty"
```

---

### 任务 2：扩展 CLI 选项

**文件：**
- 修改：`opencode_profiles/cli.py`
- 测试：`tests/test_cli.py`

- [ ] **步骤 1：编写失败的测试**

在 `tests/test_cli.py` 添加 CLI 集成测试。先读取现有内容确认 import 结构：

```python
# 在 tests/test_cli.py 末尾添加

class TestEmptyWithProviderImport:
    """测试 -e 配合 --from-current / --from-profile。"""

    def test_empty_from_current(self, paths, existing_config, sample_config):
        """CLI: -e work --from-current"""
        from click.testing import CliRunner
        from opencode_profiles.cli import main
        # 需要将 paths 注入 CLI，见步骤 3 说明
        runner = CliRunner()
        # 临时替换全局 paths
        import opencode_profiles.cli as cli_module
        original_paths = cli_module.paths
        cli_module.paths = paths
        try:
            result = runner.invoke(main, ["-e", "work", "--from-current"])
            assert result.exit_code == 0
            assert "providers from current" in result.output
            content = json.loads(paths.profile_config("work").read_text())
            assert content == {"provider": sample_config["provider"]}
        finally:
            cli_module.paths = original_paths

    def test_empty_from_profile(self, paths, existing_config, sample_config):
        """CLI: -e work --from-profile personal"""
        from click.testing import CliRunner
        from opencode_profiles.cli import main, paths as cli_paths
        from opencode_profiles.ops import ensure_initialized, create_from_current
        ensure_initialized(paths)
        create_from_current(paths, "personal")
        runner = CliRunner()
        import opencode_profiles.cli as cli_module
        original_paths = cli_module.paths
        cli_module.paths = paths
        try:
            result = runner.invoke(main, ["-e", "work", "--from-profile", "personal"])
            assert result.exit_code == 0
            assert "providers from" in result.output
            content = json.loads(paths.profile_config("work").read_text())
            assert content == {"provider": sample_config["provider"]}
        finally:
            cli_module.paths = original_paths

    def test_from_current_without_empty(self, paths, existing_config):
        """--from-current 无 -e 时报错。"""
        from click.testing import CliRunner
        from opencode_profiles.cli import main
        runner = CliRunner()
        result = runner.invoke(main, ["--from-current"])
        assert result.exit_code != 0
        assert "can only be used with -e" in result.output

    def test_from_profile_without_empty(self, paths, existing_config):
        """--from-profile 无 -e 时报错。"""
        from click.testing import CliRunner
        from opencode_profiles.cli import main
        runner = CliRunner()
        result = runner.invoke(main, ["--from-profile", "personal"])
        assert result.exit_code != 0
        assert "can only be used with -e" in result.output

    def test_mutually_exclusive(self, paths, existing_config):
        """--from-current 和 --from-profile 同时传报错。"""
        from click.testing import CliRunner
        from opencode_profiles.cli import main
        runner = CliRunner()
        result = runner.invoke(main, ["-e", "work", "--from-current", "--from-profile", "x"])
        assert result.exit_code != 0
        assert "mutually exclusive" in result.output

    def test_empty_from_current_no_provider(self, paths, existing_config):
        """源配置无 provider 时 CLI 报错。"""
        from click.testing import CliRunner
        from opencode_profiles.cli import main
        from opencode_profiles.ops import ensure_initialized
        ensure_initialized(paths)
        # 当前 default 有 provider，先创建一个无 provider 的 profile 并切换
        no_prov_dir = paths.profile_dir("no_prov")
        no_prov_dir.mkdir(parents=True)
        paths.profile_config("no_prov").write_text('{"shell": "bash"}')
        paths.profile_skills("no_prov").mkdir(exist_ok=True)
        # 切换当前到 no_prov
        import os
        cfg = paths.config_file
        if cfg.is_symlink() or cfg.exists():
            cfg.unlink()
        os.symlink(paths.relative_target("no_prov"), cfg)
        runner = CliRunner()
        import opencode_profiles.cli as cli_module
        original_paths = cli_module.paths
        cli_module.paths = paths
        try:
            result = runner.invoke(main, ["-e", "work", "--from-current"])
            assert result.exit_code != 0
            assert "no providers" in result.output.lower()
        finally:
            cli_module.paths = original_paths
```

- [ ] **步骤 2：运行测试验证失败**

运行：`pytest tests/test_cli.py::TestEmptyWithProviderImport -v`
预期：FAIL，`unexpected keyword argument` 或 option 不存在

- [ ] **步骤 3：修改 cli.py**

将 `opencode_profiles/cli.py` 替换为：

```python
import click

from opencode_profiles.ops import (
    backup,
    create_empty,
    create_from_current,
    get_active,
    list_profiles,
    switch,
)
from opencode_profiles.paths import OpenCodePaths


paths = OpenCodePaths()


@click.command()
@click.option("-b", "--backup", "backup_flag", is_flag=True, help="备份当前配置")
@click.option("-c", "--create", type=str, help="从当前配置创建新 profile")
@click.option("-e", "--empty", type=str, help="创建空 profile")
@click.option("-s", "--switch", "switch_name", type=str, help="切换到指定 profile")
@click.option("-l", "--list", "list_flag", is_flag=True, help="列出所有 profile")
@click.option("--from-current", is_flag=True, help="从当前配置导入 provider（配合 -e 使用）")
@click.option("--from-profile", type=str, help="从指定 profile 导入 provider（配合 -e 使用）")
def main(backup_flag, create, empty, switch_name, list_flag, from_current, from_profile):
    """opencode-profiles — 多配置管理工具。"""
    # 验证 --from-current / --from-profile 使用条件
    if from_current and from_profile:
        raise click.ClickException("--from-current and --from-profile are mutually exclusive")
    if (from_current or from_profile) and not empty:
        raise click.ClickException("--from-current/--from-profile can only be used with -e")

    if backup_flag:
        name = backup(paths)
        click.echo(f"Backed up to '{name}'")
    elif create:
        try:
            create_from_current(paths, create)
            click.echo(f"Created profile '{create}' from current config")
        except FileExistsError as e:
            raise click.ClickException(str(e))
    elif empty:
        try:
            if from_current:
                create_empty(paths, empty, source="current")
                click.echo(f"Created profile '{empty}' with providers from current config")
            elif from_profile:
                create_empty(paths, empty, source=from_profile)
                click.echo(f"Created profile '{empty}' with providers from '{from_profile}'")
            else:
                create_empty(paths, empty)
                click.echo(f"Created empty profile '{empty}'")
        except FileExistsError as e:
            raise click.ClickException(str(e))
        except FileNotFoundError as e:
            raise click.ClickException(str(e))
        except ValueError as e:
            raise click.ClickException(str(e))
    elif switch_name:
        try:
            switch(paths, switch_name)
            click.echo(f"Switched to '{switch_name}'")
        except FileNotFoundError as e:
            raise click.ClickException(str(e))
    elif list_flag:
        profiles = list_profiles(paths)
        active = get_active(paths)
        if not profiles:
            click.echo("No profiles found.")
            return
        for p in profiles:
            marker = " *" if p == active else ""
            click.echo(f"  {p}{marker}")
        if active:
            click.echo(f"\nActive: {active}")
    else:
        click.echo("Use --help for available commands.")
```

- [ ] **步骤 4：运行测试验证通过**

运行：`pytest tests/test_cli.py::TestEmptyWithProviderImport -v`
预期：全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add opencode_profiles/cli.py tests/test_cli.py
git commit -m "feat(cli): add --from-current and --from-profile options for -e"
```

---

### 任务 3：运行完整测试套件验证无回归

- [ ] **步骤 1：运行全部测试**

运行：`pytest -v`
预期：全部 PASS（原有 31 + 新增测试）

- [ ] **步骤 2：如有失败则修复**

若原测试因 `create_empty` 输出格式变化（indent=2）而失败，更新断言：

```python
# 原测试中 assert content == {} 不受影响
# 若有字符串比较则可能需要调整
```

- [ ] **步骤 3：Commit（如有修复）**

```bash
git add -A
git commit -m "fix: adjust tests for provider import feature"
```
