# keramos plugin list

## Synopsis

`keramos plugin list` (alias `keramos plugin ls`) prints every plugin currently installed on this machine: name, version, source (git URL or local path), and brief description. Plugins live under `~/.config/keramos/plugins/`; this command reads that directory.

## When to use it

Use to inventory the local plugin set, find a plugin's name for `keramos plugin remove` / `update`, or verify an install succeeded.

## What happens when you run it

1. Reads `~/.config/keramos/plugins/` (or `${KERAMOS_CONFIG_HOME}/plugins/`).
2. Parses each subdirectory's `plugin.yaml`.
3. Prints a tabular view to stdout.
4. No cluster contact, no network.

## Usage

```
keramos plugin list [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for list |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

List installed plugins:

```sh
keramos plugin list
```

Use the short alias:

```sh
keramos plugin ls
```

Find one specific plugin's source:

```sh
keramos plugin list | grep backup
```

## See also

- [`plugin`](plugin.md)
- [`plugin install`](plugin-install.md)
- [`plugin update`](plugin-update.md)
- [`plugin remove`](plugin-remove.md)
