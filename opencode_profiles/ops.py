import json
import os
import shutil
from datetime import datetime

from opencode_profiles.paths import OpenCodePaths
from opencode_profiles.skills import scan_current_skills, write_skills_yml


def ensure_initialized(paths: OpenCodePaths) -> None:
    """确保 opencode 配置目录已初始化。

    如果 opencode.json 不是 symlink，将其内容存入 default profile 并替换为 symlink。
    如果已经是有效 symlink（目标存在），不做任何操作。
    如果 symlink 悬空（目标不存在），清理后重新初始化。
    """
    paths.profiles_dir.mkdir(parents=True, exist_ok=True)

    config = paths.config_file

    # Valid symlink: target exists
    if config.is_symlink() and config.exists():
        # Ensure default profile has skills.yml even on existing setups
        if not paths.profile_skills_yml("default").exists():
            current = scan_current_skills(paths)
            write_skills_yml(paths, "default", current)
        return

    # Clean up dangling symlink
    if config.is_symlink():
        config.unlink()

    default_dir = paths.profile_dir("default")
    default_dir.mkdir(parents=True, exist_ok=True)
    default_config = paths.profile_config("default")
    default_tui_config = paths.profile_tui_config("default")
    skills_dir = paths.profile_skills("default")
    skills_dir.mkdir(exist_ok=True)

    if config.exists():
        backup_path = paths.base_dir / "opencode.json.bak"
        if not backup_path.exists():
            shutil.copy2(config, backup_path)
        shutil.copy2(config, default_config)
        config.unlink()
    else:
        # Config doesn't exist (dangling symlink removed or fresh install)
        # Restore from backup if available
        backup_path = paths.base_dir / "opencode.json.bak"
        if backup_path.exists():
            shutil.copy2(backup_path, default_config)
        else:
            default_config.write_text("{}")

    os.symlink(paths.relative_target("default"), config)

    # Handle tui.json (also check for dangling symlink)
    tui_config = paths.tui_config_file
    if tui_config.is_symlink() and not tui_config.exists():
        tui_config.unlink()

    if tui_config.exists() and not tui_config.is_symlink():
        shutil.copy2(tui_config, default_tui_config)
        tui_config.unlink()
        os.symlink(paths.relative_target_tui("default"), tui_config)

    # Write skills.yml for default profile
    if not paths.profile_skills_yml("default").exists():
        current = scan_current_skills(paths)
        write_skills_yml(paths, "default", current)


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

    current_tui = paths.tui_config_file
    if current_tui.is_symlink():
        tui_target = current_tui.resolve()
        shutil.copy2(tui_target, paths.profile_tui_config(backup_name))

    paths.profile_skills(backup_name).mkdir(exist_ok=True)
    return backup_name


def create_from_current(paths: OpenCodePaths, name: str) -> None:
    """从当前激活的 profile 创建新 profile。"""
    ensure_initialized(paths)

    current_config = paths.config_file
    if not current_config.is_symlink():
        raise RuntimeError("config_file is not a symlink after init")
    target = current_config.resolve()

    profile_dir = paths.profile_dir(name)
    if profile_dir.exists():
        # 先复制到临时位置（保持元数据），再 rmtree，再 move 回来——
        # 因为当覆盖当前激活的 profile 时，源和目标是同一个文件，
        # 必须先保存内容再删除目录。
        tmp_path = paths.base_dir / f".opencode.json.tmp.{name}"
        shutil.copy2(target, tmp_path)
        shutil.rmtree(profile_dir)
        profile_dir.mkdir(parents=True)
        paths.profile_skills(name).mkdir(exist_ok=True)
        shutil.move(tmp_path, paths.profile_config(name))
    else:
        profile_dir.mkdir(parents=True)
        paths.profile_skills(name).mkdir(exist_ok=True)
        shutil.copy2(target, paths.profile_config(name))

    active = get_active(paths)
    if active is not None:
        current_tui = paths.profile_tui_config(active)
        if current_tui.exists():
            shutil.copy2(current_tui, paths.profile_tui_config(name))

    # Write skills.yml for new profile
    current = scan_current_skills(paths)
    write_skills_yml(paths, name, current)


def _load_providers(paths: OpenCodePaths, source: str) -> dict:
    """从源配置读取 provider dict。source 为 'current' 或 profile 名。

    Raises:
        FileNotFoundError: 源配置不存在
        ValueError: 源配置无 provider 或 provider 为空
    """
    if source == "current":
        config = paths.config_file
        if not config.is_symlink():
            raise FileNotFoundError("Current config is not a symlink")
        target = config.resolve()
        data = json.loads(target.read_text())
    else:
        config_path = paths.profile_config(source)
        if not config_path.exists():
            raise FileNotFoundError(f"Source profile '{source}' not found")
        data = json.loads(config_path.read_text())

    providers = data.get("provider")
    if not providers:
        raise ValueError("Source config has no providers to import")
    return providers


def create_empty(paths: OpenCodePaths, name: str, source: str | None = None) -> None:
    """创建空 profile，可选从源配置导入 provider。

    Args:
        paths: 路径管理实例
        name: 新 profile 名称
        source: None 表示空配置；"current" 表示从当前激活配置导入；
                其他字符串表示从指定 profile 名称导入
    """
    ensure_initialized(paths)

    profile_dir = paths.profile_dir(name)
    if profile_dir.exists():
        shutil.rmtree(profile_dir)

    providers = _load_providers(paths, source) if source is not None else None

    profile_dir.mkdir(parents=True)
    paths.profile_skills(name).mkdir(exist_ok=True)

    if providers is None:
        paths.profile_config(name).write_text("{}")
    else:
        paths.profile_config(name).write_text(json.dumps({"provider": providers}, indent=2))

    if source is not None:
        if source == "current":
            active = get_active(paths)
            if active is None:
                # Still write skills.yml before returning
                current = scan_current_skills(paths)
                write_skills_yml(paths, name, current)
                return
            src_tui = paths.profile_tui_config(active)
        else:
            src_tui = paths.profile_tui_config(source)
        if src_tui.exists():
            shutil.copy2(src_tui, paths.profile_tui_config(name))

    # Write skills.yml for new profile
    current = scan_current_skills(paths)
    write_skills_yml(paths, name, current)


def switch(paths: OpenCodePaths, name: str) -> None:
    """切换 symlink 指向目标 profile。"""
    ensure_initialized(paths)

    target = paths.profile_config(name)
    if not target.exists():
        available = list_profiles(paths)
        raise FileNotFoundError(f"Profile '{name}' not found. Available: {available}")

    config = paths.config_file
    config.parent.mkdir(parents=True, exist_ok=True)

    if config.is_symlink() or config.exists():
        config.unlink()

    os.symlink(paths.relative_target(name), config)

    # 管理 tui.json symlink
    tui_config = paths.tui_config_file
    tui_target = paths.profile_tui_config(name)

    if tui_target.exists():
        if tui_config.is_symlink() or tui_config.exists():
            tui_config.unlink()
        os.symlink(paths.relative_target_tui(name), tui_config)
    elif tui_config.is_symlink():
        tui_config.unlink()

    # Sync skills symlinks
    from opencode_profiles.skills import sync_skills

    sync_skills(paths, name)


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
