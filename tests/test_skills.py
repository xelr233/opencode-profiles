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
