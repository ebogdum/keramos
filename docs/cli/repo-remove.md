# keramos repo remove

## Synopsis

`keramos repo remove` deletes the named repository from keramos's configuration: the entry is removed from `~/.config/keramos/repositories.yaml`, the cached `index.yaml` under `~/.cache/keramos/indexes/` is purged, and any stored credentials in `~/.config/keramos/credentials.json` keyed by the repo's URL are cleared. The repository server itself is unaffected — only this machine's view of it.

## When to use it

Use when a repo is no longer needed, when you want to re-add a repo with different credentials (alternative to `--force-update`), or when cleaning up a development environment.

## What happens when you run it

1. Looks up `<name>` in `~/.config/keramos/repositories.yaml`.
2. Removes its entry.
3. Removes its index cache file.
4. Removes its credentials from `~/.config/keramos/credentials.json`.
5. No network access.

## Usage

```
keramos repo remove <name> [flags]
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

Remove a repo:

```sh
keramos repo remove my-charts
```

Remove and re-add to update credentials:

```sh
keramos repo remove my-charts
keramos repo add    my-charts https://charts.example.com --username new-user --password new-pass
```

## See also

- [`repo`](repo.md)
- [`repo add`](repo-add.md)
- [`repo list`](repo-list.md)
- [`logout`](logout.md) — remove credentials without unregistering the repo
