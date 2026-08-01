import json

import pytest

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
                "options": {"apiKey": "test-key", "baseURL": "https://test.example.com/v1"},
            }
        },
        "shell": "bash",
    }


@pytest.fixture
def existing_config(tmp_opencode, sample_config):
    """在临时目录中创建实际的 opencode.json 配置文件。"""
    config_file = tmp_opencode / "opencode.json"
    config_file.write_text(json.dumps(sample_config, indent=2))
    return config_file


@pytest.fixture
def existing_tui_config(tmp_opencode):
    """在临时目录中创建实际的 tui.json 配置文件。"""
    tui_file = tmp_opencode / "tui.json"
    tui_file.write_text(json.dumps({"theme": "dark", "fontSize": 14}, indent=2))
    return tui_file


@pytest.fixture(autouse=True)
def _skip_db_update(tmp_path, monkeypatch):
    """Prevent tests from modifying the real cc-switch.db.

    Safe to run before skills.py exists (skips if module not importable
    or DB_PATH not yet defined).
    """
    import importlib

    try:
        mod = importlib.import_module("opencode_profiles.skills")
    except ImportError:
        return
    if hasattr(mod, "DB_PATH"):
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
