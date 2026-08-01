import json
import os
import shutil
from pathlib import Path

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
from opencode_profiles.skills import read_skills_yml


class TestSkillsIntegration:
    def test_ensure_initialized_creates_skills_yml(
        self, paths, existing_config, skill_sources, monkeypatch
    ):
        """ensure_initialized 后 default profile 有 skills.yml。"""
        monkeypatch.setattr(paths, "_skill_sources_dir", skill_sources)
        ensure_initialized(paths)
        yml = paths.profile_skills_yml("default")
        assert yml.exists()

    def test_create_from_current_copies_skills(
        self, paths, existing_config, skill_sources, monkeypatch
    ):
        """create_from_current 扫描当前 skills 并写入新 profile。"""
        monkeypatch.setattr(paths, "_skill_sources_dir", skill_sources)
        skills_dir = paths.base_dir / "skills"
        skills_dir.mkdir()

        os.symlink(skill_sources / "rtk", skills_dir / "rtk")
        ensure_initialized(paths)
        create_from_current(paths, "work")
        assert read_skills_yml(paths, "work") == ["rtk"]

    def test_switch_syncs_skills(self, paths, existing_config, skill_sources, monkeypatch):
        """switch 后 opencode/skills/ 的 symlinks 匹配目标 profile。"""
        from opencode_profiles.skills import write_skills_yml

        monkeypatch.setattr(paths, "_skill_sources_dir", skill_sources)
        skills_dir = paths.base_dir / "skills"
        skills_dir.mkdir()

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


def test_switch_with_tui_json(paths, existing_config, existing_tui_config):
    ensure_initialized(paths)
    create_from_current(paths, "work")
    switch(paths, "work")
    assert paths.tui_config_file.is_symlink()
    assert paths.tui_config_file.resolve() == paths.profile_tui_config("work").resolve()


def test_switch_to_profile_without_tui(paths, existing_config, existing_tui_config):
    ensure_initialized(paths)
    create_empty(paths, "work")  # no tui.json
    switch(paths, "work")
    # tui.json symlink should be removed if it existed
    assert not paths.tui_config_file.is_symlink()


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

    def test_create_empty_from_current_no_tui(self, paths, existing_config, existing_tui_config):
        """从当前配置导入 provider 时，不复制 tui.json。"""
        ensure_initialized(paths)
        create_empty(paths, "work", source="current")
        assert not paths.profile_tui_config("work").exists()

    def test_create_empty_from_current_no_skills(
        self, paths, existing_config, skill_sources, monkeypatch
    ):
        """从当前配置导入 provider 时，不导入 skills。"""
        import os

        monkeypatch.setattr(paths, "_skill_sources_dir", skill_sources)
        skills_dir = paths.base_dir / "skills"
        skills_dir.mkdir()
        os.symlink(skill_sources / "rtk", skills_dir / "rtk")
        ensure_initialized(paths)
        create_empty(paths, "work", source="current")
        assert read_skills_yml(paths, "work") == []

    def test_create_empty_backward_compatible(self, paths, existing_config):
        """不传 source 时行为不变（写入 {}）。"""
        ensure_initialized(paths)
        create_empty(paths, "empty")
        content = json.loads(paths.profile_config("empty").read_text())
        assert content == {}

    def test_ensure_initialized_existing_setup_creates_skills_yml(
        self, paths, existing_config, skill_sources, monkeypatch
    ):
        """存量用户（opencode.json 已是 symlink）升级后首次运行也会创建 skills.yml。"""

        monkeypatch.setattr(paths, "_skill_sources_dir", skill_sources)
        # 模拟存量用户的旧版本状态：已有 symlink + skills，但无 skills.yml
        ensure_initialized(paths)  # 旧版本首次初始化，创建 default profile
        # 删除 skills.yml 模拟旧版本没有它
        yml = paths.profile_skills_yml("default")
        if yml.exists():
            yml.unlink()
        # 添加 skills symlink（用户已有的）
        skills_dir = paths.base_dir / "skills"
        skills_dir.mkdir(exist_ok=True)
        os.symlink(skill_sources / "rtk", skills_dir / "rtk")
        # 存量用户升级后再次调用 ensure_initialized
        ensure_initialized(paths)
        assert paths.profile_skills_yml("default").exists()
        assert read_skills_yml(paths, "default") == ["rtk"]


class TestCreateOverwriteExisting:
    """测试 create 命令覆盖已有 profile。"""

    def test_create_from_current_overwrites_existing(
        self, paths, existing_config, skill_sources, monkeypatch
    ):
        """profile 已存在时被覆盖重建。"""
        monkeypatch.setattr(paths, "_skill_sources_dir", skill_sources)
        ensure_initialized(paths)
        # 先创建一个 work profile
        create_from_current(paths, "work")
        original_config = paths.profile_config("work").read_text()
        # 修改当前配置
        current = paths.config_file.resolve()
        new_content = json.loads(current.read_text())
        new_content["provider"]["new_provider"] = {"name": "New"}
        current.write_text(json.dumps(new_content, indent=2))
        # 再次 create work，应覆盖
        create_from_current(paths, "work")
        assert paths.profile_config("work").exists()
        assert paths.profile_config("work").read_text() != original_config

    def test_create_empty_overwrites_existing(
        self, paths, existing_config, skill_sources, monkeypatch
    ):
        """create_empty 覆盖已有 profile。"""
        monkeypatch.setattr(paths, "_skill_sources_dir", skill_sources)
        ensure_initialized(paths)
        # 先创建 work
        create_empty(paths, "work")
        assert paths.profile_config("work").read_text() == "{}"
        # 修改 work 内容
        paths.profile_config("work").write_text('{"modified": true}')
        # 再次 create empty work，应覆盖
        create_empty(paths, "work")
        assert paths.profile_config("work").read_text() == "{}"

    def test_rm_rf_then_create_default(
        self, paths, existing_config, skill_sources, sample_config, monkeypatch
    ):
        """rm -rf * 后 -c default 应成功创建。"""
        monkeypatch.setattr(paths, "_skill_sources_dir", skill_sources)
        ensure_initialized(paths)
        # 模拟 rm -rf profiles/*
        profiles_dir = paths.profiles_dir
        for item in profiles_dir.iterdir():
            if item.is_dir():
                shutil.rmtree(item)
        # 此时 symlink 悬空，ensure_initialized 会恢复 default
        # 但 -c default 应覆盖它
        create_from_current(paths, "default")
        assert paths.profile_config("default").exists()
        assert paths.config_file.is_symlink()
        assert paths.config_file.exists()
        # 验证内容保留
        content = json.loads(paths.profile_config("default").read_text())
        assert content == sample_config


class TestDanglingSymlinkRecovery:
    """测试悬空 symlink 的恢复逻辑。"""

    def test_dangling_opencode_symlink_restores_from_bak(self, paths, existing_config):
        """opencode.json 悬空 symlink 时，从 .bak 恢复创建 default profile。"""

        ensure_initialized(paths)
        # 模拟用户 rm -rf profiles/* 后 symlink 悬空
        profiles_dir = paths.profiles_dir
        for item in profiles_dir.iterdir():
            if item.is_dir():
                shutil.rmtree(item)
        # opencode.json 现在是悬空 symlink
        config = paths.config_file
        assert config.is_symlink()
        assert not config.exists()
        # .bak 文件仍在
        bak = paths.base_dir / "opencode.json.bak"
        assert bak.exists()
        # 重新初始化应恢复
        ensure_initialized(paths)
        default_config = paths.profile_config("default")
        assert default_config.exists()
        assert config.is_symlink()
        assert config.exists()

    def test_dangling_tui_symlink_recreated(self, paths, existing_config, existing_tui_config):
        """tui.json 悬空 symlink 时被重新创建指向 default profile。"""

        ensure_initialized(paths)
        # 删除 profiles 目录使 symlink 悬空
        profiles_dir = paths.profiles_dir
        for item in profiles_dir.iterdir():
            if item.is_dir():
                shutil.rmtree(item)
        tui_config = paths.tui_config_file
        assert tui_config.is_symlink()
        assert not tui_config.exists()
        # 初始化应重新创建 tui.json symlink
        ensure_initialized(paths)
        assert tui_config.is_symlink()
        assert tui_config.exists()
        # default profile 应有 tui.json
        default_tui = paths.profile_tui_config("default")
        assert default_tui.exists()

    def test_rm_rf_then_create_default_copies_tui(
        self, paths, existing_config, existing_tui_config, skill_sources, monkeypatch
    ):
        """rm -rf * 后 -c default 应成功复制 tui.json。"""

        monkeypatch.setattr(paths, "_skill_sources_dir", skill_sources)
        ensure_initialized(paths)
        # 模拟 rm -rf profiles/*
        profiles_dir = paths.profiles_dir
        for item in profiles_dir.iterdir():
            if item.is_dir():
                shutil.rmtree(item)
        # -c default 应覆盖重建并复制 tui.json
        create_from_current(paths, "default")
        assert paths.profile_config("default").exists()
        assert paths.config_file.is_symlink()
        assert paths.config_file.exists()
        # tui.json 应被复制到 default profile
        default_tui = paths.profile_tui_config("default")
        assert default_tui.exists()
        tui_config = paths.tui_config_file
        assert tui_config.is_symlink()
        assert tui_config.exists()

    def test_no_bak_creates_empty_config(self, paths):
        """无 .bak 文件时，悬空 symlink 恢复后创建空配置。"""

        # 创建悬空 symlink（无任何配置文件）
        config = paths.config_file
        config.parent.mkdir(parents=True, exist_ok=True)
        os.symlink(Path("profiles") / "default" / "opencode.json", config)
        assert config.is_symlink()
        assert not config.exists()
        # 初始化应创建空配置的 default profile
        ensure_initialized(paths)
        default_config = paths.profile_config("default")
        assert default_config.exists()
        assert default_config.read_text() == "{}"
