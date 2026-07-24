# opencode-profiles 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 构建一个 CLI 工具来管理多个 opencode 配置 profile，支持备份、创建、切换和列表功能。

**架构：** 以 symlink 机制实现配置切换——`~/.config/opencode/opencode.json` 始终是指向当前激活 profile 的符号链接。每个 profile 以独立目录存储，包含 `opencode.json` 和预留的 `skills/` 子目录。

**技术栈：** Python 3.11+、click（CLI 框架）、pathlib/shutil/os（文件操作，标准库）、pytest（测试）

---

## 文件结构

```
opencode_profiles/
├── __init__.py          # 包标记
├── paths.py             # 路径常量定义
├── ops.py               # 核心操作（init、backup、create、switch、list）
└── cli.py               # click CLI 入口
tests/
├── __init__.py
├── conftest.py          # 共享 fixture（临时目录、mock paths）
├── test_paths.py        # 路径常量测试
├── test_ops.py          # 核心操作测试
└── test_cli.py          # CLI 集成测试
pyproject.toml           # 添加 click 依赖和 [project.scripts] 入口
```

---

## 任务 1：项目依赖与入口配置

**文件：**
- 修改：`pyproject.toml`

- [ ] **步骤 1：添加 click 依赖和入口点**

在 `pyproject.toml` 中追加：

```toml
[project]
dependencies = ["click>=8.0"]

[project.scripts]
opencode-profiles = "opencode_profiles.cli:main"
```

- [ ] **步骤 2：同步依赖**

```bash
uv sync
```

- [ ] **步骤 3：Commit**

```bash
git add pyproject.toml pyproject.lock
git commit -m "build: add click dependency and CLI entry point"
```

---

## 任务 2：路径常量模块

**文件：**
- 创建：`opencode_profiles/__init__.py`
- 创建：`opencode_profiles/paths.py`
- 创建：`tests/__init__.py`
- 创建：`tests/test_paths.py`

- [ ] **步骤 1：编写失败测试**

`tests/test_paths.py`:

```python
from opencode_profiles.paths import OpenCodePaths


def test_paths_base_dir():
    p = OpenCodePaths()
    assert p.base_dir.name == "opencode"
    assert str(p.base_dir).endswith(".config/opencode")


def test_paths_config_file():
    p = OpenCodePaths()
    assert p.config_file.name == "opencode.json"


def test_paths_profiles_dir():
    p = OpenCodePaths()
    assert p.profiles_dir.name == "profiles"


def test_paths_profile_dir():
    p = OpenCodePaths()
    assert str(p.profile_dir("work")).endswith("profiles/work")


def test_paths_profile_config():
    p = OpenCodePaths()
    assert p.profile_config("work").name == "opencode.json"


def test_paths_profile_skills():
    p = OpenCodePaths()
    assert p.profile_skills("work").name == "skills"
```

- [ ] **步骤 2：运行测试验证失败**

```bash
uv run pytest tests/test_paths.py -v
```

预期：FAIL，报 `ModuleNotFoundError`

- [ ] **步骤 3：编写实现**

`opencode_profiles/__init__.py`:
```python
```

`opencode_profiles/paths.py`:
```python
from pathlib import Path


class OpenCodePaths:
    """管理 opencode 配置路径常量。"""

    def __init__(self, base_dir: Path | None = None):
        self._base_dir = base_dir or Path.home() / ".config" / "opencode"

    @property
    def base_dir(self) -> Path:
        return self._base_dir

    @property
    def config_file(self) -> Path:
        return self._base_dir / "opencode.json"

    @property
    def profiles_dir(self) -> Path:
        return self._base_dir / "profiles"

    def profile_dir(self, name: str) -> Path:
        return self._base_dir / "profiles" / name

    def profile_config(self, name: str) -> Path:
        return self.profile_dir(name) / "opencode.json"

    def profile_skills(self, name: str) -> Path:
        return self.profile_dir(name) / "skills"
```

- [ ] **步骤 4：运行测试验证通过**

```bash
uv run pytest tests/test_paths.py -v
```

预期：全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add opencode_profiles/paths.py opencode_profiles/__init__.py tests/test_paths.py tests/__init__.py
git commit -m "feat: add paths module for opencode config locations"
```

---

## 任务 3：测试 fixtures

**文件：**
- 创建：`tests/conftest.py`

- [ ] **步骤 1：编写共享 fixture**

`tests/conftest.py`:

```python
import json
import pytest
from pathlib import Path
from opencode_profiles.paths import OpenCodePaths


@pytest.fixture
def tmp_opencode(tmp_path):
    """创建临时 opencode 配置目录结构。"""
    base = tmp_path / ".config" / "opencode"
    base.mkdir(parents=True)
    return base


@pytest.fixture
def paths(tmp_opencode):
    """返回基于临时目录的 OpenCodePaths 实例。"""
    return OpenCodePaths(base_dir=tmp_opencode)


@pytest.fixture
def sample_config():
    """返回示例配置内容。"""
    return {
        "$schema": "https://opencode.ai/config.json",
        "provider": {
            "test": {
                "name": "Test",
                "npm": "@ai-sdk/openai-compatible",
                "options": {"apiKey": "test-key", "baseURL": "https://test.example.com/v1"}
            }
        },
        "shell": "bash"
    }


@pytest.fixture
def existing_config(tmp_opencode, sample_config):
    """在临时目录中创建实际的 opencode.json 配置文件。"""
    config_file = tmp_opencode / "opencode.json"
    config_file.write_text(json.dumps(sample_config, indent=2))
    return config_file
```

- [ ] **步骤 2：Commit**

```bash
git add tests/conftest.py
git commit -m "test: add shared fixtures for opencode paths and config"
```

---

## 任务 4：核心操作模块

**文件：**
- 创建：`opencode_profiles/ops.py`
- 创建：`tests/test_ops.py`

- [ ] **步骤 1：编写失败测试**

`tests/test_ops.py`:

```python
import json
import pytest
from opencode_profiles.ops import (
    backup,
    create_empty,
    create_from_current,
    ensure_initialized,
    get_active,
    list_profiles,
    switch,
)


# --- init ---

def test_init_creates_profiles_dir(paths, existing_config):
    ensure_initialized(paths)
    assert paths.profiles_dir.exists()
    assert paths.profile_config("default").exists()


def test_init_creates_symlink(paths, existing_config):
    ensure_initialized(paths)
    config = paths.config_file
    assert config.is_symlink()
    assert config.resolve() == paths.profile_config("default").resolve()


def test_init_creates_backup(paths, existing_config, sample_config):
    ensure_initialized(paths)
    backup_file = paths.base_dir / "opencode.json.bak"
    assert backup_file.exists()
    assert json.loads(backup_file.read_text()) == sample_config


def test_init_creates_reserved_dirs(paths, existing_config):
    ensure_initialized(paths)
    assert paths.profile_skills("default").is_dir()


def test_init_idempotent(paths, existing_config):
    ensure_initialized(paths)
    ensure_initialized(paths)
    assert paths.config_file.is_symlink()


def test_init_creates_default_if_no_existing_config(paths):
    ensure_initialized(paths)
    default_config = paths.profile_config("default")
    assert default_config.exists()
    assert json.loads(default_config.read_text()) == {}


# --- backup ---

def test_backup_creates_backup_dir(paths, existing_config):
    ensure_initialized(paths)
    name = backup(paths)
    assert name.startswith("backup_")
    assert name in list_profiles(paths)


def test_backup_preserves_config_content(paths, existing_config, sample_config):
    ensure_initialized(paths)
    name = backup(paths)
    backup_content = json.loads(paths.profile_config(name).read_text())
    assert backup_content == sample_config


def test_backup_creates_skills_dir(paths, existing_config):
    ensure_initialized(paths)
    name = backup(paths)
    assert paths.profile_skills(name).is_dir()


# --- create ---

def test_create_from_current(paths, existing_config, sample_config):
    ensure_initialized(paths)
    create_from_current(paths, "work")
    assert "work" in list_profiles(paths)
    content = json.loads(paths.profile_config("work").read_text())
    assert content == sample_config


def test_create_from_current_raises_if_exists(paths, existing_config):
    ensure_initialized(paths)
    create_from_current(paths, "work")
    with pytest.raises(FileExistsError):
        create_from_current(paths, "work")


def test_create_empty(paths, existing_config):
    ensure_initialized(paths)
    create_empty(paths, "empty")
    assert "empty" in list_profiles(paths)
    content = json.loads(paths.profile_config("empty").read_text())
    assert content == {}


def test_create_empty_raises_if_exists(paths, existing_config):
    ensure_initialized(paths)
    create_empty(paths, "empty")
    with pytest.raises(FileExistsError):
        create_empty(paths, "empty")


def test_create_creates_skills_dir(paths, existing_config):
    ensure_initialized(paths)
    create_from_current(paths, "work")
    assert paths.profile_skills("work").is_dir()


# --- switch ---

def test_switch_updates_symlink(paths, existing_config):
    ensure_initialized(paths)
    create_empty(paths, "work")
    switch(paths, "work")
    assert paths.config_file.is_symlink()
    assert get_active(paths) == "work"


def test_switch_raises_if_not_found(paths, existing_config):
    ensure_initialized(paths)
    with pytest.raises(FileNotFoundError):
        switch(paths, "nonexistent")


# --- list ---

def test_list_profiles(paths, existing_config):
    ensure_initialized(paths)
    create_empty(paths, "work")
    create_from_current(paths, "personal")
    profiles = list_profiles(paths)
    assert "default" in profiles
    assert "work" in profiles
    assert "personal" in profiles


def test_list_profiles_empty(paths):
    assert list_profiles(paths) == []


# --- active ---

def test_get_active(paths, existing_config):
    ensure_initialized(paths)
    assert get_active(paths) == "default"
    create_empty(paths, "work")
    switch(paths, "work")
    assert get_active(paths) == "work"


def test_get_active_returns_none_if_not_symlink(paths):
    assert get_active(paths) is None
```

- [ ] **步骤 2：运行测试验证失败**

```bash
uv run pytest tests/test_ops.py -v
```

预期：FAIL，报 `ModuleNotFoundError`

- [ ] **步骤 3：编写实现**

`opencode_profiles/ops.py`:

```python
import json
import os
import shutil
from datetime import datetime
from pathlib import Path

from opencode_profiles.paths import OpenCodePaths


def ensure_initialized(paths: OpenCodePaths) -> None:
    """确保 opencode 配置目录已初始化。

    如果 opencode.json 不是 symlink，将其内容存入 default profile 并替换为 symlink。
    如果已经是 symlink，不做任何操作。
    """
    paths.profiles_dir.mkdir(parents=True, exist_ok=True)

    config = paths.config_file

    if config.is_symlink():
        return

    default_dir = paths.profile_dir("default")
    default_dir.mkdir(parents=True, exist_ok=True)
    default_config = paths.profile_config("default")
    skills_dir = paths.profile_skills("default")
    skills_dir.mkdir(exist_ok=True)

    if config.exists():
        backup_path = paths.base_dir / "opencode.json.bak"
        if not backup_path.exists():
            shutil.copy2(config, backup_path)
        shutil.copy2(config, default_config)
        config.unlink()
    else:
        default_config.write_text("{}")

    rel_target = Path("profiles") / "default" / "opencode.json"
    os.symlink(rel_target, config)


def backup(paths: OpenCodePaths) -> str:
    """备份当前激活的 profile 配置。返回备份目录名称。"""
    ensure_initialized(paths)

    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    backup_name = f"backup_{timestamp}"
    backup_dir = paths.profile_dir(backup_name)
    backup_dir.mkdir(parents=True, exist_ok=True)

    current_config = paths.config_file
    if current_config.is_symlink():
        target = current_config.resolve()
        shutil.copy2(target, paths.profile_config(backup_name))
    else:
        paths.profile_config(backup_name).write_text("{}")

    paths.profile_skills(backup_name).mkdir(exist_ok=True)
    return backup_name


def create_from_current(paths: OpenCodePaths, name: str) -> None:
    """从当前激活的 profile 创建新 profile。"""
    ensure_initialized(paths)

    profile_dir = paths.profile_dir(name)
    if profile_dir.exists():
        raise FileExistsError(f"Profile '{name}' already exists")

    profile_dir.mkdir(parents=True)
    paths.profile_skills(name).mkdir(exist_ok=True)

    current_config = paths.config_file
    if current_config.is_symlink():
        target = current_config.resolve()
        shutil.copy2(target, paths.profile_config(name))
    else:
        paths.profile_config(name).write_text(
            json.dumps(json.loads(current_config.read_text()), indent=2)
        )


def create_empty(paths: OpenCodePaths, name: str) -> None:
    """创建空 profile（最小合法 JSON）。"""
    ensure_initialized(paths)

    profile_dir = paths.profile_dir(name)
    if profile_dir.exists():
        raise FileExistsError(f"Profile '{name}' already exists")

    profile_dir.mkdir(parents=True)
    paths.profile_config(name).write_text("{}")
    paths.profile_skills(name).mkdir(exist_ok=True)


def switch(paths: OpenCodePaths, name: str) -> None:
    """切换 symlink 指向目标 profile。"""
    ensure_initialized(paths)

    target = paths.profile_config(name)
    if not target.exists():
        available = list_profiles(paths)
        raise FileNotFoundError(
            f"Profile '{name}' not found. Available: {available}"
        )

    config = paths.config_file
    config.parent.mkdir(parents=True, exist_ok=True)

    if config.is_symlink() or config.exists():
        config.unlink()

    rel_target = Path("profiles") / name / "opencode.json"
    os.symlink(rel_target, config)


def list_profiles(paths: OpenCodePaths) -> list[str]:
    """列出所有 profile 名称。"""
    if not paths.profiles_dir.exists():
        return []

    profiles = []
    for d in sorted(paths.profiles_dir.iterdir()):
        if d.is_dir() and (d / "opencode.json").exists():
            profiles.append(d.name)
    return profiles


def get_active(paths: OpenCodePaths) -> str | None:
    """获取当前激活的 profile 名称。"""
    config = paths.config_file
    if not config.is_symlink():
        return None

    target = config.resolve()
    parts = target.relative_to(paths.base_dir).parts
    if len(parts) >= 2 and parts[0] == "profiles":
        return parts[1]
    return None
```

- [ ] **步骤 4：运行测试验证通过**

```bash
uv run pytest tests/test_ops.py -v
```

预期：全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add opencode_profiles/ops.py tests/test_ops.py
git commit -m "feat: add core operations (init, backup, create, switch, list)"
```

---

## 任务 5：CLI 入口

**文件：**
- 创建：`opencode_profiles/cli.py`
- 创建：`tests/test_cli.py`

- [ ] **步骤 1：编写失败测试**

`tests/test_cli.py`:

```python
import pytest
from click.testing import CliRunner
from opencode_profiles.cli import main
from opencode_profiles.paths import OpenCodePaths


@pytest.fixture
def runner():
    return CliRunner()


@pytest.fixture
def cli_paths(tmp_path):
    base = tmp_path / ".config" / "opencode"
    base.mkdir(parents=True)
    config = base / "opencode.json"
    config.write_text('{"shell": "bash"}')
    return OpenCodePaths(base_dir=base)


def test_list_command(runner, cli_paths, monkeypatch):
    monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)
    from opencode_profiles.ops import ensure_initialized
    ensure_initialized(cli_paths)
    result = runner.invoke(main, ["-l"])
    assert result.exit_code == 0
    assert "default" in result.output


def test_create_command(runner, cli_paths, monkeypatch):
    monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)
    result = runner.invoke(main, ["-c", "work"])
    assert result.exit_code == 0
    assert "work" in result.output


def test_create_empty_command(runner, cli_paths, monkeypatch):
    monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)
    result = runner.invoke(main, ["-e", "minimal"])
    assert result.exit_code == 0
    assert "minimal" in result.output


def test_switch_command(runner, cli_paths, monkeypatch):
    monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)
    runner.invoke(main, ["-c", "work"])
    result = runner.invoke(main, ["-s", "work"])
    assert result.exit_code == 0
    assert "work" in result.output


def test_backup_command(runner, cli_paths, monkeypatch):
    monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)
    result = runner.invoke(main, ["-b"])
    assert result.exit_code == 0
    assert "backup_" in result.output
```

- [ ] **步骤 2：运行测试验证失败**

```bash
uv run pytest tests/test_cli.py -v
```

预期：FAIL，报 `ModuleNotFoundError`

- [ ] **步骤 3：编写实现**

`opencode_profiles/cli.py`:

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
@click.option("-b", "--backup", is_flag=True, help="备份当前配置")
@click.option("-c", "--create", type=str, help="从当前配置创建新 profile")
@click.option("-e", "--empty", type=str, help="创建空 profile")
@click.option("-s", "--switch", type=str, help="切换到指定 profile")
@click.option("-l", "--list", "list_flag", is_flag=True, help="列出所有 profile")
def main(backup, create, empty, switch, list_flag):
    """opencode-profiles — 多配置管理工具。"""
    if backup:
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
            create_empty(paths, empty)
            click.echo(f"Created empty profile '{empty}'")
        except FileExistsError as e:
            raise click.ClickException(str(e))
    elif switch:
        try:
            switch(paths, switch)
            click.echo(f"Switched to '{switch}'")
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

```bash
uv run pytest tests/test_cli.py -v
```

预期：全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add opencode_profiles/cli.py tests/test_cli.py
git commit -m "feat: add CLI entry point with click"
```

---

## 任务 6：端到端验证

**文件：** 无

- [ ] **步骤 1：安装 CLI 并验证帮助信息**

```bash
uv run opencode-profiles --help
```

预期：显示 `-b`、`-c`、`-e`、`-s`、`-l` 选项

- [ ] **步骤 2：验证完整工作流**

```bash
# 初始化（将当前配置转为 default profile）
uv run opencode-profiles -l

# 从当前配置创建 work profile
uv run opencode-profiles -c work

# 创建空 profile
uv run opencode-profiles -e minimal

# 列出所有 profile
uv run opencode-profiles -l

# 切换到 work
uv run opencode-profiles -s work

# 备份
uv run opencode-profiles -b
```

预期：所有命令正常执行，无报错

- [ ] **步骤 3：运行全部测试**

```bash
uv run pytest -v
```

预期：全部 PASS
