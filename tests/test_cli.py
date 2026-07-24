import json
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


class TestEmptyWithProviderImport:
    """测试 -e 配合 --from-current / --from-profile。"""

    def test_empty_from_current(self, paths, existing_config, sample_config, monkeypatch):
        """CLI: -e work --from-current"""
        monkeypatch.setattr("opencode_profiles.cli.paths", paths)
        from click.testing import CliRunner
        runner = CliRunner()
        result = runner.invoke(main, ["-e", "work", "--from-current"])
        assert result.exit_code == 0
        assert "providers from current" in result.output
        content = json.loads(paths.profile_config("work").read_text())
        assert content == {"provider": sample_config["provider"]}

    def test_empty_from_profile(self, paths, existing_config, sample_config, monkeypatch):
        """CLI: -e work --from-profile personal"""
        from opencode_profiles.ops import ensure_initialized, create_from_current
        ensure_initialized(paths)
        create_from_current(paths, "personal")
        monkeypatch.setattr("opencode_profiles.cli.paths", paths)
        from click.testing import CliRunner
        runner = CliRunner()
        result = runner.invoke(main, ["-e", "work", "--from-profile", "personal"])
        assert result.exit_code == 0
        assert "providers from" in result.output
        content = json.loads(paths.profile_config("work").read_text())
        assert content == {"provider": sample_config["provider"]}

    def test_from_current_without_empty(self, paths, existing_config, monkeypatch):
        """--from-current 无 -e 时报错。"""
        monkeypatch.setattr("opencode_profiles.cli.paths", paths)
        from click.testing import CliRunner
        runner = CliRunner()
        result = runner.invoke(main, ["--from-current"])
        assert result.exit_code != 0
        assert "can only be used with -e" in result.output

    def test_from_profile_without_empty(self, paths, existing_config, monkeypatch):
        """--from-profile 无 -e 时报错。"""
        monkeypatch.setattr("opencode_profiles.cli.paths", paths)
        from click.testing import CliRunner
        runner = CliRunner()
        result = runner.invoke(main, ["--from-profile", "personal"])
        assert result.exit_code != 0
        assert "can only be used with -e" in result.output

    def test_mutually_exclusive(self, paths, existing_config, monkeypatch):
        """--from-current 和 --from-profile 同时传报错。"""
        monkeypatch.setattr("opencode_profiles.cli.paths", paths)
        from click.testing import CliRunner
        runner = CliRunner()
        result = runner.invoke(main, ["-e", "work", "--from-current", "--from-profile", "x"])
        assert result.exit_code != 0
        assert "mutually exclusive" in result.output

    def test_empty_from_current_no_provider(self, paths, monkeypatch):
        """源配置无 provider 时 CLI 报错。"""
        from opencode_profiles.ops import ensure_initialized
        ensure_initialized(paths)
        # 创建一个无 provider 的 profile
        no_prov_dir = paths.profile_dir("no_prov")
        no_prov_dir.mkdir(parents=True)
        paths.profile_config("no_prov").write_text('{"shell": "bash"}')
        paths.profile_skills("no_prov").mkdir(exist_ok=True)
        # 切换当前到 no_prov
        cfg = paths.config_file
        if cfg.is_symlink() or cfg.exists():
            cfg.unlink()
        import os
        os.symlink(paths.relative_target("no_prov"), cfg)
        monkeypatch.setattr("opencode_profiles.cli.paths", paths)
        from click.testing import CliRunner
        runner = CliRunner()
        result = runner.invoke(main, ["-e", "work", "--from-current"])
        assert result.exit_code != 0
        assert "no providers" in result.output.lower()
