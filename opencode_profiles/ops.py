import json
import os
import shutil
from datetime import datetime
from opencode_profiles.paths import OpenCodePaths


def ensure_initialized(paths: OpenCodePaths) -> None:
    """确保 opencode 配置目录已初始化。

    如果 opencode.json 不是 symlink，将其内容存入 default profile 并替换为 symlink。
    如果已经是 symlink，不做任何操作。
    """
    paths.profiles_dir.mkdir(parents=True, exist_ok=True)

    config = paths.config_file

    if config.is_symlink():
        return

    default_dir = paths.profile_dir("default")
    default_dir.mkdir(parents=True, exist_ok=True)
    default_config = paths.profile_config("default")
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


def backup(paths: OpenCodePaths) -> str:
    """备份当前激活的 profile 配置。返回备份目录名称。"""
    ensure_initialized(paths)

    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    backup_name = f"backup_{timestamp}"
    backup_dir = paths.profile_dir(backup_name)
    backup_dir.mkdir(parents=True, exist_ok=True)

    current_config = paths.config_file
    if current_config.is_symlink():
        target = current_config.resolve()
        shutil.copy2(target, paths.profile_config(backup_name))
    else:
        raise RuntimeError("config_file is not a symlink after init")

    paths.profile_skills(backup_name).mkdir(exist_ok=True)
    return backup_name


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


def create_empty(paths: OpenCodePaths, name: str) -> None:
    """创建空 profile（最小合法 JSON）。"""
    ensure_initialized(paths)

    profile_dir = paths.profile_dir(name)
    if profile_dir.exists():
        raise FileExistsError(f"Profile '{name}' already exists")

    profile_dir.mkdir(parents=True)
    paths.profile_config(name).write_text("{}")
    paths.profile_skills(name).mkdir(exist_ok=True)


def switch(paths: OpenCodePaths, name: str) -> None:
    """切换 symlink 指向目标 profile。"""
    ensure_initialized(paths)

    target = paths.profile_config(name)
    if not target.exists():
        available = list_profiles(paths)
        raise FileNotFoundError(
            f"Profile '{name}' not found. Available: {available}"
        )

    config = paths.config_file
    config.parent.mkdir(parents=True, exist_ok=True)

    if config.is_symlink() or config.exists():
        config.unlink()

    os.symlink(paths.relative_target(name), config)


def list_profiles(paths: OpenCodePaths) -> list[str]:
    """列出所有 profile 名称。"""
    if not paths.profiles_dir.exists():
        return []

    profiles = []
    for d in sorted(paths.profiles_dir.iterdir()):
        if d.is_dir() and (d / "opencode.json").exists():
            profiles.append(d.name)
    return profiles


def get_active(paths: OpenCodePaths) -> str | None:
    """获取当前激活的 profile 名称。"""
    config = paths.config_file
    if not config.is_symlink():
        return None

    target = config.resolve()
    parts = target.relative_to(paths.base_dir).parts
    if len(parts) >= 2 and parts[0] == "profiles":
        return parts[1]
    return None
