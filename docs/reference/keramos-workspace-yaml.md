# `keramos-workspace.yaml` Reference

A workspace file groups multiple keramos packages into a single installable unit with dependency-aware ordering. The whole workspace can be installed, upgraded, diffed, or uninstalled in one command. Packages within a workspace remain **separate releases** — unlike `layers`, which compose into a single release. Use a workspace when you have ten related microservices to roll out together but each must be independently upgradeable; use layers when you have one application built from several reusable pieces.

The `keramos workspace` family of commands operates on this file.

---

## File layout

`keramos-workspace.yaml` lives at the root of the workspace directory. Its sibling subdirectories are typically the member package roots (or member paths can point anywhere accessible from the workspace root).

```
my-platform/
├── keramos-workspace.yaml
├── api/                      # member package
│   ├── keramos.yaml
│   ├── values.yaml
│   └── templates/
├── worker/                   # member package
│   ├── keramos.yaml
│   └── ...
└── postgres/
    ├── keramos.yaml
    └── ...
```

---

## Top-level fields

### `apiVersion` (string, required)

`keramos/v1`. Same versioning convention as `keramos.yaml`.

### `members` (list, required)

The packages in this workspace. Each entry is a `Member` (see schema below). Order is preserved for diagnostics and for level-0 ordering when no `dependsOn` constraints separate two members.

### `defaults` (object, optional)

Workspace-level defaults applied to each member that doesn't override the field. Allowed keys mirror the `Member` fields: `namespace`, `profile`, `atomic`, `wait`. Useful for repetitive workspaces (e.g. all members install into the same namespace).

```yaml
defaults:
  namespace: my-platform
  atomic: true
  wait: true
```

---

## `Member` schema

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Identifier used in CLI output, log lines, and `dependsOn` references. Must be unique within the workspace. |
| `path` | string | yes | Path to the member package, relative to the workspace root (or absolute). The directory must contain a `keramos.yaml`. |
| `namespace` | string | no | Kubernetes namespace where this member's release is installed. Falls back to `defaults.namespace`, then to the CLI `-n`. |
| `profile` | string | no | Profile name to activate when rendering this member (see *Profiles* in the package guide). |
| `dependsOn` | string list | no | Names of other members this member must wait for. Keramos computes a topological order; cycles are reported as errors. |
| `atomic` | bool | no | Roll back this member's install if it fails. Default: `true`. Set to `false` to leave the failed install in place for inspection. |
| `wait` | bool | no | Wait for this member's resources to become Ready before considering the install complete. Default: `true`. |

---

## Topology and parallelism

Keramos resolves members into **levels** using Kahn's algorithm:

- Level 0: members with no `dependsOn`.
- Level N: members whose every `dependsOn` target is at level < N.

Members within the same level are mutually independent and can be installed concurrently. The `keramos workspace` commands install level-by-level: every member of level N must be Ready (when `wait: true` and `--health-gate` is set) before level N+1 starts.

`keramos workspace plan --levels` prints the level grouping for review:

```
Level 0:
  - postgres
  - redis
Level 1:
  - api          (depends on postgres, redis)
  - worker       (depends on postgres)
Level 2:
  - frontend     (depends on api)
```

The `--parallel N` flag on workspace commands controls per-level concurrency.

---

## Atomic mode

When a workspace install fails partway through, `--atomic-workspace` instructs keramos to roll back every member it had successfully installed before the failure. Without it, the cluster keeps the partial state for debugging. Default is **non-atomic** for the workspace as a whole; per-member `atomic` controls whether an individual member's failed install rolls back its own resources.

---

## Health-gate

The `--health-gate` flag adds a between-level wait: after every member of level N has been issued, keramos waits for their pods to all become Ready before starting level N+1. Without `--health-gate`, level N+1 starts as soon as level N's apply calls return (the resources may not be Ready yet). Use `--health-gate` for hard ordering guarantees (database must be accepting connections before app starts), and skip it for faster rollouts when each member can tolerate its dependencies being slow to come up.

---

## Continue-on-error

`--continue-on-error` means a single member's failure does not abort the workspace; remaining members still get installed. Combine with `--atomic-workspace` for symmetric behaviour: each member self-rolls-back, the workspace continues. Default is to stop on first failure.

---

## Diff and status

- `keramos workspace diff` runs a per-member `keramos diff`, returning a unified report of what would change in any member.
- `keramos workspace status` queries each member's release record and reports current revision, status, and last-deployed time.

Both honour the same level grouping and `--parallel` concurrency.

---

## Example: tiered platform

```yaml
apiVersion: keramos/v1

defaults:
  namespace: platform
  atomic: true
  wait: true

members:
  # Level 0 — infrastructure
  - name: postgres
    path: ./postgres
    profile: ha-3node

  - name: redis
    path: ./redis

  # Level 1 — services
  - name: api
    path: ./api
    dependsOn: [postgres, redis]

  - name: worker
    path: ./worker
    dependsOn: [postgres]

  # Level 2 — edge
  - name: gateway
    path: ./gateway
    dependsOn: [api]
    namespace: platform-edge   # overrides default
```

Run:

```
keramos workspace install . --parallel 4 --health-gate --atomic-workspace
keramos workspace plan . --levels
keramos workspace diff .
keramos workspace status .
keramos workspace uninstall .          # reverse-order uninstall
```
