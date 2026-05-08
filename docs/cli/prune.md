# keramos prune

## Synopsis

`keramos prune` deletes superseded revision records, keeping only the most recent N. The current `deployed` revision is always preserved regardless of N. Pruning frees etcd space; it does not affect cluster resources, only the historical record stored in release Secrets.

## When to use it

Use periodically on long-lived releases that have accumulated dozens or hundreds of revisions. The default `--keep` retains 10. Set higher for releases under heavy CD churn where you need a longer rewind window.

## Usage

```
keramos prune [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dry-run` | — | — | list revisions that would be deleted without deleting them |
| `-h, --help` | — | — | help for prune |
| `--keep` | int | 10 | number of recent revisions to retain per release |
| `--release` | string | — | prune a single release; empty means every release in the namespace |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Keep only the 10 most recent revisions of a single release:

```sh
keramos prune --release my-app --keep 10 -n prod
```

Dry-run first to see what would be removed:

```sh
keramos prune --release my-app --keep 10 --dry-run -n prod
```

Prune every release in the namespace to 5 revisions each:

```sh
keramos prune --keep 5 -n prod
```

## See also

- [`history`](history.md)
