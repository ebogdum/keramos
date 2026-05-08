# keramos workspace

## Synopsis

`keramos workspace` orchestrates multiple keramos packages declared in `keramos-workspace.yaml`. Subcommands install, upgrade, uninstall, plan, diff, and report status across the whole workspace using a topological order computed from `dependsOn` declarations.

## When to use it

Use when many sibling packages from one repository should roll out together with explicit dependency ordering between them. For releases sourced from disparate places (different registries, paths, repos), see `keramos releases`.

## Usage

```
keramos workspace [command]
```

## Subcommands

- [`keramos workspace install`](workspace-install.md) — install every member in topological order
- [`keramos workspace upgrade`](workspace-upgrade.md) — upgrade every member; install if missing
- [`keramos workspace uninstall`](workspace-uninstall.md) — uninstall every member in reverse topological order
- [`keramos workspace plan`](workspace-plan.md) — print the install plan with optional level grouping
- [`keramos workspace status`](workspace-status.md) — show current revision and status of every declared member
- [`keramos workspace diff`](workspace-diff.md) — show pending changes per member

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for workspace |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Plan a workspace install:

```sh
keramos workspace plan .
```

Install with parallelism within levels and a health-gate between levels:

```sh
keramos workspace install . --parallel 4 --health-gate
```

Diff every member's pending changes:

```sh
keramos workspace diff .
```

## See also

- [`keramos-workspace.yaml` reference](../reference/keramos-workspace-yaml.md)
- [Workspaces guide](../guides/workspaces.md)
- [`releases`](releases.md)
