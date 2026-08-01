from pathlib import Path

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


def test_paths_tui_config_file():
    p = OpenCodePaths()
    assert p.tui_config_file.name == "tui.json"


def test_paths_profile_tui_config():
    p = OpenCodePaths()
    assert p.profile_tui_config("work").name == "tui.json"


def test_paths_relative_target_tui():
    p = OpenCodePaths()
    assert str(p.relative_target_tui("work")).endswith("profiles/work/tui.json")


class TestSkillPaths:
    def test_skill_sources_dir_default(self, paths):
        assert paths.skill_sources_dir == Path.home() / ".cc-switch" / "skills"

    def test_skill_sources_dir_custom(self, tmp_path):
        custom = tmp_path / "my-skills"
        p = OpenCodePaths(base_dir=tmp_path / "opencode", skill_sources_dir=custom)
        assert p.skill_sources_dir == custom

    def test_profile_skills_yml(self, paths):
        assert (
            paths.profile_skills_yml("work") == paths.base_dir / "profiles" / "work" / "skills.yml"
        )

    def test_skill_source(self, paths):
        assert paths.skill_source("rtk") == Path.home() / ".cc-switch" / "skills" / "rtk"
