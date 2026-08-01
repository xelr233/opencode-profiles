# Skills Management 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为 opencode-profiles 添加 skills 管理能力：每个 profile 通过 skills.yml 记录使用的 skills，切换 profile 时自动同步软链接并更新 cc-switch.db。

**架构：** 新增 `skills.py` 模块处理所有 skill 读写/同步逻辑；`paths.py` 添加 skill_sources_dir 配置；`ops.py` 在 switch/create 流程中调用 skill 同步；`cli.py` 添加 --add-skill/--remove-skill 命令。

**技术栈：** Python 3.11+, pyyaml, sqlite3 (stdlib), click, pytest

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `pyproject.toml` | 修改 | 添加 pyyaml 依赖 |
| `opencode_profiles/paths.py` | 修改 | 添加 skill_sources_dir, profile_skills_yml(), skill_source() |
| `opencode_profiles/skills.py` | 创建 | 所有 skill 操作：read/write yml, scan, diff, sync, add/remove, db update |
| `opencode_profiles/ops.py` | 修改 | ensure_initialized/create_* 写入 skills.yml；switch() 调用 sync_skills() |
| `opencode_profiles/cli.py` | 修改 | 添加 --add-skill, --remove-skill, --profile 选项 |
| `tests/test_skills.py` | 创建 | 所有 skill 相关测试 |
| `tests/conftest.py` | 修改 | 添加 skill_sources_dir fixture |

---

## 任务 1：添加 pyyaml 依赖

**文件：**
- 修改：`pyproject.toml`

- [ ] **步骤 1：添加 pyyaml 到 dependencies**

将 `pyproject.toml` 的 dependencies 行改为：

```toml
dependencies = ["click>=8.0", "pyyaml>=6.0"]
```

- [ ] **步骤 2：同步 lockfile**

```bash
uv lock
```

- [ ] **步骤 3：Commit**

```bash
git add pyproject.toml uv.lock
git commit -m "build: add pyyaml dependency for skills.yml parsing"
```

---

## 任务 2：扩展 paths.py

**文件：**
- 修改：`opencode_profiles/paths.py`
- 测试：`tests/test_paths.py`

- [ ] **步骤 1：编写失败的测试**

在 `tests/test_paths.py` 末尾添加：

```python
class TestSkillPaths:
    def test_skill_sources_dir_default(self, paths):
        assert paths.skill_sources_dir == Path.home() / ".cc-switch" / "skills"

    def test_skill_sources_dir_custom(self, tmp_path):
        custom = tmp_path / "my-skills"
        p = OpenCodePaths(base_dir=tmp_path / "opencode", skill_sources_dir=custom)
        assert p.skill_sources_dir == custom

    def test_profile_skills_yml(self, paths):
        assert paths.profile_skills_yml("work") == paths.base_dir / "profiles" / "work" / "skills.yml"

    def test_skill_source(self, paths):
        assert paths.skill_source("rtk") == Path.home() / ".cc-switch" / "skills" / "rtk"
```

- [ ] **步骤 2：运行测试验证失败**

```bash
uv run pytest tests/test_paths.py::TestSkillPaths -v
```
预期：FAIL，报错 "OpenCodePaths has no attribute skill_sources_dir"

- [ ] **步骤 3：修改 paths.py**

```python
class OpenCodePaths:
    """管理 opencode 配置路径常量。"""

    def __init__(self, base_dir: Path | None = None, skill_sources_dir: Path | None = None):
        self._base_dir = base_dir or Path.home() / ".config" / "opencode"
        self._skill_sources_dir = skill_sources_dir or Path.home() / ".cc-switch" / "skills"

    @property
    def skill_sources_dir(self) -> Path:
        return self._skill_sources_dir

    def profile_skills_yml(self, name: str) -> Path:
        return self.profile_dir(name) / "skills.yml"

    def skill_source(self, name: str) -> Path:
        return self._skill_sources_dir / name
```

- [ ] **步骤 4：运行测试验证通过**

```bash
uv run pytest tests/test_paths.py::TestSkillPaths -v
```
预期：4 passed

- [ ] **步骤 5：Commit**

```bash
git add opencode_profiles/paths.py tests/test_paths.py
git commit -m "feat(paths): add skill_sources_dir, profile_skills_yml, skill_source"
```

---

## 任务 3：创建 skills.py — read/write/scan/diff 函数

**文件：**
- 创建：`opencode_profiles/skills.py`
- 测试：`tests/test_skills.py`（创建新文件）
- 修改：`tests/conftest.py`

- [ ] **步骤 1：添加 conftest.py fixture**

在 `tests/conftest.py` 末尾添加：

```python
@pytest.fixture(autouse=True)
def _skip_db_update(tmp_path, monkeypatch):
    """Prevent tests from modifying the real cc-switch.db.

    Safe to run before skills.py exists (skips if module not importable).
    """
    import importlib
    try:
        mod = importlib.import_module("opencode_profiles.skills")
    except ImportError:
        return
    monkeypatch.setattr(mod, "DB_PATH", tmp_path / "nonexistent.db")


@pytest.fixture
def skill_sources(tmp_path):
    """创建临时 skill 源目录，包含几个测试 skill。"""
    src = tmp_path / "skill-sources"
    src.mkdir()
    for name in ["brainstorming", "rtk", "mavenbuild"]:
        skill_dir = src / name
        skill_dir.mkdir()
        (skill_dir / "SKILL.md").write_text(f"# {name}\n")
    return src


@pytest.fixture
def paths_with_sources(tmp_opencode, skill_sources):
    """返回带有 skill_sources_dir 的 OpenCodePaths 实例。"""
    return OpenCodePaths(base_dir=tmp_opencode, skill_sources_dir=skill_sources)
```

- [ ] **步骤 2：编写失败的测试**

创建 `tests/test_skills.py`：

```python
import pytest
from opencode_profiles.skills import (
    add_skill,
    compute_diff,
    read_skills_yml,
    remove_skill,
    scan_current_skills,
    write_skills_yml,
)


class TestReadWriteSkillsYml:
    def test_read_missing_returns_empty(self, paths_with_sources):
        assert read_skills_yml(paths_with_sources, "default") == []

    def test_read_existing(self, paths_with_sources):
        write_skills_yml(paths_with_sources, "work", ["brainstorming", "rtk"])
        assert read_skills_yml(paths_with_sources, "work") == ["brainstorming", "rtk"]

    def test_write_creates_file(self, paths_with_sources):
        write_skills_yml(paths_with_sources, "test", ["mavenbuild"])
        yml = paths_with_sources.profile_skills_yml("test")
        assert yml.exists()

    def test_write_overwrites(self, paths_with_sources):
        write_skills_yml(paths_with_sources, "test", ["a", "b"])
        write_skills_yml(paths_with_sources, "test", ["c"])
        assert read_skills_yml(paths_with_sources, "test") == ["c"]


class TestScanCurrentSkills:
    def test_scan_no_dir(self, paths_with_sources):
        assert scan_current_skills(paths_with_sources) == []

    def test_scan_symlinks(self, paths_with_sources):
        skills_dir = paths_with_sources.base_dir / "skills"
        skills_dir.mkdir()
        import os
        os.symlink(paths_with_sources.skill_source("rtk"), skills_dir / "rtk")
        os.symlink(paths_with_sources.skill_source("mavenbuild"), skills_dir / "mavenbuild")
        result = scan_current_skills(paths_with_sources)
        assert result == ["mavenbuild", "rtk"]

    def test_scan_ignores_non_symlinks(self, paths_with_sources):
        skills_dir = paths_with_sources.base_dir / "skills"
        skills_dir.mkdir()
        (skills_dir / "real-dir").mkdir()
        (skills_dir / "file.txt").write_text("not a skill")
        assert scan_current_skills(paths_with_sources) == []


class TestComputeDiff:
    def test_add_only(self):
        to_add, to_remove = compute_diff(["a"], ["a", "b", "c"])
        assert to_add == ["b", "c"]
        assert to_remove == []

    def test_remove_only(self):
        to_add, to_remove = compute_diff(["a", "b", "c"], ["a"])
        assert to_add == []
        assert to_remove == ["b", "c"]

    def test_mixed(self):
        to_add, to_remove = compute_diff(["a", "b"], ["b", "c"])
        assert to_add == ["c"]
        assert to_remove == ["a"]

    def test_identical(self):
        to_add, to_remove = compute_diff(["a", "b"], ["a", "b"])
        assert to_add == []
        assert to_remove == []


class TestAddSkill:
    def test_add_new(self, paths_with_sources):
        add_skill(paths_with_sources, "work", "rtk")
        assert read_skills_yml(paths_with_sources, "work") == ["rtk"]

    def test_add_no_duplicate(self, paths_with_sources):
        add_skill(paths_with_sources, "work", "rtk")
        add_skill(paths_with_sources, "work", "rtk")
        assert read_skills_yml(paths_with_sources, "work") == ["rtk"]

    def test_add_missing_source_raises(self, paths_with_sources):
        with pytest.raises(FileNotFoundError, match="not found"):
            add_skill(paths_with_sources, "work", "nonexistent")


class TestRemoveSkill:
    def test_remove_existing(self, paths_with_sources):
        write_skills_yml(paths_with_sources, "work", ["rtk", "mavenbuild"])
        remove_skill(paths_with_sources, "work", "rtk")
        assert read_skills_yml(paths_with_sources, "work") == ["mavenbuild"]

    def test_remove_not_present(self, paths_with_sources):
        write_skills_yml(paths_with_sources, "work", ["rtk"])
        remove_skill(paths_with_sources, "work", "mavenbuild")
        assert read_skills_yml(paths_with_sources, "work") == ["rtk"]
```

- [ ] **步骤 3：运行测试验证失败**

```bash
uv run pytest tests/test_skills.py -v
```
预期：FAIL，报错 "module not found"

- [ ] **步骤 4：编写 skills.py 基础实现**

创建 `opencode_profiles/skills.py`：

```python
"""Profile skills management: read/write skills.yml, scan, diff, add/remove."""

from __future__ import annotations

from pathlib import Path

import yaml

from opencode_profiles.paths import OpenCodePaths


def read_skills_yml(paths: OpenCodePaths, name: str) -> list[str]:
    """Read skills list from a profile's skills.yml. Empty list if missing."""
    yml_path = paths.profile_skills_yml(name)
    if not yml_path.exists():
        return []
    data = yaml.safe_load(yml_path.read_text())
    return data if isinstance(data, list) else []


def write_skills_yml(paths: OpenCodePaths, name: str, skills: list[str]) -> None:
    """Write skills list to a profile's skills.yml."""
    yml_path = paths.profile_skills_yml(name)
    yml_path.parent.mkdir(parents=True, exist_ok=True)
    yml_path.write_text(yaml.safe_dump(skills, default_flow_style=False))


def scan_current_skills(paths: OpenCodePaths) -> list[str]:
    """Scan ~/.config/opencode/skills/ for symlinks pointing to skill_sources_dir."""
    skills_dir = paths.base_dir / "skills"
    if not skills_dir.exists():
        return []
    result = []
    for entry in sorted(skills_dir.iterdir()):
        if entry.is_symlink():
            resolved = entry.resolve()
            if resolved.parent == paths.skill_sources_dir:
                result.append(entry.name)
    return result


def compute_diff(current: list[str], target: list[str]) -> tuple[list[str], list[str]]:
    """Return (to_add, to_remove) based on set difference."""
    cur, tgt = set(current), set(target)
    return sorted(tgt - cur), sorted(cur - tgt)


def add_skill(paths: OpenCodePaths, name: str, skill: str) -> None:
    """Add a skill to a profile's skills.yml. Validates source exists."""
    source = paths.skill_source(skill)
    if not source.exists():
        raise FileNotFoundError(f"Skill source '{skill}' not found at {source}")
    skills = read_skills_yml(paths, name)
    if skill not in skills:
        skills.append(skill)
    write_skills_yml(paths, name, skills)


def remove_skill(paths: OpenCodePaths, name: str, skill: str) -> None:
    """Remove a skill from a profile's skills.yml."""
    skills = read_skills_yml(paths, name)
    if skill in skills:
        skills.remove(skill)
    write_skills_yml(paths, name, skills)
```

- [ ] **步骤 5：运行测试验证通过**

```bash
uv run pytest tests/test_skills.py::TestReadWriteSkillsYml tests/test_skills.py::TestScanCurrentSkills tests/test_skills.py::TestComputeDiff tests/test_skills.py::TestAddSkill tests/test_skills.py::TestRemoveSkill -v
```
预期：13 passed

- [ ] **步骤 6：Commit**

```bash
git add opencode_profiles/skills.py tests/test_skills.py tests/conftest.py
git commit -m "feat(skills): add read/write yml, scan, diff, add/remove functions"
```

---

## 任务 4：添加 sync_skills 和 db update

**文件：**
- 修改：`opencode_profiles/skills.py`
- 修改：`tests/test_skills.py`

- [ ] **步骤 1：编写失败的测试**

在 `tests/test_skills.py` 末尾添加：

```python
import os
import sqlite3

from opencode_profiles.skills import sync_skills


class TestSyncSkills:
    def test_sync_adds_symlinks(self, paths_with_sources):
        write_skills_yml(paths_with_sources, "work", ["rtk", "mavenbuild"])
        sync_skills(paths_with_sources, "work", db_path=Path("/dev/null"))
        skills_dir = paths_with_sources.base_dir / "skills"
        assert (skills_dir / "rtk").is_symlink()
        assert (skills_dir / "mavenbuild").is_symlink()

    def test_sync_removes_symlinks(self, paths_with_sources):
        skills_dir = paths_with_sources.base_dir / "skills"
        skills_dir.mkdir()
        os.symlink(paths_with_sources.skill_source("rtk"), skills_dir / "rtk")
        os.symlink(paths_with_sources.skill_source("mavenbuild"), skills_dir / "mavenbuild")
        write_skills_yml(paths_with_sources, "default", ["rtk", "mavenbuild"])
        write_skills_yml(paths_with_sources, "work", ["rtk"])
        sync_skills(paths_with_sources, "work", db_path=Path("/dev/null"))
        assert (skills_dir / "rtk").is_symlink()
        assert not (skills_dir / "mavenbuild").exists()

    def test_sync_missing_source_fails_no_partial(self, paths_with_sources):
        write_skills_yml(paths_with_sources, "work", ["rtk", "nonexistent"])
        with pytest.raises(FileNotFoundError, match="nonexistent"):
            sync_skills(paths_with_sources, "work", db_path=Path("/dev/null"))
        skills_dir = paths_with_sources.base_dir / "skills"
        assert not (skills_dir / "rtk").exists()

    def test_sync_removes_real_dir_not_in_target(self, paths_with_sources):
        skills_dir = paths_with_sources.base_dir / "skills"
        skills_dir.mkdir()
        (skills_dir / "old-skill").mkdir()
        (skills_dir / "old-skill" / "SKILL.md").write_text("old")
        write_skills_yml(paths_with_sources, "default", [])
        write_skills_yml(paths_with_sources, "work", ["rtk"])
        sync_skills(paths_with_sources, "work", db_path=Path("/dev/null"))
        assert not (skills_dir / "old-skill").exists()
        assert (skills_dir / "rtk").is_symlink()


class TestUpdateDb:
    def test_update_sets_enabled_flag(self, tmp_path, monkeypatch):
        db_path = tmp_path / "test.db"
        conn = sqlite3.connect(db_path)
        conn.execute("""CREATE TABLE skills (
            id TEXT PRIMARY KEY, name TEXT NOT NULL, directory TEXT NOT NULL,
            enabled_opencode BOOLEAN NOT NULL DEFAULT 0)""")
        conn.execute("INSERT INTO skills (id, name, directory) VALUES ('local:a', 'a', 'a')")
        conn.execute("INSERT INTO skills (id, name, directory) VALUES ('local:b', 'b', 'b')")
        conn.execute("INSERT INTO skills (id, name, directory) VALUES ('local:c', 'c', 'c')")
        conn.commit()
        conn.close()

        import opencode_profiles.skills as skills_mod
        monkeypatch.setattr(skills_mod, "DB_PATH", db_path)
        skills_mod._update_db(["a", "c"])

        conn = sqlite3.connect(db_path)
        rows = conn.execute("SELECT name, enabled_opencode FROM skills ORDER BY name").fetchall()
        conn.close()
        assert rows == [("a", 1), ("b", 0), ("c", 1)]

    def test_update_missing_db_skips(self, tmp_path, monkeypatch):
        import opencode_profiles.skills as skills_mod
        monkeypatch.setattr(skills_mod, "DB_PATH", tmp_path / "nonexistent.db")
        skills_mod._update_db(["a"])
        # Should not raise
```

- [ ] **步骤 2：运行测试验证失败**

```bash
uv run pytest tests/test_skills.py::TestSyncSkills tests/test_skills.py::TestUpdateDb -v
```
预期：FAIL，报错 "sync_skills not defined"

- [ ] **步骤 3：添加 sync_skills 和 _update_db 到 skills.py**

在 `opencode_profiles/skills.py` 顶部添加 import 和常量：

```python
import os
import shutil
import sqlite3
from pathlib import Path
```

在 `from opencode_profiles.paths import OpenCodePaths` 之后添加：

```python
DB_PATH = Path.home() / ".cc-switch" / "cc-switch.db"
```

在文件末尾添加：

```python
def sync_skills(paths: OpenCodePaths, target_name: str, db_path: Path | None = None) -> None:
    """Sync opencode/skills/ symlinks to match target profile's skills.yml.

    Validates all target sources exist before modifying anything.
    Updates cc-switch.db after successful sync.
    """
    from opencode_profiles.ops import get_active

    target_skills = read_skills_yml(paths, target_name)

    active = get_active(paths)
    current_skills = read_skills_yml(paths, active) if active else []

    # Validate all target sources exist BEFORE modifying anything
    for skill in target_skills:
        source = paths.skill_source(skill)
        if not source.exists():
            raise FileNotFoundError(f"Skill source '{skill}' not found at {source}")

    to_add, to_remove = compute_diff(current_skills, target_skills)

    skills_dir = paths.base_dir / "skills"
    skills_dir.mkdir(exist_ok=True)

    # Remove
    for skill in to_remove:
        link = skills_dir / skill
        if link.is_symlink():
            link.unlink()
        elif link.exists():
            if link.is_dir():
                shutil.rmtree(link)
            else:
                link.unlink()

    # Add
    for skill in to_add:
        link = skills_dir / skill
        if link.is_symlink():
            link.unlink()
        elif link.exists():
            if link.is_dir():
                shutil.rmtree(link)
            else:
                link.unlink()
        os.symlink(paths.skill_source(skill), link)

    # Update db
    _update_db(target_skills, db_path=db_path)


def _update_db(active_skills: list[str], db_path: Path | None = None) -> None:
    """Update enabled_opencode in cc-switch.db based on active skills."""
    if db_path is None:
        db_path = DB_PATH
    if not db_path.exists():
        return
    conn = sqlite3.connect(db_path)
    try:
        conn.execute("UPDATE skills SET enabled_opencode = 0")
        for skill in active_skills:
            conn.execute(
                "UPDATE skills SET enabled_opencode = 1 WHERE name = ?",
                (skill,),
            )
        conn.commit()
    finally:
        conn.close()
```

- [ ] **步骤 4：运行测试验证通过**

```bash
uv run pytest tests/test_skills.py -v
```
预期：17+ passed

- [ ] **步骤 5：Commit**

```bash
git add opencode_profiles/skills.py tests/test_skills.py
git commit -m "feat(skills): add sync_skills with atomic validation and db update"
```

---

## 任务 5：修改 ops.py — 集成 skills 管理

**文件：**
- 修改：`opencode_profiles/ops.py`
- 修改：`tests/test_ops.py`

- [ ] **步骤 1：编写失败的测试**

在 `tests/test_ops.py` 末尾添加：

```python
from opencode_profiles.skills import read_skills_yml, scan_current_skills


class TestSkillsIntegration:
    def test_ensure_initialized_creates_skills_yml(self, paths, existing_config, skill_sources, monkeypatch):
        """ensure_initialized 后 default profile 有 skills.yml。"""
        from opencode_profiles.ops import ensure_initialized
        monkeypatch.setattr(paths, "_skill_sources_dir", skill_sources)
        # Prevent db modification
        monkeypatch.setattr("opencode_profiles.skills.DB_PATH", paths.base_dir / "nonexistent.db")
        ensure_initialized(paths)
        yml = paths.profile_skills_yml("default")
        assert yml.exists()

    def test_create_from_current_copies_skills(self, paths, existing_config, skill_sources, monkeypatch):
        """create_from_current 扫描当前 skills 并写入新 profile。"""
        from opencode_profiles.ops import create_from_current, ensure_initialized
        monkeypatch.setattr(paths, "_skill_sources_dir", skill_sources)
        monkeypatch.setattr("opencode_profiles.skills.DB_PATH", paths.base_dir / "nonexistent.db")
        # 创建一些 symlinks
        skills_dir = paths.base_dir / "skills"
        skills_dir.mkdir()
        import os
        os.symlink(skill_sources / "rtk", skills_dir / "rtk")
        ensure_initialized(paths)
        create_from_current(paths, "work")
        assert read_skills_yml(paths, "work") == ["rtk"]

    def test_switch_syncs_skills(self, paths, existing_config, skill_sources, monkeypatch):
        """switch 后 opencode/skills/ 的 symlinks 匹配目标 profile。"""
        from opencode_profiles.ops import create_empty, ensure_initialized, switch
        from opencode_profiles.skills import write_skills_yml
        monkeypatch.setattr(paths, "_skill_sources_dir", skill_sources)
        monkeypatch.setattr("opencode_profiles.skills.DB_PATH", paths.base_dir / "nonexistent.db")
        skills_dir = paths.base_dir / "skills"
        skills_dir.mkdir()
        import os
        os.symlink(skill_sources / "rtk", skills_dir / "rtk")
        ensure_initialized(paths)  # default scans rtk
        # work 只保留 mavenbuild，移除 rtk
        write_skills_yml(paths, "default", ["rtk"])
        create_empty(paths, "work")  # work inherits rtk from scan
        from opencode_profiles.skills import add_skill, remove_skill
        add_skill(paths, "work", "mavenbuild")
        remove_skill(paths, "work", "rtk")
        switch(paths, "work")
        assert (skills_dir / "mavenbuild").is_symlink()
        assert not (skills_dir / "rtk").exists()
```

- [ ] **步骤 2：运行测试验证失败**

```bash
uv run pytest tests/test_ops.py::TestSkillsIntegration -v
```
预期：FAIL

- [ ] **步骤 3：修改 ops.py**

在 `ops.py` 顶部添加 import：

```python
from opencode_profiles.skills import scan_current_skills, write_skills_yml
```

修改 `ensure_initialized` 函数，在创建 default profile 的 skills 目录后添加 skills.yml 写入：

```python
def ensure_initialized(paths: OpenCodePaths) -> None:
    """确保 opencode 配置目录已初始化。"""
    paths.profiles_dir.mkdir(parents=True, exist_ok=True)

    config = paths.config_file

    if config.is_symlink():
        return

    default_dir = paths.profile_dir("default")
    default_dir.mkdir(parents=True, exist_ok=True)
    default_config = paths.profile_config("default")
    default_tui_config = paths.profile_tui_config("default")
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

    os.symlink(paths.relative_target("default"), config)

    tui_config = paths.tui_config_file
    if tui_config.exists() and not tui_config.is_symlink():
        shutil.copy2(tui_config, default_tui_config)
        tui_config.unlink()
        os.symlink(paths.relative_target_tui("default"), tui_config)

    # Write skills.yml for default profile
    if not paths.profile_skills_yml("default").exists():
        current = scan_current_skills(paths)
        write_skills_yml(paths, "default", current)
```

修改 `create_from_current` 函数，在创建后添加 skills.yml 写入：

```python
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
        raise RuntimeError("config_file is not a symlink after init")

    active = get_active(paths)
    if active is not None:
        current_tui = paths.profile_tui_config(active)
        if current_tui.exists():
            shutil.copy2(current_tui, paths.profile_tui_config(name))

    # Write skills.yml for new profile
    current = scan_current_skills(paths)
    write_skills_yml(paths, name, current)
```

修改 `create_empty` 函数，在创建后添加 skills.yml 写入：

```python
def create_empty(paths: OpenCodePaths, name: str, source: str | None = None) -> None:
    """创建空 profile，可选从源配置导入 provider。"""
    ensure_initialized(paths)

    profile_dir = paths.profile_dir(name)
    if profile_dir.exists():
        raise FileExistsError(f"Profile '{name}' already exists")

    providers = _load_providers(paths, source) if source is not None else None

    profile_dir.mkdir(parents=True)
    paths.profile_skills(name).mkdir(exist_ok=True)

    if providers is None:
        paths.profile_config(name).write_text("{}")
    else:
        paths.profile_config(name).write_text(json.dumps({"provider": providers}, indent=2))

    if source is not None:
        if source == "current":
            active = get_active(paths)
            if active is None:
                # Still write skills.yml before returning
                current = scan_current_skills(paths)
                write_skills_yml(paths, name, current)
                return
            src_tui = paths.profile_tui_config(active)
        else:
            src_tui = paths.profile_tui_config(source)
        if src_tui.exists():
            shutil.copy2(src_tui, paths.profile_tui_config(name))

    # Write skills.yml for new profile
    current = scan_current_skills(paths)
    write_skills_yml(paths, name, current)
```

修改 `switch` 函数，在 symlink 交换后调用 sync_skills：

```python
def switch(paths: OpenCodePaths, name: str) -> None:
    """切换 symlink 指向目标 profile。"""
    ensure_initialized(paths)

    target = paths.profile_config(name)
    if not target.exists():
        available = list_profiles(paths)
        raise FileNotFoundError(f"Profile '{name}' not found. Available: {available}")

    config = paths.config_file
    config.parent.mkdir(parents=True, exist_ok=True)

    if config.is_symlink() or config.exists():
        config.unlink()

    os.symlink(paths.relative_target(name), config)

    # 管理 tui.json symlink
    tui_config = paths.tui_config_file
    tui_target = paths.profile_tui_config(name)

    if tui_target.exists():
        if tui_config.is_symlink() or tui_config.exists():
            tui_config.unlink()
        os.symlink(paths.relative_target_tui(name), tui_config)
    elif tui_config.is_symlink():
        tui_config.unlink()

    # Sync skills symlinks
    from opencode_profiles.skills import sync_skills
    sync_skills(paths, name)
```

- [ ] **步骤 4：运行测试验证通过**

```bash
uv run pytest tests/test_ops.py -v
```
预期：all passed (existing + new)

- [ ] **步骤 5：Commit**

```bash
git add opencode_profiles/ops.py tests/test_ops.py
git commit -m "feat(ops): integrate skills management into init/create/switch flow"
```

---

## 任务 6：修改 cli.py — 添加 skill 命令

**文件：**
- 修改：`opencode_profiles/cli.py`
- 修改：`tests/test_cli.py`

- [ ] **步骤 1：编写失败的测试**

在 `tests/test_cli.py` 末尾添加：

```python
class TestSkillCommands:
    def test_add_skill_command(self, runner, cli_paths, monkeypatch, tmp_path):
        """CLI: --add-skill rtk --profile work"""
        from opencode_profiles.ops import create_empty, ensure_initialized
        from opencode_profiles.skills import read_skills_yml

        # Setup skill sources
        src = tmp_path / "skill-sources"
        src.mkdir()
        (src / "rtk").mkdir()
        (src / "rtk" / "SKILL.md").write_text("# rtk")
        monkeypatch.setattr(cli_paths, "_skill_sources_dir", src)
        monkeypatch.setattr("opencode_profiles.skills.DB_PATH", tmp_path / "nonexistent.db")

        ensure_initialized(cli_paths)
        create_empty(cli_paths, "work")
        monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)

        result = runner.invoke(main, ["--add-skill", "rtk", "--profile", "work"])
        assert result.exit_code == 0
        assert "Added skill" in result.output
        assert read_skills_yml(cli_paths, "work") == ["rtk"]

    def test_remove_skill_command(self, runner, cli_paths, monkeypatch, tmp_path):
        """CLI: --remove-skill rtk --profile work"""
        from opencode_profiles.ops import create_empty, ensure_initialized
        from opencode_profiles.skills import read_skills_yml, write_skills_yml

        src = tmp_path / "skill-sources"
        src.mkdir()
        (src / "rtk").mkdir()
        monkeypatch.setattr(cli_paths, "_skill_sources_dir", src)
        monkeypatch.setattr("opencode_profiles.skills.DB_PATH", tmp_path / "nonexistent.db")

        ensure_initialized(cli_paths)
        create_empty(cli_paths, "work")
        write_skills_yml(cli_paths, "work", ["rtk"])
        monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)

        result = runner.invoke(main, ["--remove-skill", "rtk", "--profile", "work"])
        assert result.exit_code == 0
        assert "Removed skill" in result.output
        assert read_skills_yml(cli_paths, "work") == []

    def test_add_skill_requires_profile(self, runner, cli_paths, monkeypatch):
        """--add-skill 无 --profile 时报错。"""
        monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)
        result = runner.invoke(main, ["--add-skill", "rtk"])
        assert result.exit_code != 0
        assert "requires --profile" in result.output
```

- [ ] **步骤 2：运行测试验证失败**

```bash
uv run pytest tests/test_cli.py::TestSkillCommands -v
```
预期：FAIL

- [ ] **步骤 3：修改 cli.py**

在 `cli.py` 顶部更新 import：

```python
from opencode_profiles.ops import (
    backup,
    create_empty,
    create_from_current,
    get_active,
    list_profiles,
    switch,
)
from opencode_profiles.skills import add_skill, remove_skill
```

添加 CLI 选项和逻辑：

```python
@click.command()
@click.option("-b", "--backup", "backup_flag", is_flag=True, help="备份当前配置")
@click.option("-c", "--create", type=str, help="从当前配置创建新 profile")
@click.option("-e", "--empty", type=str, help="创建空 profile")
@click.option("-s", "--switch", "switch_name", type=str, help="切换到指定 profile")
@click.option("-l", "--list", "list_flag", is_flag=True, help="列出所有 profile")
@click.option("--from-current", is_flag=True, help="从当前配置导入 provider（配合 -e 使用）")
@click.option("--from-profile", type=str, help="从指定 profile 导入 provider（配合 -e 使用）")
@click.option("--add-skill", type=str, help="Add a skill to a profile (requires --profile)")
@click.option("--remove-skill", type=str, help="Remove a skill from a profile (requires --profile)")
@click.option("--profile", type=str, help="Target profile for --add-skill/--remove-skill")
def main(backup_flag, create, empty, switch_name, list_flag, from_current, from_profile,
         add_skill_name, remove_skill_name, profile):
    """opencode-profiles — 多配置管理工具。"""
    if from_current and from_profile:
        raise click.ClickException("--from-current and --from-profile are mutually exclusive")
    if (from_current or from_profile) and not empty:
        raise click.ClickException("--from-current/--from-profile can only be used with -e")
    if from_profile == "current":
        raise click.ClickException(
            "'current' is a reserved name and cannot be used as --from-profile value"
        )

    if backup_flag:
        name = backup(paths)
        click.echo(f"Backed up to '{name}'")
    elif create:
        try:
            create_from_current(paths, create)
            click.echo(f"Created profile '{create}' from current config")
        except FileExistsError as e:
            raise click.ClickException(str(e)) from e
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
            raise click.ClickException(str(e)) from e
        except FileNotFoundError as e:
            raise click.ClickException(str(e)) from e
        except ValueError as e:
            raise click.ClickException(str(e)) from e
    elif switch_name:
        try:
            switch(paths, switch_name)
            click.echo(f"Switched to '{switch_name}'")
        except FileNotFoundError as e:
            raise click.ClickException(str(e)) from e
    elif add_skill_name:
        if not profile:
            raise click.ClickException("--add-skill requires --profile")
        try:
            add_skill(paths, profile, add_skill_name)
            click.echo(f"Added skill '{add_skill_name}' to profile '{profile}'")
        except FileNotFoundError as e:
            raise click.ClickException(str(e)) from e
    elif remove_skill_name:
        if not profile:
            raise click.ClickException("--remove-skill requires --profile")
        remove_skill(paths, profile, remove_skill_name)
        click.echo(f"Removed skill '{remove_skill_name}' from profile '{profile}'")
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
预期：all passed

- [ ] **步骤 5：Commit**

```bash
git add opencode_profiles/cli.py tests/test_cli.py
git commit -m "feat(cli): add --add-skill and --remove-skill commands"
```

---

## 任务 7：运行完整测试套件 + lint + typecheck

- [ ] **步骤 1：运行所有测试**

```bash
uv run pytest -v
```
预期：all passed

- [ ] **步骤 2：运行 lint**

```bash
uv run ruff check .
uv run ruff format --check .
```
预期：all clean

- [ ] **步骤 3：运行 typecheck**

```bash
uv run ty check opencode_profiles/
```
预期：no errors

- [ ] **步骤 4：如有问题，修复并重新运行**

- [ ] **步骤 5：最终 Commit（如有修复）**

```bash
git add -A
git commit -m "chore: pass all lint and type checks" --allow-empty
```
