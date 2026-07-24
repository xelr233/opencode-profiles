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
