# keramos repo

## Synopsis

`keramos repo` manages HTTP package repositories. Subcommands add, remove, and list repositories; refresh local index caches; and generate or update an `index.yaml` for a directory of packaged archives (the publication side).

## When to use it

Use to manage your local list of upstream repositories and to generate index files for repositories you publish.

## Usage

```
keramos repo [command]
```

## Subcommands

- [`keramos repo add`](repo-add.md) — register a repository
- [`keramos repo remove`](repo-remove.md) — unregister a repository
- [`keramos repo list`](repo-list.md) — list registered repositories
- [`keramos repo update`](repo-update.md) — refresh local index caches
- [`keramos repo index`](repo-index.md) — generate or update an `index.yaml` for a directory of archives

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for repo |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Add a repo:

```sh
keramos repo add my-charts https://charts.example.com
```

Refresh the local index cache for every added repo:

```sh
keramos repo update
```

Generate an index for a directory you're publishing:

```sh
keramos repo index ./build --url https://charts.example.com
```

## See also

- [`pull`](pull.md)
- [`search`](search.md)
- [Repositories guide](../guides/repositories.md)
