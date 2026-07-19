# keramos workspace plan

## Synopsis

`keramos workspace plan` reads `keramos-workspace.yaml` and prints the order in which
`keramos workspace install` and `upgrade` will process the members. Nothing is
applied and the cluster is not contacted — it is a preview of the dependency
ordering. With `--levels` it groups the members by depth, so you can see which
ones are eligible to run in parallel.

## When to use it

- Before an install or upgrade, to confirm the dependency order is what you
  expect — especially after editing a `dependsOn` list.
- To see where parallelism kicks in: any level with more than one member can
  run those members concurrently under `--parallel`.
- To catch a dependency cycle early — a cycle stops the command with an error
  naming the members involved.

## What happens

1. Reads `keramos-workspace.yaml` from `--dir` (default `.`).
2. Builds the dependency graph from every member's `dependsOn` list.
3. Sorts the members so each one comes after everything it depends on.
4. Prints the order: a numbered flat list by default, or one group per level
   with `--levels`.

## Usage

```
keramos workspace plan [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dir` | string | `.` | Directory containing `keramos-workspace.yaml`. Point it elsewhere to plan a workspace in another directory. |
| `--levels` | bool | `false` | Group the output by dependency depth instead of a flat list, so members that can run in parallel appear together. |

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

**Run it:**

```sh
keramos workspace plan
```

**OUTPUT:**

```
1. postgres (path=./postgres, ns=apps, profile=)
2. api (path=./api, ns=apps, profile=)
```

`postgres` is listed first because `api` declares `dependsOn: [postgres]`, so it
must come up before `api`. Both inherit `ns=apps` from `defaults.namespace`, and
`profile=` is empty because neither member sets one.

**Group the same two members by level:**

```sh
keramos workspace plan --levels
```

**OUTPUT:**

```
level 0 (1 members, parallelisable):
  - postgres (path=./postgres, ns=apps)
level 1 (1 members, parallelisable):
  - api (path=./api, ns=apps)
```

`postgres` has no dependencies, so it sits alone at level 0; `api` waits for
level 0 to finish, so it lands at level 1. Add a second dependency-free package
and it would join `postgres` at level 0 and run alongside it under `--parallel`.

## See also

- [`workspace`](workspace.md) — the workspace index
- [`workspace install`](workspace-install.md), [`workspace upgrade`](workspace-upgrade.md)
  — run the members in this order
- [`plan`](plan.md) — the single-release analogue
