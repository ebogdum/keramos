# keramos uninstall

## Synopsis

`keramos uninstall` removes a release from the cluster. Pre-delete hooks fire, the release's resources are deleted (with cascade as defined by their owner references), post-delete hooks fire, and finally the release record is removed unless `--keep-history` is set.

## When to use it

Use this when a release is no longer needed. For test or experimental environments where you want to clean up everything keramos installed across the cluster, prefer `keramos purge`. To preserve the release record for audit while removing the cluster resources, the default behaviour already does that — pass `--purge` to also drop the history.

## What happens when you run it

1. Reads the current release record.
2. Marks the release status `uninstalling`.
3. Runs `pre-delete` hooks.
4. Deletes the release's resources from the cluster (relying on owner-references for cascading where applicable).
5. With the default `--wait`, blocks until resources are gone.
6. Runs `post-delete` hooks.
7. Marks the release status `uninstalled`. With `--purge`, also deletes the release record (Secrets) entirely; otherwise the record is kept for audit.

## Usage

```
keramos uninstall <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--description` | string | — | description recorded against the uninstall revision |
| `-h, --help` | — | — | help for uninstall |
| `--ignore-not-found` | — | — | exit zero when the release is not found |
| `--keep-history` | — | behaviour; explicit positive form | keep release history |
| `--no-hooks` | — | — | skip lifecycle hooks for this operation |
| `--no-wait` | — | — | do not wait for resource deletion |
| `-o, --output` | string | "table" | output format: table, json, yaml |
| `--purge` | — | — | delete release history (default: history is kept) |
| `--timeout` | duration | 5m0s | timeout for resource deletion |
| `--wait` | — | — | wait for resource deletion to complete (default) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Uninstall a release:

```sh
keramos uninstall my-app -n my-app-prod
```

Uninstall but keep the release record for audit:

```sh
keramos uninstall my-app --keep-history -n my-app-prod
```

Uninstall without running pre/post-delete hooks (skip when hooks themselves are broken):

```sh
keramos uninstall hello --no-hooks -n prod
```

Uninstall and also remove the release history record (cannot rollback after this):

```sh
keramos uninstall hello --purge -n prod
```

Idempotent uninstall in CI — exit 0 even if the release was already gone:

```sh
keramos uninstall hello --ignore-not-found -n prod
```

## See also

- [`purge`](purge.md)
- [`history`](history.md)
- [Hooks guide](../guides/hooks.md)
