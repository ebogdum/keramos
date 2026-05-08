# keramos workspace install

## Synopsis

Install every member of the workspace in topological order. Members within a level run in parallel up to `--parallel`. With `--health-gate`, level N+1 starts only after every member of level N is Ready.

## When to use it

Use to bring up a workspace fresh.

## Usage

```
keramos workspace install [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--atomic-workspace` | — | — | if any member fails, roll back every successful one (mutually exclusive with --continue-on-error) |
| `--continue-on-error` | — | — | keep processing remaining members past a failed one; report all failures at the end |
| `--dir` | string | "." | directory containing keramos-workspace.yaml |
| `--dry-run` | — | — | render every member with client-side dry-run; do not apply to the cluster |
| `--health-gate` | — | — | between levels, wait for ALL pods of every member in the level to be Ready (not just --wait) |
| `--health-gate-timeout` | duration | 5m0s | per-level health-gate wait |
| `-h, --help` | — | — | help for install |
| `--parallel` | int | 1 | max members to process concurrently within a topological level (1 = sequential) |
| `--progress` | — | — | print live progress lines as members complete |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Workspace install:

```sh
keramos workspace install . --parallel 4 --health-gate
```

## See also

- [`workspace`](workspace.md)
