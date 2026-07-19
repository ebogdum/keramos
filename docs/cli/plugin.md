---
title: "keramos plugin"
parent: "CLI"
---
{% raw %}
# keramos plugin

## Synopsis

`keramos plugin` manages the extra commands you bolt onto keramos. A plugin is a small
program plus a `plugin.yaml` that names it; once installed, you run it as if it
were built in — `keramos <plugin-name> [args]`.

Plugins live in `~/.config/keramos/plugins/`, one directory per plugin. Everything
here is local to your machine and your user account — managing plugins never
touches a cluster.

Want to **build** one? See the [Plugins guide](../guides/plugins.md) — a plugin
is any executable (not just Go) that keramos runs as a subprocess.

## Subcommands

| Command | What it does |
|---|---|
| [`install`](plugin-install.md) | Install a plugin from a git URL or a local directory |
| [`list`](plugin-list.md) | Show the plugins you have installed |
| [`update`](plugin-update.md) | Pull the newest version of an installed plugin |
| [`remove`](plugin-remove.md) | Uninstall a plugin |

## Usage

```
keramos plugin [command]
```

Once a plugin is installed you invoke it directly — keramos treats an unknown
command as a plugin name:

```sh
keramos plugin install https://github.com/acme/keramos-backup.git
keramos backup --release web        # "backup" now runs as a keramos command
```

## See also

- [Plugins guide](../guides/plugins.md) — how to build and distribute a plugin
- [`marketplace`](marketplace.md) — find and verify signed plugins to install
- [`plugin install`](plugin-install.md)
- [`plugin list`](plugin-list.md)
{% endraw %}
