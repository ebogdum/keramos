# keramos search repo

## Synopsis

`keramos search repo` searches the locally-cached `index.yaml` of every registered HTTP repository for packages whose name or description matches the keyword. Results include the source repo, package name, latest version, and short description. The freshness of the result depends on how recently you ran `keramos repo update` — without a recent update, the search is against potentially stale data.

## When to use it

Use to find packages across your trusted, registered repositories without going to the public Artifact Hub. Pair with `keramos repo update` first if you want the most current view.

## What happens when you run it

1. Reads cached indexes from `~/.cache/keramos/indexes/`.
2. Substring-matches the keyword against package names and descriptions.
3. Prints a tabular view to stdout.
4. No cluster contact, no network access (the index has already been cached).

## Usage

```
keramos search repo <keyword> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for repo |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Refresh first, then search:

```sh
keramos repo update
keramos search repo postgres
```

Search without refreshing (uses cached indexes — may be stale):

```sh
keramos search repo postgres
```

Find every package across registered repos with `redis` in the name:

```sh
keramos search repo redis
```

## See also

- [`search`](search.md)
- [`search hub`](search-hub.md) — search Artifact Hub instead
- [`repo update`](repo-update.md) — refresh index caches
- [`pull`](pull.md)
