import os
import sqlite3
from pathlib import Path

import pytest

from opencode_profiles.skills import (
    add_skill,
    compute_diff,
    read_skills_yml,
    remove_skill,
    scan_current_skills,
    sync_skills,
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
