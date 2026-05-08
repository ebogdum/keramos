# keramos plugin update

## Synopsis

`keramos plugin update` (aliases: `up`, `upgrade`) refreshes an installed plugin against its source. For git-sourced plugins this means pulling new commits from the recorded URL and rebuilding; for local-path plugins it re-syncs from the source directory and re-runs any update hook. Re-running the plugin's own update hook is part of the contract — plugins use it to migrate config, recompile binaries, or apply schema changes.

## When to use it

Run periodically (or as part of CI) to keep plugins current. After updating a plugin, re-run `keramos <plugin> --help` to see any new subcommands or changed flags.

## What happens when you run it

1. Resolves `<name>` to a plugin directory under `~/.config/keramos/plugins/`.
2. Reads the recorded source from the plugin's metadata.
3. For git sources, runs `git fetch && git pull` (or equivalent). For local paths, re-syncs files.
4. Runs the plugin's update hook if declared.
5. Reports the new version on success.

## Usage

```
keramos plugin update <name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for update |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Update a single plugin:

```sh
keramos plugin update backup
```

Use the short alias:

```sh
keramos plugin up backup
```

Update every installed plugin (loop in shell):

```sh
for p in $(keramos plugin list -q 2>/dev/null || keramos plugin list | awk 'NR>1{print $1}'); do
  keramos plugin update "$p"
done
```

## See also

- [`plugin`](plugin.md)
- [`plugin install`](plugin-install.md)
- [`plugin list`](plugin-list.md)
- [`plugin remove`](plugin-remove.md)
