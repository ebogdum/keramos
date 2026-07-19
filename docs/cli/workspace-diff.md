---
title: "keramos workspace diff"
parent: "CLI"
---
{% raw %}
# keramos workspace diff

## Synopsis

`keramos workspace diff` renders every member declared in `keramos-workspace.yaml` and
compares each one against its own recorded state — what `keramos` stored the last
time that member was applied. It prints, grouped by member, the per-resource
changes that a `keramos workspace upgrade` would make. Nothing is applied.

## When to use it

- Before an upgrade, to review every member's pending changes in one report.
- As a CI change-detection gate: render the workspace, see what would move, and
  decide whether to proceed.

## What happens

1. Reads `keramos-workspace.yaml` from `--dir` (default `.`) and orders the members
   by dependency.
2. Renders each member locally and reads its stored state from the member's
   namespace (best-effort — with no reachable cluster or no prior state, every
   resource shows as a create).
3. Prints a `=== <member> (ns=<namespace>) ===` header per member, then either
   `no changes` or the per-resource change preview.
4. Ends each changed member with a `Summary: N added, N changed, N removed.`
   line. No resources are modified.

## Usage

```
keramos workspace diff [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dir` | string | `.` | Directory containing `keramos-workspace.yaml`. Point it elsewhere to diff a workspace in another directory. |

Inherits the global flags.

## Worked example

**INPUT — `keramos-workspace.yaml` with two members**, where `api` depends on
`postgres`. `postgres` matches its recorded state; `api`'s package now sets
`replicas: 3` where the stored state has `1`:

```yaml
apiVersion: keramos/v1
defaults:
  namespace: apps
members:
  - name: postgres
    path: ./postgres      # unchanged since last apply
  - name: api
    path: ./api           # replicas edited 1 → 3
    dependsOn: [postgres]
```

**Run it:**

```sh
keramos workspace diff
```

**OUTPUT:**

```
=== postgres (ns=apps) ===
  no changes

=== api (ns=apps) ===
~ update  Deployment/api  (namespace apps)
      ~ spec.replicas
          - 1   (state)
          + 3

Summary: 0 added, 1 changed, 0 removed.
```

`postgres` renders identically to its stored state, so its section reads
`no changes`. `api` renders with `replicas: 3`; the `- 1 (state)` line is the
value `keramos` recorded, and `+ 3` is what the package now produces — the one
change a `keramos workspace upgrade` would apply. Members are shown in dependency
order, `postgres` before `api`.

## See also

- [`workspace`](workspace.md) — the workspace index
- [`workspace plan`](workspace-plan.md) — preview the order of members
- [`workspace upgrade`](workspace-upgrade.md) — apply the changes shown here
- [`diff`](diff.md) — the single-release analogue
{% endraw %}
