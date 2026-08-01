# AGENTS.md

## Project

CLI tool to manage multiple [OpenCode](https://opencode.ai) configuration profiles via symlink switching. Create, backup, switch, and list profiles; import providers from existing configs when creating new ones. Installed as `opencode-profiles` (entry point → `opencode_profiles.cli:main`).

## Architecture

The core invariant: `~/.config/opencode/opencode.json` **must always be a symlink** pointing to `profiles/<name>/opencode.json` after initialization. Everything else follows from this.

- `opencode_profiles/paths.py` — `OpenCodePaths` class. All path resolution lives here. `relative_target()` returns the symlink target for a profile.
- `opencode_profiles/ops.py` — Every function calls `ensure_initialized()` first, which enforces the symlink invariant by migrating a bare `opencode.json` into a `default` profile. Don't skip this call when adding new operations.
- `opencode_profiles/cli.py` — Click CLI. Holds a module-level `paths = OpenCodePaths()` singleton that tests monkeypatch.

## Commands

```bash
# Dev install
uv pip install -e .

# Run all tests
uv run pytest -v

# Run a single test file / test
uv run pytest tests/test_ops.py -v
uv run pytest tests/test_cli.py::TestEmptyWithProviderImport -v

# Lint & format check
uv run ruff check .
uv run ruff format --check .

# Type check
uv run ty check opencode_profiles/

# Build for distribution
uv build
```

`ruff` and `ty` are configured in `pyproject.toml`.

## Testing

- Tests inject an isolated `OpenCodePaths` via `tmp_path` + monkeypatch of `opencode_profiles.cli.paths` (see `tests/conftest.py::cli_paths`).
- The `CliRunner` from `click.testing` invokes the CLI; assert on `result.exit_code` and `result.output`.
- Each test gets a fresh temp dir under `tmp_path / ".config" / "opencode"`.

## Conventions

- Commit messages follow Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`).
- Python 3.11+ (see `.python-version`).
- Dependencies managed with `uv`; `uv.lock` is committed.
