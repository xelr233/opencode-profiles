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
def main(backup_flag, create, empty, switch_name, list_flag):
    """opencode-profiles — 多配置管理工具。"""
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
            create_empty(paths, empty)
            click.echo(f"Created empty profile '{empty}'")
        except FileExistsError as e:
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
