import pytest
from click.testing import CliRunner
from opencode_profiles.cli import main
from opencode_profiles.paths import OpenCodePaths


@pytest.fixture
def runner():
    return CliRunner()


@pytest.fixture
def cli_paths(tmp_path):
    base = tmp_path / ".config" / "opencode"
    base.mkdir(parents=True)
    config = base / "opencode.json"
    config.write_text('{"shell": "bash"}')
    return OpenCodePaths(base_dir=base)


def test_list_command(runner, cli_paths, monkeypatch):
    monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)
    from opencode_profiles.ops import ensure_initialized
    ensure_initialized(cli_paths)
    result = runner.invoke(main, ["-l"])
    assert result.exit_code == 0
    assert "default" in result.output


def test_create_command(runner, cli_paths, monkeypatch):
    monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)
    result = runner.invoke(main, ["-c", "work"])
    assert result.exit_code == 0
    assert "work" in result.output


def test_create_empty_command(runner, cli_paths, monkeypatch):
    monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)
    result = runner.invoke(main, ["-e", "minimal"])
    assert result.exit_code == 0
    assert "minimal" in result.output


def test_switch_command(runner, cli_paths, monkeypatch):
    monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)
    runner.invoke(main, ["-c", "work"])
    result = runner.invoke(main, ["-s", "work"])
    assert result.exit_code == 0
    assert "work" in result.output


def test_backup_command(runner, cli_paths, monkeypatch):
    monkeypatch.setattr("opencode_profiles.cli.paths", cli_paths)
    result = runner.invoke(main, ["-b"])
    assert result.exit_code == 0
    assert "backup_" in result.output
