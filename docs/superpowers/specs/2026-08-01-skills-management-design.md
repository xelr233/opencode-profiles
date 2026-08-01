# Skills Management Design Spec

**Date:** 2026-08-01
**Status:** Approved

## Overview

Add skills management to opencode-profiles. Each profile records which skills it uses in a `skills.yml` file. On profile switch, the tool computes the diff between current and target skills, then creates/removes symlinks in `~/.config/opencode/skills/` accordingly. The `cc-switch.db` `enabled_opencode` column is updated to reflect active skills.

## Requirements

1. **skills.yml format**: Simple YAML list of skill names (e.g., `[brainstorming, rtk, mavenbuild]`)
2. **Create profile**: Scan current `~/.config/opencode/skills/` symlinks and record them in the new profile's `skills.yml`
3. **Switch profile**: Compute diff between current and target `skills.yml`, create/remove symlinks, update db
4. **Unmanaged skills**: Real directories in `~/.config/opencode/skills/` not in target profile are removed; skills in target are created as symlinks
5. **Source path**: Configurable via `OpenCodePaths.skill_sources_dir`, defaults to `~/.cc-switch/skills/`
6. **DB sync**: After switch, update `enabled_opencode` in `cc-switch.db` to match active profile's skills
7. **Failure mode**: Validate all target skill sources exist BEFORE modifying any symlinks (atomic)
8. **CLI**: `--add-skill <skill> --profile <name>` and `--remove-skill <skill> --profile <name>` flags
9. **SQLite access**: Python stdlib `sqlite3` module

## Architecture

```
opencode_profiles/
├── paths.py    ← + skill_sources_dir, + profile_skills_yml(), + skill_source()
├── skills.py   ← NEW: all skill operations
├── ops.py      ← switch() calls sync_skills(); create_*() scan & write skills.yml
└── cli.py      ← + --add-skill, --remove-skill, --profile flags
```

## Detailed Design

### paths.py additions

```python
class OpenCodePaths:
    def __init__(self, base_dir=None, skill_sources_dir=None):
        self._base_dir = base_dir or Path.home() / ".config" / "opencode"
        self._skill_sources_dir = skill_sources_dir or Path.home() / ".cc-switch" / "skills"

    @property
    def skill_sources_dir(self) -> Path:
        return self._skill_sources_dir

    def profile_skills_yml(self, name: str) -> Path:
        return self.profile_dir(name) / "skills.yml"

    def skill_source(self, name: str) -> Path:
        return self._skill_sources_dir / name
```

### skills.py module

| Function | Signature | Purpose |
|----------|-----------|---------|
| `read_skills_yml` | `(paths, name) -> list[str]` | Read skills.yml; `[]` if missing |
| `write_skills_yml` | `(paths, name, skills) -> None` | Write skills list to skills.yml |
| `scan_current_skills` | `(paths) -> list[str]` | Scan `~/.config/opencode/skills/` for symlinks to `skill_sources_dir` |
| `compute_diff` | `(current, target) -> (to_add, to_remove)` | Set difference, sorted output |
| `sync_skills` | `(paths, target_name, db_path=None) -> None` | Validate, diff, apply symlinks, update db (db_path defaults to `~/.cc-switch/cc-switch.db`) |
| `add_skill` | `(paths, name, skill) -> None` | Add skill to profile; validates source |
| `remove_skill` | `(paths, name, skill) -> None` | Remove skill from profile |
| `_update_db` | `(active_skills) -> None` | Update `enabled_opencode` in cc-switch.db |

### sync_skills() flow

1. Read target profile's `skills.yml` → `target_skills`
2. Get active profile via `get_active()`, read its `skills.yml` → `current_skills`
3. **Pre-validate**: For each skill in `target_skills`, check `paths.skill_source(skill)` exists. If any missing → raise `FileNotFoundError` immediately (no modifications made)
4. Compute diff: `to_add`, `to_remove`
5. Remove: for each skill in `to_remove`, delete symlink/real dir in `~/.config/opencode/skills/`
6. Add: for each skill in `to_add`, create symlink from `~/.config/opencode/skills/<skill>` → `skill_sources_dir/<skill>`
7. Update `cc-switch.db` (path passed via `db_path` param, defaults to `~/.cc-switch/cc-switch.db`): set all `enabled_opencode = 0`, then set `enabled_opencode = 1` for each active skill

### ops.py changes

**ensure_initialized**: After creating default profile's `skills/` dir, if no `skills.yml` exists, scan current skills and write it.

**create_from_current / create_empty**: After creating profile, scan current skills and write `skills.yml` for the new profile.

**switch**: After swapping config/tui symlinks, call `sync_skills(paths, name)`.

### cli.py changes

Add options:
- `--add-skill <skill>`: requires `--profile <name>`, adds skill to that profile's `skills.yml`
- `--remove-skill <skill>`: requires `--profile <name>`, removes skill from that profile's `skills.yml`
- `--profile <name>`: target profile for add/remove operations

### DB update strategy

- Use Python `sqlite3` stdlib module
- `db_path` parameter on `sync_skills()` defaults to `~/.cc-switch/cc-switch.db`
- If db doesn't exist at the given path, skip silently (graceful degradation)
- Transaction: `UPDATE skills SET enabled_opencode = 0` then `UPDATE skills SET enabled_opencode = 1 WHERE name = ?` for each active skill
- Tests monkeypatch `opencode_profiles.skills.DB_PATH` to a tmp file to avoid touching the real db
- Existing `switch()` tests: since skills lists are empty (no symlinks in tmp), `_update_db` sets all `enabled_opencode = 0`; monkeypatch DB_PATH to prevent real db modification

## Testing

New file `tests/test_skills.py` with ~17 tests:

- `test_read_skills_yml_missing` — returns `[]`
- `test_read_skills_yml_exists` — reads correctly
- `test_write_skills_yml` — roundtrip
- `test_scan_current_skills` — detects symlinks
- `test_compute_diff` — correct diff
- `test_sync_skills_add` — creates symlinks
- `test_sync_skills_remove` — removes symlinks
- `test_sync_skills_missing_source_fails` — FileNotFoundError, no partial state
- `test_sync_skills_unmanaged_real_dir` — removes real dirs not in target
- `test_add_skill_to_profile` — appends
- `test_add_skill_already_exists` — no duplicate
- `test_remove_skill_from_profile` — removes
- `test_remove_skill_not_present` — no-op
- `test_ensure_initialized_creates_skills_yml` — default gets yml
- `test_create_from_current_copies_skills` — inherits skills
- `test_switch_syncs_skills` — full switch flow
- `test_cli_add_skill` — CLI integration
- `test_cli_remove_skill` — CLI integration
- `test_update_db_sets_enabled_flag` — verifies db update
- `test_update_db_missing_file` — graceful skip when no db

Existing tests remain unchanged (backward compatible).

## Dependencies

- `pyyaml` — **new dependency**, add to `pyproject.toml` dependencies list
- `sqlite3` — stdlib, no extra dependency

## Migration

- Existing profiles without `skills.yml` are treated as having empty skill list
- On first `ensure_initialized` after upgrade, default profile's `skills.yml` is created from scanning current symlinks
- No destructive migration; existing configs untouched
