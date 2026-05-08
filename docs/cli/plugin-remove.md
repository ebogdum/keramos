# keramos plugin remove

## Synopsis

`keramos plugin remove` (aliases: `rm`, `uninstall`) deletes an installed plugin from the local keramos installation. The plugin's directory under `~/.config/keramos/plugins/` is removed; any post-uninstall hook the plugin declares is run before deletion.

## When to use it

Use when retiring a plugin, cleaning up a development install, or troubleshooting (sometimes a clean re-install is faster than reasoning about a broken state).

## What happens when you run it

1. Resolves `<name>` to a plugin directory under `~/.config/keramos/plugins/`.
2. Runs the plugin's uninstall hook if declared in `plugin.yaml`.
3. Removes the directory recursively.
4. Subsequent `keramos <name>` invocations no longer find the plugin.

## Usage

```
keramos plugin remove <name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for remove |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Remove a plugin by name:

```sh
keramos plugin remove backup
```

Use one of the aliases:

```sh
keramos plugin rm backup
keramos plugin uninstall backup
```

Re-install after a clean removal:

```sh
keramos plugin remove  backup
keramos plugin install https://github.com/example/keramos-backup-plugin.git
```

## See also

- [`plugin`](plugin.md)
- [`plugin install`](plugin-install.md)
- [`plugin update`](plugin-update.md)
- [`plugin list`](plugin-list.md)
