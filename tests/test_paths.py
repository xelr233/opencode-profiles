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
