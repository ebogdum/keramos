# Layers

A layer is another keramos package whose templates and values are composed into the current package, producing a single rendered manifest. Layers are how you build small reusable pieces (a "Postgres-with-PVC" layer, a "monitoring sidecar" layer, a "common labels" layer) and combine them to form a larger application without copy-pasting YAML between packages.

This guide explains when to use a layer (versus a `requires` dependency or a separate workspace member), how composition works, and the patterns for conditional and tag-based inclusion.

## Layer vs. requires vs. workspace member

| Construct | Result | Use when |
|---|---|---|
| **Layer** (`layers:` in `keramos.yaml`) | Templates and values compose into **one** release. | The pieces are not separately useful — they always ship together. |
| **Requires** (`requires:` in `keramos.yaml`) | Each entry is its own **separate** release; the parent's install asserts they are already installed. | Loosely-coupled releases that have their own lifecycle. |
| **Workspace member** (`members:` in `keramos-workspace.yaml`) | Each member is a separate release; the workspace orchestrates install/upgrade/uninstall. | The pieces are independently developed but rolled out as a set. |

Same `LayerSource` schema for `layers:` and `requires:` (see [`keramos.yaml` reference](../reference/keramos-yaml.md)). The semantic difference is install-time only.

## Composition mechanics

When keramos installs a package with layers:

1. **Resolve.** For each layer, fetch the source (local path, OCI, HTTPS, git) into the package's local cache (`./.keramos/layers/`).
2. **Merge values.** Layer values are merged in declared order, then the parent package's `values.yaml` overrides them.
3. **Merge templates.** Every YAML in `<layer>/templates/` joins the render set. The parent package's templates override on filename collision (so the parent can replace a layer's `service.yaml` by shipping its own `service.yaml`).
4. **Merge hooks.** Same rules as templates.
5. **Merge CRDs.** Every YAML in `<layer>/crds/` is added to the apply-first set. CRD collisions across layers are an error (it's never right to silently shadow a CRD).
6. **Merge `files/`.** Layer files are mounted at `<layer-name>/<path>` in the parent's `.Files` namespace; the parent's own `files/` is at the top level.

Result: a single rendered manifest, one release record, one set of hooks.

## A small example

```
shared-base/
├── keramos.yaml
├── values.yaml
└── templates/
    ├── _labels.yaml         # partial — common label set
    └── serviceaccount.yaml

my-app/
├── keramos.yaml                # has layers: [shared-base]
├── values.yaml
└── templates/
    ├── deployment.yaml      # uses ${include "_labels.yaml" .}
    └── service.yaml
```

`shared-base/templates/_labels.yaml` ships a partial that the layer-aware parent can `${include}` from. The ServiceAccount template at `shared-base/templates/serviceaccount.yaml` is a normal manifest — it will be applied as part of the `my-app` release.

## Source schemes

The `source` field accepts four prefixes; keramos picks the handler from the prefix.

### Local

```yaml
layers:
  - name: shared-base
    source: ../shared-base
  - name: sidecar
    source: ./layers/sidecar
```

Resolved relative to the parent's `keramos.yaml`. Useful in monorepos.

### HTTPS

```yaml
layers:
  - name: my-layer
    source: https://charts.example.com/my-layer-1.2.3.tgz
```

Keramos downloads the archive and verifies its checksum. If the URL is a repo index, the `version` field is interpreted as a constraint and resolved against the index.

### Git

```yaml
layers:
  - name: my-layer
    source: git::https://github.com/example/repo.git
    ref: v1.2.3            # branch, tag, or commit SHA
  - name: ssh-layer
    source: git::ssh://git@github.com/example/repo.git
    ref: main
  - name: subpath
    source: git::https://github.com/example/repo.git//path/to/package
    ref: v1.2.3
```

Keramos does a shallow clone of the named ref. The double-slash `//path/to/package` syntax selects a subdirectory of the repository as the package root. Authentication uses your local SSH agent / git credentials helper as configured.

### OCI

```yaml
layers:
  - name: my-layer
    source: oci://ghcr.io/example/charts/my-layer
    version: ^1.2.0
```

Keramos pulls via OCI distribution-spec API. Auth comes from `keramos login` / `keramos registry login` (stored in `~/.config/keramos/credentials.json`). Environment knobs:

- `KERAMOS_OCI_PLAIN_HTTP=true` — use HTTP instead of HTTPS (for local registries).
- `KERAMOS_OCI_INSECURE_SKIP_TLS=true` — skip TLS verification (for self-signed registries).

`version` for OCI is a SemVer constraint; keramos lists the registry's available tags and resolves to the highest matching version.

## Conditional layers

The `condition` field gates a layer on a values lookup. The path is dotted; the layer is enabled when the resolved value is truthy (`true`, non-empty string, non-zero number, non-empty list/map).

```yaml
# keramos.yaml
layers:
  - name: redis
    source: oci://ghcr.io/example/redis-layer
    version: ^2.0.0
    condition: redis.enabled

  - name: postgres
    source: ../layers/postgres
    condition: database.engine
```

```yaml
# values.yaml — redis included, postgres excluded
redis:
  enabled: true
database:
  engine: ""           # falsy → postgres layer skipped
```

```yaml
# values.yaml — both included
redis:
  enabled: true
database:
  engine: postgres     # truthy string → postgres layer included
```

Missing path = falsy. To force a layer on (override the condition), set `enabled: true` on the layer entry.

## Tag-based layers

`tags` enable a layer when **any** of the named tags is truthy under `values.tags`. Use this when one toggle should activate a coordinated set of layers.

```yaml
# keramos.yaml
layers:
  - name: monitoring
    source: ./layers/monitoring
    tags: [observability]

  - name: logging
    source: ./layers/logging
    tags: [observability]

  - name: tracing
    source: ./layers/tracing
    tags: [observability, debug]
```

```yaml
# values.yaml
tags:
  observability: true       # monitoring + logging + tracing included
  debug: false
```

A single value file or `--set tags.observability=true` enables all three layers.

`tags` and `condition` are mutually exclusive on the same layer. If both are set, `tags` wins.

## Explicit `enabled` override

```yaml
layers:
  - name: experimental
    source: ./layers/experimental
    condition: features.experimental
    enabled: false             # explicit false ALWAYS wins, regardless of condition
```

When `enabled` is non-nil, it short-circuits the entire enablement decision. Use it sparingly — the usual pattern is just to omit the layer.

## Layer ordering

Layers are merged in declared order:

```yaml
layers:
  - name: a
    source: ./a
  - name: b
    source: ./b
  - name: c
    source: ./c
```

Values: `a` is merged first, `b` overrides keys from `a`, `c` overrides keys from `a` and `b`, then the parent's own `values.yaml` overrides everything. Templates and hooks: collisions resolve by "later layer wins, parent always wins".

For deterministic builds, layer order matters even when no field collides — the merge is documented stably so a `keramos diff` between two installs is meaningful.

## The lockfile

`keramos dependency update` resolves every layer's `source` + `version` + `ref` to a specific digest and writes `keramos.lock`:

```yaml
apiVersion: keramos/v1
generated: 2026-05-08T10:14:32Z
layers:
  - name: shared-base
    source: ../shared-base
    digest: sha256:9c1b8e3f4a...
  - name: redis
    source: oci://ghcr.io/example/redis-layer
    resolvedVersion: 2.4.1
    digest: sha256:7e2a90d8c5...
  - name: monitoring
    source: git::https://github.com/example/monitoring-layer.git
    ref: v3.1.0
    resolvedCommit: 4f2a8c1...
    digest: sha256:6db4f2e1a3...
```

`keramos install` and `keramos template` consult `keramos.lock` first; the constraint in `keramos.yaml` is the *upper* bound, the lockfile is the *exact* version. Re-run `keramos dependency update` to bump the lock; `keramos dependency build` fetches every locked layer into the local cache.

**Commit `keramos.lock`** — without it, two builds of the same package can pull different versions of the same layer if the constraint allows it.

## What templates inside a layer see

Layer templates render with the same `values`/`release`/`package`/`capabilities` namespaces as the parent's templates, against the same merged values map. There is no per-layer namespace at template time — both the parent and every layer see the union of all merged values, with the parent's `layers.<layer-name>.<key>` overrides applied. See [Layers in templates](../templates/layers.md) for the full mechanics.

A layer can use `${release.name}` to make its resource names unique across releases:

```yaml
# in shared-base/templates/serviceaccount.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${release.name}-base
  labels:
    app.kubernetes.io/instance: ${release.name}
    app.kubernetes.io/managed-by: keramos
```

The layer can also read `${package.name}` and `${package.version}` — but those resolve to the **root** package's identity (the package being installed), not the layer's identity. Layers do not have a separate identity at template time.

## When layers don't fit

- The pieces have **independent lifecycles** (you upgrade redis on its own schedule). → use `requires:` or a workspace.
- The pieces should be **separately deletable**. → use `requires:`.
- The pieces should run in **different namespaces**. → use a workspace member.
- You only want to **share a partial** like a label helper. → publish a small library package (`type: library`) and pull it in as a layer; templates from a library are fine, but library packages can't be installed standalone.
