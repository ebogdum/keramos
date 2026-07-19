---
title: "keramos workspace install"
parent: "CLI"
---
{% raw %}
# keramos workspace install

## Synopsis

`keramos workspace install` installs every member declared in
`keramos-workspace.yaml`, in dependency order. A member starts only after
everything it depends on has finished. Members that do not depend on each other
share a level and can run concurrently up to `--parallel`.

## When to use it

- To bring a whole workspace up from nothing in one command.
- When you want dependencies installed and ready before their dependents start
  — add `--health-gate` to wait for pods, not just for the apply to return.
- When a partial rollout is unacceptable — add `--atomic-workspace` to undo
  every successful member if any member fails.

## What happens

1. Reads `keramos-workspace.yaml` from `--dir` (default `.`) and sorts the members
   into dependency levels.
2. Installs level 0 first. Within a level, up to `--parallel` members install at
   once; the default of `1` installs them one at a time.
3. Waits for the whole level to finish before starting the next. With
   `--health-gate`, it also waits for every pod of that level to be Ready.
4. Advances level by level until every member is installed.
5. On failure, stops unless `--continue-on-error` is set; with
   `--atomic-workspace`, uninstalls every member that had already succeeded.

Each member is installed the same way [`keramos install`](install.md) would install
it, using the member's `namespace` and `profile`.

## Usage

```
keramos workspace install [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dir` | string | `.` | Directory containing `keramos-workspace.yaml`. Point it elsewhere to install a workspace in another directory. |
| `--parallel` | int | `1` | Maximum members to install at once within a level. Raise it to install independent members concurrently; `1` keeps them sequential. |
| `--health-gate` | bool | `false` | Between levels, wait until every pod of the finished level is Ready, not just applied — so dependents start against a serving dependency. |
| `--health-gate-timeout` | duration | `5m0s` | How long each level's health-gate waits before giving up. |
| `--continue-on-error` | bool | `false` | Keep installing the remaining members after one fails, then report all failures at the end. |
| `--atomic-workspace` | bool | `false` | If any member fails, uninstall every member that already succeeded. Mutually exclusive with `--continue-on-error`. |
| `--dry-run` | bool | `false` | Render every member locally and skip applying anything to the cluster. |
| `--progress` | bool | `false` | Print live lines as each member starts and finishes, plus a final summary. |

Inherits the global flags.

## Worked example

**INPUT — `keramos-workspace.yaml` with two members**, where `api` depends on
`postgres`:

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
keramos workspace install --progress
```

**OUTPUT:**

```
workspace: 2 members across 2 level(s), parallel=1, op=install

[level 0/1] 1 member(s) starting concurrently
  → postgres (ns=apps) start
  ✓ postgres done in 4.1s

[level 1/1] 1 member(s) starting concurrently
  → api (ns=apps) start
  ✓ api done in 2.7s

All 2 member(s) succeeded.
```

`postgres` is the only member at level 0, so it installs first; `api` waits at
level 1 until `postgres` finishes, matching its `dependsOn: [postgres]`. Both
land in `ns=apps` from `defaults.namespace`. Without `--progress`, a fully
successful run prints nothing; failures are always reported.

## See also

- [`workspace`](workspace.md) — the workspace index
- [`workspace plan`](workspace-plan.md) — preview the order first
- [`workspace uninstall`](workspace-uninstall.md) — tear the workspace back down
- [`install`](install.md) — the single-release analogue
{% endraw %}
