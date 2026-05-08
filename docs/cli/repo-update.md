# keramos repo update

## Synopsis

`keramos repo update` re-fetches the `index.yaml` for every registered repository and writes the result into the local index cache (`~/.cache/keramos/indexes/<name>.yaml`). After this command, `keramos search` and dependency-resolution paths see the freshest list of available packages. By default, an unreachable repo is logged but does not fail the whole operation; with `--fail-on-repo-update-fail`, any failure produces a non-zero exit.

## When to use it

Run periodically (and always immediately before `keramos search` or `keramos pull` for a precise version pick) to refresh the local view of upstream catalogues.

## What happens when you run it

1. Reads `~/.config/keramos/repositories.yaml`.
2. For each registered repo, performs `GET <url>/index.yaml` with the stored credentials and TLS material.
3. Writes the response to `~/.cache/keramos/indexes/<name>.yaml`.
4. Reports per-repo success or failure on stdout.

## Usage

```
keramos repo update [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--fail-on-repo-update-fail` | bool | false | exit non-zero if any repository update fails |
| `-h, --help` | bool | false | help for update |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Refresh every registered repo's index:

```sh
keramos repo update
```

In CI, fail the build if any repo can't be refreshed:

```sh
keramos repo update --fail-on-repo-update-fail
```

Refresh, then search across the freshly-updated catalogues:

```sh
keramos repo update
keramos search repo redis
```

## See also

- [`repo`](repo.md)
- [`repo list`](repo-list.md)
- [`search repo`](search-repo.md)
- [Repositories guide](../guides/repositories.md)
