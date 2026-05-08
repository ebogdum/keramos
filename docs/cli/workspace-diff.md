# keramos workspace diff

## Synopsis

`keramos workspace diff` runs a `keramos diff` for every member declared in `keramos-workspace.yaml` and prints the combined output, grouped by member. Each member's section shows the per-resource patch that `keramos workspace upgrade` would apply. The command honours member-level dependsOn ordering for output but does not parallelise diffs.

## When to use it

Use as a workspace-wide change-detection gate before `keramos workspace upgrade` — particularly in CI, where seeing every member's pending changes in one report makes review tractable.

## What happens when you run it

1. Reads `<dir>/keramos-workspace.yaml` (default: current directory).
2. For each member, calls the same code path as `keramos diff`: renders the member, server-side dry-runs against the cluster, computes the structured per-resource diff.
3. Prints results grouped by member.
4. Cluster contact is read-only (server-side dry-run).

## Usage

```
keramos workspace diff [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dir` | string | . | directory containing `keramos-workspace.yaml` |
| `-h, --help` | bool | false | help for diff |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Diff every workspace member from the current directory:

```sh
keramos workspace diff
```

Diff a workspace at a specific path:

```sh
keramos workspace diff --dir ./platform
```

CI gate — fail when any member has changes:

```sh
keramos workspace diff > /tmp/diff.txt
[ -s /tmp/diff.txt ] && { cat /tmp/diff.txt; exit 1; }
```

## See also

- [`workspace`](workspace.md)
- [`workspace plan`](workspace-plan.md)
- [`workspace upgrade`](workspace-upgrade.md)
- [`diff`](diff.md) — single-release diff
- [`keramos-workspace.yaml` reference](../reference/keramos-workspace-yaml.md)
