# keramos workspace uninstall

## Synopsis

Uninstall every workspace member in reverse topological order.

## When to use it

Use when tearing down a managed workspace cleanly.

## Usage

```
keramos workspace uninstall [flags]
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
| `-h, --help` | — | — | help for uninstall |
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

Workspace uninstall:

```sh
keramos workspace uninstall .
```

## See also

- [`workspace`](workspace.md)
