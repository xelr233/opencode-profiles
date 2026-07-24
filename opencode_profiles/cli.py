import click

from opencode_profiles.ops import (
    backup,
    create_empty,
    create_from_current,
    get_active,
    list_profiles,
    switch,
)
from opencode_profiles.paths import OpenCodePaths


paths = OpenCodePaths()


@click.command()
@click.option("-b", "--backup", "backup_flag", is_flag=True, help="备份当前配置")
@click.option("-c", "--create", type=str, help="从当前配置创建新 profile")
@click.option("-e", "--empty", type=str, help="创建空 profile")
@click.option("-s", "--switch", "switch_name", type=str, help="切换到指定 profile")
@click.option("-l", "--list", "list_flag", is_flag=True, help="列出所有 profile")
@click.option("--from-current", is_flag=True, help="从当前配置导入 provider（配合 -e 使用）")
@click.option("--from-profile", type=str, help="从指定 profile 导入 provider（配合 -e 使用）")
def main(backup_flag, create, empty, switch_name, list_flag, from_current, from_profile):
    """opencode-profiles — 多配置管理工具。"""
    if from_current and from_profile:
        raise click.ClickException("--from-current and --from-profile are mutually exclusive")
    if (from_current or from_profile) and not empty:
        raise click.ClickException("--from-current/--from-profile can only be used with -e")
    if from_profile == "current":
        raise click.ClickException("'current' is a reserved name and cannot be used as --from-profile value")

    if backup_flag:
        name = backup(paths)
        click.echo(f"Backed up to '{name}'")
    elif create:
        try:
            create_from_current(paths, create)
            click.echo(f"Created profile '{create}' from current config")
        except FileExistsError as e:
            raise click.ClickException(str(e))
    elif empty:
        try:
            if from_current:
                create_empty(paths, empty, source="current")
                click.echo(f"Created profile '{empty}' with providers from current config")
            elif from_profile:
                create_empty(paths, empty, source=from_profile)
                click.echo(f"Created profile '{empty}' with providers from '{from_profile}'")
            else:
                create_empty(paths, empty)
                click.echo(f"Created empty profile '{empty}'")
        except FileExistsError as e:
            raise click.ClickException(str(e))
        except FileNotFoundError as e:
            raise click.ClickException(str(e))
        except ValueError as e:
            raise click.ClickException(str(e))
    elif switch_name:
        try:
            switch(paths, switch_name)
            click.echo(f"Switched to '{switch_name}'")
        except FileNotFoundError as e:
            raise click.ClickException(str(e))
    elif list_flag:
        profiles = list_profiles(paths)
        active = get_active(paths)
        if not profiles:
            click.echo("No profiles found.")
            return
        for p in profiles:
            marker = " *" if p == active else ""
            click.echo(f"  {p}{marker}")
        if active:
            click.echo(f"\nActive: {active}")
    else:
        click.echo("Use --help for available commands.")
