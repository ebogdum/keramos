---
title: "keramos workspace upgrade"
parent: "CLI"
---
{% raw %}
# keramos workspace upgrade

## Synopsis

`keramos workspace upgrade` upgrades every member declared in
`keramos-workspace.yaml`, in dependency order, and installs any member that is not
yet deployed. It is the command to run every time you roll out a new version of
the whole workspace — safe whether a member already exists or not.

## When to use it

- As the routine deploy command for a workspace: it upgrades what is there and
  installs what is missing.
- When dependents must not roll before their dependencies are healthy — add
  `--health-gate`.
- When you want the whole batch to move together — add `--atomic-workspace` to
  roll back every member if any one fails.

## What happens

1. Reads `keramos-workspace.yaml` from `--dir` (default `.`) and sorts the members
   into dependency levels.
2. Processes level 0 first. Within a level, up to `--parallel` members run at
   once; the default of `1` runs them one at a time.
3. For each member, upgrades it — or installs it if it has no current release.
4. Waits for the level to finish before advancing. With `--health-gate`, also
   waits for every pod of that level to be Ready.
5. On failure, stops unless `--continue-on-error` is set; with
   `--atomic-workspace`, uninstalls every member that had already succeeded.

Each member is upgraded the same way [`keramos upgrade`](upgrade.md) would, using
the member's `namespace` and `profile`.

## Usage

```
keramos workspace upgrade [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dir` | string | `.` | Directory containing `keramos-workspace.yaml`. Point it elsewhere to upgrade a workspace in another directory. |
| `--parallel` | int | `1` | Maximum members to process at once within a level. Raise it to upgrade independent members concurrently; `1` keeps them sequential. |
| `--health-gate` | bool | `false` | Between levels, wait until every pod of the finished level is Ready, not just applied — so dependents upgrade against a serving dependency. |
| `--health-gate-timeout` | duration | `5m0s` | How long each level's health-gate waits before giving up. |
| `--continue-on-error` | bool | `false` | Keep processing the remaining members after one fails, then report all failures at the end. |
| `--atomic-workspace` | bool | `false` | If any member fails, uninstall every member that already succeeded. Mutually exclusive with `--continue-on-error`. |
| `--dry-run` | bool | `false` | Render every member locally and skip applying anything to the cluster. |
| `--progress` | bool | `false` | Print live lines as each member starts and finishes, plus a final summary. |

Inherits the global flags.

## Worked example

**INPUT — `keramos-workspace.yaml` with two members**, where `api` depends on
`postgres`. `postgres` is already deployed; `api` is not yet installed:

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

**Run it with live progress:**

```sh
keramos workspace upgrade --progress
```

**OUTPUT:**

```
workspace: 2 members across 2 level(s), parallel=1, op=upgrade

[level 0/1] 1 member(s) starting concurrently
  → postgres (ns=apps) start
  ✓ postgres done in 3.4s

[level 1/1] 1 member(s) starting concurrently
  → api (ns=apps) start
  ✓ api done in 2.9s

All 2 member(s) succeeded.
```

`postgres` upgrades first at level 0. `api` waits at level 1 for `postgres` to
finish, matching its `dependsOn: [postgres]`; because `api` had no release yet,
the upgrade installs it. Without `--progress`, a fully successful run prints
nothing; failures are always reported.

## See also

- [`workspace`](workspace.md) — the workspace index
- [`workspace plan`](workspace-plan.md) — preview the order first
- [`workspace diff`](workspace-diff.md) — see what the upgrade would change
- [`upgrade`](upgrade.md) — the single-release analogue
{% endraw %}
