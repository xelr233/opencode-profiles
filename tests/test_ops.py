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


def test_init_migrates_tui_json(paths, existing_config, existing_tui_config):
    ensure_initialized(paths)
    assert paths.tui_config_file.is_symlink()
    assert paths.tui_config_file.resolve() == paths.profile_tui_config("default").resolve()


def test_init_without_tui_json(paths, existing_config):
    ensure_initialized(paths)
    assert not paths.tui_config_file.exists()


def test_init_tui_json_already_symlink(paths, existing_config, existing_tui_config):
    ensure_initialized(paths)
    ensure_initialized(paths)
    assert paths.tui_config_file.is_symlink()


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


def test_backup_with_tui_json(paths, existing_config, existing_tui_config):
    ensure_initialized(paths)
    name = backup(paths)
    assert paths.profile_tui_config(name).exists()


def test_backup_without_tui_json(paths, existing_config):
    ensure_initialized(paths)
    name = backup(paths)
    assert not paths.profile_tui_config(name).exists()


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


def test_create_from_current_with_tui(paths, existing_config, existing_tui_config):
    ensure_initialized(paths)
    create_from_current(paths, "work")
    assert paths.profile_tui_config("work").exists()


def test_create_from_current_without_tui(paths, existing_config):
    ensure_initialized(paths)
    create_from_current(paths, "work")
    assert not paths.profile_tui_config("work").exists()


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


class TestCreateEmptyWithSource:
    """测试 create_empty 的 source 参数功能。"""

    def test_create_empty_with_from_current(self, paths, existing_config, sample_config):
        """从当前配置导入 provider 创建新 profile。"""
        ensure_initialized(paths)
        create_empty(paths, "work", source="current")
        content = json.loads(paths.profile_config("work").read_text())
        assert content == {"provider": sample_config["provider"]}

    def test_create_empty_with_from_profile(self, paths, existing_config, sample_config):
        """从指定 profile 导入 provider 创建新 profile。"""
        create_from_current(paths, "personal")
        create_empty(paths, "work", source="personal")
        content = json.loads(paths.profile_config("work").read_text())
        assert content == {"provider": sample_config["provider"]}

    def test_create_empty_source_not_found(self, paths, existing_config):
        """源 profile 不存在时报错。"""
        ensure_initialized(paths)
        with pytest.raises(FileNotFoundError):
            create_empty(paths, "work", source="nonexistent")

    def test_create_empty_source_no_provider(self, paths, existing_config, tmp_path):
        """源配置无 provider 时报错。"""
        ensure_initialized(paths)
        no_provider_dir = paths.profile_dir("no_provider")
        no_provider_dir.mkdir(parents=True)
        paths.profile_config("no_provider").write_text('{"shell": "bash"}')
        paths.profile_skills("no_provider").mkdir(exist_ok=True)
        with pytest.raises(ValueError, match="no providers"):
            create_empty(paths, "work", source="no_provider")

    def test_create_empty_with_tui_from_current(self, paths, existing_config, existing_tui_config):
        """从当前配置导入时，tui.json 也被复制。"""
        ensure_initialized(paths)
        create_empty(paths, "work", source="current")
        assert paths.profile_tui_config("work").exists()

    def test_create_empty_without_tui_from_current(self, paths, existing_config):
        """当前配置无 tui.json 时不创建。"""
        ensure_initialized(paths)
        create_empty(paths, "work", source="current")
        assert not paths.profile_tui_config("work").exists()

    def test_create_empty_backward_compatible(self, paths, existing_config):
        """不传 source 时行为不变（写入 {}）。"""
        ensure_initialized(paths)
        create_empty(paths, "empty")
        content = json.loads(paths.profile_config("empty").read_text())
        assert content == {}
