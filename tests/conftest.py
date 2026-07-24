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


@pytest.fixture
def existing_tui_config(tmp_opencode):
    """在临时目录中创建实际的 tui.json 配置文件。"""
    tui_file = tmp_opencode / "tui.json"
    tui_file.write_text(json.dumps({"theme": "dark", "fontSize": 14}, indent=2))
    return tui_file
