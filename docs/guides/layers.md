# Layers

A layer is another keramos package whose templates and values compose into the
current package, producing a single rendered manifest. Layers let you build
small reusable pieces — a "common labels" layer, a "Postgres-with-PVC" layer, a
"monitoring sidecar" layer — and combine them into a larger application without
copy-pasting YAML.

This guide covers when to reach for a layer, how composition works, and the
patterns for conditional and tag-based inclusion.

## Layer vs. requires vs. workspace member

| Construct | Result | Use when |
|---|---|---|
| **Layer** (`layers:` in `keramos.yaml`) | Templates and values compose into **one** release. | The pieces always ship together. |
| **Requires** (`requires:` in `keramos.yaml`) | Each entry is its own **separate** release; the parent's install asserts they exist. | Loosely-coupled releases with their own lifecycle. |
| **Workspace member** (`members:` in `keramos-workspace.yaml`) | Each member is a separate release the workspace orchestrates. | Independently developed, rolled out as a set. |

`layers:` and `requires:` share the same source schema (see the
[`keramos.yaml` reference](../reference/keramos-yaml.md)); the difference is
install-time only.

## A worked example

A `base` layer that ships a partial and a ServiceAccount, and an `app` package
that composes it:

```
base/
├── keramos.yaml
├── values.yaml            # labelValue: base-default
└── templates/
    ├── _labels.yaml       # partial: common
    └── serviceaccount.yaml

app/
├── keramos.yaml              # layers: [base → ../base]
├── values.yaml            # labelValue: app-override
└── templates/
    └── cm.yaml            # uses $include: common
```

```yaml
# base/templates/_labels.yaml
common:
  app.kubernetes.io/managed-by: keramos
  team: "${values.labelValue}"
```

```yaml
# app/keramos.yaml
apiVersion: keramos/v1
name: app
version: 0.1.0
layers:
  - name: base
    source: ../base
```

Render the parent:

```sh
keramos template ./app
```

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/managed-by: keramos
    team: app-override        # parent's value won
  name: app-cm
---
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app.kubernetes.io/managed-by: keramos
    team: app-override
  name: app-base              # the layer's ServiceAccount, part of this release
```

Both the parent's `cm.yaml` and the layer's `serviceaccount.yaml` render into
one manifest. The layer's `common` partial is visible to the parent's templates,
and the parent's `values.yaml` (`labelValue: app-override`) overrode the layer's
default.

## Composition mechanics

When keramos resolves a package with layers:

1. **Resolve** each layer's source (local, HTTPS, git, OCI) into a local cache.
2. **Merge values** in declared layer order; the parent's `values.yaml`
   overrides them.
3. **Merge templates** — every layer template joins the render set; on a
   filename collision the parent wins (so the parent can replace a layer's
   `service.yaml`).
4. **Merge hooks** — same rules as templates.
5. **Merge CRDs** — added to the apply-first set; a CRD collision across layers
   is an error.

The result is one rendered manifest, one release record, one set of hooks.

## Source schemes

The `source` field's prefix selects the handler.

**Local** — resolved relative to the parent's `keramos.yaml`:

```yaml
layers:
  - name: shared-base
    source: ../shared-base
```

**HTTPS** — keramos downloads and checksum-verifies the archive:

```yaml
layers:
  - name: my-layer
    source: https://charts.example.com/my-layer-1.2.3.tgz
```

**Git** — shallow clone of `ref`; `//subpath` selects a subdirectory:

```yaml
layers:
  - name: my-layer
    source: git::https://github.com/example/repo.git//path/to/package
    ref: v1.2.3
```

**OCI** — pulled via the distribution spec; `version` is a SemVer constraint:

```yaml
layers:
  - name: my-layer
    source: oci://ghcr.io/example/charts/my-layer
    version: ^1.2.0
```

OCI auth comes from [`keramos login`](../cli/login.md); for local registries set
`KERAMOS_OCI_PLAIN_HTTP=1` or `KERAMOS_OCI_INSECURE_SKIP_TLS=1` (or the matching
`--oci-*` flags). See the [OCI guide](oci.md).

## Conditional layers

`condition` gates a layer on a dotted values path; the layer is enabled when the
value is truthy (`true`, non-empty string, non-zero number, non-empty
list/map). Missing path = falsy.

```yaml
# keramos.yaml
layers:
  - name: base
    source: ../base
    condition: base.enabled
```

```yaml
# values.yaml — layer excluded
base:
  enabled: false
```

With `base.enabled: false`, `keramos template ./app` and `keramos dependency tree ./app`
both drop the layer:

```sh
keramos dependency tree ./app
```

```
app@0.1.0
```

Set `base.enabled: true` and the layer (and its ServiceAccount) reappear. At
install and upgrade time, `--set`/`-f` overrides also toggle enablement, so
`keramos install app ./app --set base.enabled=true` activates a layer whose
`values.yaml` default is `false`. (`keramos template` evaluates enablement from
files only — `values.yaml`, layer values, environment, and profile — not from
`--set`.)

## Tag-based layers

`tags` enable a layer when **any** listed tag is truthy under `values.tags`. Use
this when one toggle should activate a coordinated set of layers:

```yaml
# keramos.yaml
layers:
  - name: monitoring
    source: ./layers/monitoring
    tags: [observability]
  - name: logging
    source: ./layers/logging
    tags: [observability]
```

```yaml
# values.yaml
tags:
  observability: true       # monitoring + logging both included
```

## Explicit `enabled` override

`enabled` is an explicit on/off that wins over `condition` and `tags`:

```yaml
layers:
  - name: experimental
    source: ./layers/experimental
    condition: features.experimental
    enabled: false             # always off, regardless of the condition
```

Use it sparingly — usually omitting the layer is clearer.

## Layer ordering

Layers merge in declared order: for values, `a` first, then `b` overriding `a`,
then `c`, then the parent overriding all. For templates and hooks, later layer
wins on a collision and the parent always wins. The order is documented as
stable so a `keramos diff` between two builds is meaningful.

## The lockfile

`keramos dependency update <path>` resolves every layer's `source` + `version` +
`ref` and writes `keramos.lock`:

```sh
keramos dependency update ./app
```

```
Layers updated successfully.
```

```yaml
# keramos.lock (local layer)
apiVersion: keramos/v1
generated: 2026-07-19T00:12:40+02:00
layers:
    - name: base
      source: ../base
```

For versioned sources the lock also records the resolved version (OCI/HTTPS),
resolved commit (git), and a `sha256` digest. `keramos install` and `keramos template`
prefer the locked version; the constraint in `keramos.yaml` is the upper bound, the
lock is the exact pin. `keramos dependency build <path>` fetches every locked layer
into the cache. Inspect the set with:

```sh
keramos dependency list ./app
```

```
LAYERS:
  NAME    TYPE     SOURCE      STATUS
  base    local    ../base     unlocked
```

**Commit `keramos.lock`** — without it, two builds can pull different versions of
the same layer. See [`keramos dependency`](../cli/dependency.md).

## What layer templates see

Layer templates render with the same `values` / `release` / `package` /
`capabilities` namespaces as the parent, against the same merged values map.
There is no per-layer namespace — the parent and every layer see the union of
merged values, with the parent's `layers.<name>.<key>` overrides applied. A
layer uses `${release.name}` to keep its resource names unique across releases;
`${package.name}` and `${package.version}` resolve to the **root** package's
identity, not the layer's. See [Layers in templates](../templates/layers.md).

## When layers don't fit

- **Independent lifecycles** (upgrade redis on its own schedule) → `requires:`
  or a [workspace](workspaces.md).
- **Separately deletable** → `requires:`.
- **Different namespaces** → a workspace member.
- **Share only a partial** → publish a `type: library` package and pull it in as
  a layer; library packages ship templates but cannot be installed standalone.

## See also

- [Package anatomy](packages.md) — the files a layer contributes.
- [Values](values.md) — how layer values merge and the `!` pin.
- [`keramos.yaml` reference](../reference/keramos-yaml.md) — the `layers` schema.
- [Layers in templates](../templates/layers.md) — the rendering mechanics.
