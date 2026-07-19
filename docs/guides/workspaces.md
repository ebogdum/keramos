---
title: "Manage a workspace of packages"
parent: "Guides"
---
{% raw %}
# Manage a workspace of packages

A workspace is a `keramos-workspace.yaml` file plus a tree of member packages.
One command installs, upgrades, diffs, or uninstalls the whole set, with keramos
computing a topological order from `dependsOn` and running independent members
in parallel. Each member is its own release — a workspace orchestrates many
independently-installable packages from one repository. (To compose one release
from reusable pieces, use `layers:` instead; see [Layers](layers.md).)

The file-format reference is
[`keramos-workspace.yaml`](../reference/keramos-workspace-yaml.md); the command
reference is [`keramos workspace`](../cli/workspace.md). This guide covers the
workflow.

## When to reach for a workspace

You want a workspace when all of these hold:

- Multiple packages in **one repository**.
- A **dependency graph** between them (postgres before api, api before gateway).
- A wish to **install/upgrade/uninstall the whole graph** with one command.
- Each member is its own **independently-versioned release**.

If any of those is false, a workspace is overkill:

- One release built from reusable blocks → use `layers:`.
- Many releases from many sources → use [`keramos-releases.yaml`](releases.md).

## Lay out the workspace

```
my-platform/
├── keramos-workspace.yaml
├── postgres/
│   ├── keramos.yaml
│   ├── values.yaml
│   └── templates/
├── redis/
│   └── ...
├── api/
│   └── ...
├── worker/
│   └── ...
└── gateway/
    └── ...
```

`keramos-workspace.yaml` sits at the root; member paths resolve relative to it.

## Write the workspace file

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
    namespace: my-platform-edge      # overrides the default
```

Each member has a `name`, a `path`, and optionally `namespace`, `profile`,
`dependsOn`, `atomic`, and `wait`. The `defaults` block sets `namespace`,
`profile`, `atomic`, and `wait` for any member that does not override them.

## Preview the order

Every `keramos workspace` command reads `keramos-workspace.yaml` from the directory
given by `--dir` (default the current directory) — there is no positional path
argument.

```sh
keramos workspace plan
```

```
1. postgres (path=./postgres, ns=my-platform, profile=ha-3node)
2. redis (path=./redis, ns=my-platform, profile=)
3. api (path=./api, ns=my-platform, profile=)
4. worker (path=./worker, ns=my-platform, profile=)
5. gateway (path=./gateway, ns=my-platform-edge, profile=)
```

Add `--levels` to group members by dependency depth — members in the same
level are mutually independent and can run in parallel:

```sh
keramos workspace plan --levels
```

```
level 0 (2 members, parallelisable):
  - postgres (path=./postgres, ns=my-platform)
  - redis (path=./redis, ns=my-platform)
level 1 (2 members, parallelisable):
  - api (path=./api, ns=my-platform)
  - worker (path=./worker, ns=my-platform)
level 2 (1 members, parallelisable):
  - gateway (path=./gateway, ns=my-platform-edge)
```

Level 0 holds members with no `dependsOn`; a member lands at level N when every
member it depends on sits at a lower level. To plan a workspace in another
directory, point `--dir` at it:

```sh
keramos workspace plan --dir ./my-platform --levels
```

## Install, upgrade, uninstall

```sh
keramos workspace install     # install all members in dependency order
keramos workspace upgrade     # upgrade existing, install missing
keramos workspace uninstall   # reverse order: dependents down first
keramos workspace status      # per-member revision and status
keramos workspace diff        # per-member preview of pending changes
```

## Run members in parallel

`--parallel N` sets the per-level concurrency (default `1`, i.e. sequential).
Within a level, up to N members run at once; the next level starts only when
**every** member of the current one has finished.

```sh
keramos workspace install --parallel 4 --progress
```

```
workspace: 5 members across 3 level(s), parallel=4, op=install

[level 0/2] 2 member(s) starting concurrently
  → postgres (ns=my-platform) start
  → redis (ns=my-platform) start
  ✓ redis done in 8.1s
  ✓ postgres done in 12.4s

[level 1/2] 2 member(s) starting concurrently
  → api (ns=my-platform) start
  → worker (ns=my-platform) start
  ✓ worker done in 4.7s
  ✓ api done in 5.2s

[level 2/2] 1 member(s) starting concurrently
  → gateway (ns=my-platform-edge) start
  ✓ gateway done in 3.1s

All 5 member(s) succeeded.
```

`--progress` prints these live lines. Without it, a fully successful run prints
nothing; failures are always reported.

## Gate on readiness between levels

By default a level advances as soon as its apply calls return — pods may still
be starting. `--health-gate` makes keramos wait until every pod owned by a level
is Ready before starting the next level, so dependents start against a
dependency that is actually serving.

```sh
keramos workspace install --parallel 4 --health-gate
```

Use it when a level genuinely depends on the previous one being functionally up
(api cannot start until postgres accepts connections). Skip it when each member
tolerates its dependencies being slow to come up, for a faster rollout.
`--health-gate-timeout` (default `5m0s`) caps each level's wait.

## Control failure handling

`--atomic-workspace` makes the run all-or-nothing: if any member fails, every
member that already succeeded in this run is uninstalled.

```sh
keramos workspace install --atomic-workspace
```

`--continue-on-error` does the opposite — a member's failure does not abort the
run; the rest keep going and all failures are reported at the end.

```sh
keramos workspace install --continue-on-error
```

The two are **mutually exclusive**. Pair per-member `atomic: true` (the
default) with `--atomic-workspace` for symmetric behaviour: each member rolls
back its own resources on failure, and the workspace rolls back the whole set.

## Preview changes before upgrading

```sh
keramos workspace diff
```

```
=== postgres (ns=my-platform) ===
  no changes

=== api (ns=my-platform) ===
~ update  Deployment/api  (namespace my-platform)
      ~ spec.replicas
          - 2   (state)
          + 3

Summary: 0 added, 1 changed, 0 removed.
```

`diff` renders each member and compares it against the state keramos recorded at
its last apply, grouped by member in dependency order. Nothing is applied.

## Check status

```sh
keramos workspace status
```

```
MEMBER                         NAMESPACE            REVISION   STATUS
postgres                       my-platform          7          deployed
redis                          my-platform          2          deployed
api                            my-platform          12         deployed
worker                         my-platform          12         deployed
gateway                        my-platform-edge     -          not deployed
```

A member with no release shows `-` and `not deployed`.

## Override settings per member

Most per-release settings can be customised on the member entry:

```yaml
members:
  - name: worker
    path: ./worker
    namespace: worker-prod      # overrides the default namespace
    profile: prod               # selects worker's prod profile
    atomic: false               # leave a failed install in place to inspect
    wait: false                 # don't wait for Ready (e.g. CronJob members)
```

## Share values across members

Workspace commands do not expose `-f`/`--set`: each member renders against its
own `values.yaml` plus its selected `profile`. To propagate a common value —
a shared registry, a shared domain — keep it under the conventional `global`
key in each member's `values.yaml`:

```yaml
# api/values.yaml
global:
  imageRegistry: registry.example.com
  domain: example.com
```

Templates reference it the same way regardless of where the member sits:
`${values.global.imageRegistry}`.

## See also

- [`keramos workspace`](../cli/workspace.md) — command reference
- [`keramos-workspace.yaml`](../reference/keramos-workspace-yaml.md) — file-format
  reference
- [Cross-release dependencies](releases.md) — orchestrate releases from many
  sources
- [Layers](layers.md) — compose one release from reusable pieces
- [Values](values.md) — profiles and environments
{% endraw %}
