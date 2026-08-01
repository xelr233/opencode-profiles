"""Profile skills management: read/write skills.yml, scan, diff, add/remove."""

from __future__ import annotations

import os
import shutil
import sqlite3
from pathlib import Path

import yaml

from opencode_profiles.paths import OpenCodePaths

DB_PATH = Path.home() / ".cc-switch" / "cc-switch.db"


def read_skills_yml(paths: OpenCodePaths, name: str) -> list[str]:
    """Read skills list from a profile's skills.yml. Empty list if missing."""
    yml_path = paths.profile_skills_yml(name)
    if not yml_path.exists():
        return []
    data = yaml.safe_load(yml_path.read_text())
    return data if isinstance(data, list) else []


def write_skills_yml(paths: OpenCodePaths, name: str, skills: list[str]) -> None:
    """Write skills list to a profile's skills.yml."""
    yml_path = paths.profile_skills_yml(name)
    yml_path.parent.mkdir(parents=True, exist_ok=True)
    yml_path.write_text(yaml.safe_dump(skills, default_flow_style=False))


def scan_current_skills(paths: OpenCodePaths) -> list[str]:
    """Scan ~/.config/opencode/skills/ for symlinks pointing to skill_sources_dir."""
    skills_dir = paths.base_dir / "skills"
    if not skills_dir.exists():
        return []
    result = []
    for entry in sorted(skills_dir.iterdir()):
        if entry.is_symlink():
            resolved = entry.resolve()
            if resolved.parent == paths.skill_sources_dir:
                result.append(entry.name)
    return result


def compute_diff(current: list[str], target: list[str]) -> tuple[list[str], list[str]]:
    """Return (to_add, to_remove) based on set difference."""
    cur, tgt = set(current), set(target)
    return sorted(tgt - cur), sorted(cur - tgt)


def add_skill(paths: OpenCodePaths, name: str, skill: str) -> None:
    """Add a skill to a profile's skills.yml. Validates source exists."""
    source = paths.skill_source(skill)
    if not source.exists():
        raise FileNotFoundError(f"Skill source '{skill}' not found at {source}")
    skills = read_skills_yml(paths, name)
    if skill not in skills:
        skills.append(skill)
    write_skills_yml(paths, name, skills)


def remove_skill(paths: OpenCodePaths, name: str, skill: str) -> None:
    """Remove a skill from a profile's skills.yml."""
    skills = read_skills_yml(paths, name)
    if skill in skills:
        skills.remove(skill)
    write_skills_yml(paths, name, skills)


def sync_skills(paths: OpenCodePaths, target_name: str, db_path: Path | None = None) -> None:
    """Sync opencode/skills/ symlinks to match target profile's skills.yml.

    Validates all target sources exist before modifying anything.
    Updates cc-switch.db after successful sync.
    """
    target_skills = read_skills_yml(paths, target_name)

    skills_dir = paths.base_dir / "skills"
    current_skills = []
    if skills_dir.exists():
        current_skills = sorted(entry.name for entry in skills_dir.iterdir())

    for skill in target_skills:
        source = paths.skill_source(skill)
        if not source.exists():
            raise FileNotFoundError(f"Skill source '{skill}' not found at {source}")

    to_add, to_remove = compute_diff(current_skills, target_skills)

    skills_dir.mkdir(exist_ok=True)

    for skill in to_remove:
        link = skills_dir / skill
        if link.is_symlink():
            link.unlink()
        elif link.exists():
            if link.is_dir():
                shutil.rmtree(link)
            else:
                link.unlink()

    for skill in to_add:
        link = skills_dir / skill
        if link.is_symlink():
            link.unlink()
        elif link.exists():
            if link.is_dir():
                shutil.rmtree(link)
            else:
                link.unlink()
        os.symlink(paths.skill_source(skill), link)

    _update_db(target_skills, db_path=db_path)


def _update_db(active_skills: list[str], db_path: Path | None = None) -> None:
    """Update enabled_opencode in cc-switch.db based on active skills."""
    if db_path is None:
        db_path = DB_PATH
    if not db_path.exists():
        return
    try:
        conn = sqlite3.connect(db_path)
        try:
            conn.execute("UPDATE skills SET enabled_opencode = 0")
            for skill in active_skills:
                conn.execute(
                    "UPDATE skills SET enabled_opencode = 1 WHERE name = ?",
                    (skill,),
                )
            conn.commit()
        finally:
            conn.close()
    except sqlite3.Error:
        pass
