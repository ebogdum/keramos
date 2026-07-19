---
title: "keramos workspace status"
parent: "CLI"
---
{% raw %}
# keramos workspace status

## Synopsis

`keramos workspace status` reads `keramos-workspace.yaml`, looks up each member in the
cluster, and prints one table row per member showing its namespace, current
revision, and status. It is the quickest way to see whether every part of a
workspace is deployed and at which revision.

## When to use it

- After an install or upgrade, to confirm every member converged.
- As a routine health check on a workspace-managed platform.
- To spot members that are declared but not yet deployed — they show as
  `not deployed`.

## What happens

1. Reads `keramos-workspace.yaml` from `--dir` (default `.`) and orders the members
   by dependency.
2. For each member, queries its namespace for the current release.
3. Prints a table: member name, namespace, revision, and status.
4. A member with no release shows `-` and `not deployed`; if its namespace
   cannot be reached, the row shows `(client error)`.

## Usage

```
keramos workspace status [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dir` | string | `.` | Directory containing `keramos-workspace.yaml`. Point it elsewhere to check a workspace in another directory. |

Inherits the global flags.

## Worked example

**INPUT — `keramos-workspace.yaml` with two members**, where `api` depends on
`postgres`. `postgres` is deployed; `api` has not been installed yet:

```yaml
apiVersion: keramos/v1
defaults:
  namespace: apps
members:
  - name: postgres
    path: ./postgres
  - name: api
    path: ./api
    dependsOn: [postgres]
```

**Run it:**

```sh
keramos workspace status
```

**OUTPUT:**

```
MEMBER                         NAMESPACE            REVISION   STATUS
postgres                       apps                 3          deployed
api                            apps                 -          not deployed
```

`postgres` is listed first (dependencies before dependents) and reports
revision `3`, `deployed`. `api` has no release, so its revision is `-` and its
status is `not deployed`. Both rows show `apps`, inherited from
`defaults.namespace`.

## See also

- [`workspace`](workspace.md) — the workspace index
- [`workspace install`](workspace-install.md), [`workspace upgrade`](workspace-upgrade.md)
  — bring members up to a deployed state
- [`status`](status.md) — the single-release analogue
{% endraw %}
