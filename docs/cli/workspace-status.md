# keramos workspace status

## Synopsis

`keramos workspace status` queries the cluster for the current state of every member declared in `keramos-workspace.yaml`. For each member, it reports the current revision, status, package, namespace, and last-deployed time — the same data `keramos status <release>` produces for individual releases, in one tabular view.

## When to use it

Use after `keramos workspace install` / `upgrade` to confirm every member converged, or as a routine health check on a workspace-managed platform. Members declared but not installed show as `not installed`.

## What happens when you run it

1. Reads `<dir>/keramos-workspace.yaml` (default: current directory).
2. For each member, queries the cluster for the corresponding release record (in the member's namespace).
3. Composes a tabular view with revision, status, package, and last-deployed timestamp.
4. Prints to stdout. Exits 0 if all are deployed; non-zero if any are missing or failed.

## Usage

```
keramos workspace status [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dir` | string | . | directory containing `keramos-workspace.yaml` |
| `-h, --help` | bool | false | help for status |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Status of every member in the current-directory workspace:

```sh
keramos workspace status
```

Status of a workspace at a custom path:

```sh
keramos workspace status --dir ./platform
```

CI health check — fail when a member is missing or failed:

```sh
keramos workspace status || { echo "workspace not fully up"; exit 1; }
```

## See also

- [`workspace`](workspace.md)
- [`workspace install`](workspace-install.md)
- [`workspace upgrade`](workspace-upgrade.md)
- [`status`](status.md) — single-release status
- [`keramos-workspace.yaml` reference](../reference/keramos-workspace-yaml.md)
