# Workspaces

A workspace is a `keramos-workspace.yaml` file plus a tree of member packages. The workspace is installed, upgraded, diffed, or uninstalled by a single command, with keramos computing a topological order from `dependsOn` declarations and running members in parallel where the graph allows it. Each member is its own release — workspaces are not a way to compose templates (use `layers:` for that); they're a way to orchestrate many independently-installable packages from one repository.

The full file-format reference is in [`keramos-workspace.yaml`](../reference/keramos-workspace-yaml.md). This guide explains *how* to use it.

## When to reach for a workspace

You have a workspace if you have all of these:

- Multiple packages in **one repository** (a monorepo for an application platform).
- A **dependency graph** between them (postgres before app, app before edge).
- A desire to **install/upgrade/uninstall the whole graph** with one command.
- Each member is its own **independently-versioned release** (you can upgrade `api` without touching `postgres`).

If any of those isn't true, a workspace is overkill.

- One package, with reusable building blocks → use `layers:`.
- Many releases from many sources → use [`keramos-releases.yaml`](releases.md).
- One release backing a CR → use the KeramosRelease controller.

## Layout

```
my-platform/
├── keramos-workspace.yaml
├── postgres/
│   ├── keramos.yaml
│   ├── values.yaml
│   └── templates/
├── redis/
│   ├── keramos.yaml
│   └── ...
├── api/
│   └── ...
├── worker/
│   └── ...
└── gateway/
    └── ...
```

`keramos-workspace.yaml` at the root, members underneath. The members can be anywhere reachable from the workspace root — paths resolve relative to the file.

## A small workspace

```yaml
# keramos-workspace.yaml
apiVersion: keramos/v1

defaults:
  namespace: my-platform
  atomic: true
  wait: true

members:
  - name: postgres
    path: ./postgres
    profile: ha-3node

  - name: redis
    path: ./redis

  - name: api
    path: ./api
    dependsOn: [postgres, redis]

  - name: worker
    path: ./worker
    dependsOn: [postgres]

  - name: gateway
    path: ./gateway
    dependsOn: [api]
    namespace: my-platform-edge      # overrides default
```

## Topology and levels

Keramos resolves members into **levels** using Kahn's algorithm:

- Level 0: members with no `dependsOn`.
- Level N: members whose every `dependsOn` target is at a lower level.

Members within the same level are mutually independent and can be installed concurrently. The above workspace yields:

```
Level 0:
  - postgres
  - redis
Level 1:
  - api          (depends on postgres, redis)
  - worker       (depends on postgres)
Level 2:
  - gateway      (depends on api)
```

Inspect with:

```sh
keramos workspace plan . --levels
```

Output is the same level grouping; `keramos workspace plan . --json` prints a structured representation suitable for piping into other tooling.

## Install, upgrade, uninstall

```sh
keramos workspace install   .   # install all members in level order
keramos workspace upgrade   .   # upgrade existing, install missing
keramos workspace uninstall .   # reverse-level order: uninstall everything
keramos workspace plan      .   # what would happen — no changes applied
keramos workspace status    .   # per-member release status
keramos workspace diff      .   # per-member diff against current values
```

The `.` in each invocation is the workspace directory (the directory containing `keramos-workspace.yaml`).

## Parallelism

`--parallel N` controls per-level concurrency. Within a level, keramos runs up to N installs in parallel; the next level only starts when **every** member of the current level has finished.

```sh
keramos workspace install . --parallel 4
```

For the example above, with `--parallel 4`:

- Level 0 starts both `postgres` and `redis` in parallel.
- Level 1 waits for both to complete, then starts `api` and `worker` in parallel.
- Level 2 waits for those, then starts `gateway`.

## Health-gate

`--health-gate` adds a wait between levels: keramos doesn't start level N+1 until every member of level N has its workloads Ready (Deployment available, StatefulSet ready, etc.). Without `--health-gate`, level N+1 starts as soon as level N's apply calls return — Pods may still be coming up.

```sh
keramos workspace install . --parallel 4 --health-gate
```

Use `--health-gate` when:

- Members at level N+1 actually depend on level N being **functionally** ready (api can't start until postgres is accepting connections).
- Your environment has fast scheduling but slow image pulls (without the gate, the api would CrashLoopBackOff for several minutes before postgres's Pod is up).

Skip `--health-gate` when:

- Each member can tolerate its dependencies being slow to come up (Kubernetes-native retry semantics handle the temporary unavailability).
- You want the fastest possible rollout and don't mind brief restarts as dependencies catch up.

## Atomic-workspace

`--atomic-workspace` means: if any member fails, every member that previously succeeded in this run gets uninstalled. The workspace is "all or nothing".

```sh
keramos workspace install . --atomic-workspace
```

Pair with per-member `atomic: true` (the default) for symmetric behaviour: each member self-rolls-back its own resources on failure, AND the workspace as a whole rolls everything back.

Without `--atomic-workspace`, a partial workspace install leaves successful members deployed even if a later member fails. This is sometimes what you want — you can re-run the workspace install and it'll skip already-installed members.

## Continue-on-error

`--continue-on-error` means a single member's failure does not abort the workspace. Remaining members keep installing. Use with `--atomic-workspace` for "each member self-rolls-back, but we keep going" semantics.

```sh
keramos workspace install . --continue-on-error
```

The summary at the end lists which members succeeded and which failed. Exit code is non-zero if any failed.

## Progress

`--progress` toggles a verbose, level-by-level progress display. Without it, keramos prints a one-line summary per member as it finishes.

```sh
keramos workspace install . --progress
```

Output:

```
Level 0 (2 members) ────────────────────────────
  ✓ postgres        12.4s  → ready
  ✓ redis            8.1s  → ready
Level 1 (2 members) ────────────────────────────
  ✓ api              5.2s  → ready
  ✓ worker           4.7s  → ready
Level 2 (1 member)  ────────────────────────────
  ✓ gateway          3.1s  → ready

Workspace installed in 33.5s.
```

## Diff

`keramos workspace diff .` runs a per-member `keramos diff` against current state. Output groups by member:

```
api ─────────────────────
  Deployment/api
    spec.replicas: 2 → 3
    spec.template.spec.containers[0].image:
      registry/api:1.0.0 → registry/api:1.1.0

postgres ────────────────
  (no changes)
```

## Status

`keramos workspace status .` queries each member's release record:

```
NAME       NS              REV  STATUS    PACKAGE       VERSION   AGE
postgres   my-platform     7    deployed  postgres      3.0.4     14d
redis      my-platform     2    deployed  redis         2.4.1      8d
api        my-platform     12   deployed  api           1.1.0      4h
worker     my-platform     12   deployed  worker        1.1.0      4h
gateway    my-platform-edge 3   deployed  gateway       0.5.2      4h
```

## Per-member overrides

Most flags on `keramos install`/`keramos upgrade` have analogues on the workspace commands and can be customised per member via `keramos-workspace.yaml`:

```yaml
members:
  - name: api
    path: ./api
    namespace: api-prod         # overrides default ns for this member
    profile: prod               # selects api/profiles/prod.yaml
    atomic: false               # don't roll back this member on failure
    wait: false                 # don't wait for ready (good for non-deployment members like CronJobs)
```

## Shared values across members

Workspace commands don't expose `--set` / `-f` flags — each member is rendered against its own `values.yaml` plus its own selected profile or environment. To propagate a common value across members (a shared image registry, a shared domain, common labels), keep it in each member's `values.yaml` under the conventional `global` key:

```yaml
# api/values.yaml
global:
  imageRegistry: registry.example.com
  domain: example.com

# worker/values.yaml — same global block
global:
  imageRegistry: registry.example.com
  domain: example.com
```

Inside any layer's templates, `${values.global.imageRegistry}` resolves the same way regardless of where the layer is composed.

For environment-specific overrides (dev vs staging vs prod), use the `environments:` block in each member's `keramos.yaml` (see [Values guide](values.md)) and select with `--env` on a per-member basis (workspaces honour the member's own environment selection).
