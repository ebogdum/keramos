---
title: "keramos plugin update"
parent: "CLI"
---
{% raw %}
# keramos plugin update

## Synopsis

`keramos plugin update` (aliases `up`, `upgrade`) refreshes an installed plugin. For
a plugin installed from git, it pulls the latest commit; then it re-reads the
plugin's metadata and runs its update hook.

## When to use it

Run it to pick up a newer version of a plugin — a bug fix, a new subcommand, a
changed flag. After updating, run `keramos <name> --help` to see what changed.

## What happens

1. keramos finds the plugin's directory under `~/.config/keramos/plugins/`, matching
   `<name>` against either the directory name or the `name` in its
   `plugin.yaml`.
2. If that directory is a git checkout, keramos runs `git pull --ff-only` there, so
   the update only applies when it fast-forwards cleanly. A plugin installed
   from a local directory has no git checkout, so this step is skipped.
3. keramos re-reads `plugin.yaml` and re-checks the plugin's command.
4. keramos runs the plugin's update hook if it declares one.
5. keramos prints the plugin's name and version.

## Usage

```
keramos plugin update <name> [flags]
```

## Flags

Inherits the global flags.

## Worked example

Update a git-installed plugin:

```sh
keramos plugin update backup
```

```
Updated plugin: backup v0.5.0
```

You can use an alias:

```sh
keramos plugin up backup
```

## See also

- [`plugin install`](plugin-install.md)
- [`plugin list`](plugin-list.md)
- [`plugin remove`](plugin-remove.md)
{% endraw %}
