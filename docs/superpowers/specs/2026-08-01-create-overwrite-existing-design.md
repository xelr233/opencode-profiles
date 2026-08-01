# Create Overwrite Existing Profile

## Problem

After the dangling symlink fix in `ensure_initialized`, running `rm -rf *` in the profiles directory followed by `opencode-profiles -c default` fails with "Profile 'default' already exists". This happens because `ensure_initialized` recovers the dangling symlink by creating a `default` profile from `.bak`, and then `create_from_current` sees the existing directory and raises `FileExistsError`.

## Decision

When `create_from_current` or `create_empty` is called with a profile name that already exists, remove the existing profile directory and recreate it. The user's explicit create command takes precedence over a profile created by recovery logic.

## Changes

### `opencode_profiles/ops.py`

Both `create_from_current` and `create_empty`: before `profile_dir.mkdir(parents=True)`, check if the directory exists and `shutil.rmtree` it first.

```python
profile_dir = paths.profile_dir(name)
if profile_dir.exists():
    shutil.rmtree(profile_dir)
profile_dir.mkdir(parents=True)
```

### `tests/test_ops.py`

Add tests:
- `test_create_from_current_overwrites_existing` — existing profile is replaced
- `test_create_empty_overwrites_existing` — same for create_empty
- End-to-end: `rm -rf *` scenario then `-c default` succeeds

## Rationale

- Profiles are derived data (copied from current config or created empty), not user-owned unique data
- An explicit `-c <name>` / `-e <name>` command expresses clear intent to create that profile
- Recovery-created profiles are a side effect of symlink repair, not user intent
- No `--force` flag needed: the create command is already explicit
