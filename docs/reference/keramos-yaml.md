# `keramos.yaml` Reference

`keramos.yaml` is the package manifest at the root of every keramos package. It declares the package's identity, version, layers, dependencies, environments, and supporting metadata. The file is required; without it a directory is not a keramos package and `keramos lint`, `keramos install`, `keramos template`, and friends will refuse to operate.

This document describes every field keramos recognises. Fields not listed here are ignored; keramos's loader uses strict typed parsing for the keys it knows about and tolerates extras for forward compatibility.

---

## Top-level fields

### `apiVersion` (string, required)

Format identifier. The current value is `keramos/v1`. Future schema migrations will introduce `keramos/v2` etc.; keramos rejects an unrecognised `apiVersion` rather than guessing. Example:

```yaml
apiVersion: keramos/v1
```

### `name` (string, required)

The package name. Must be a DNS-1123 label: lowercase letters, digits, and hyphens; cannot start or end with a hyphen; max 63 characters. The name is used as the default release name and as a directory anchor when a layer is published to a registry.

```yaml
name: my-app
```

### `version` (string, required)

The package version. Must be a valid SemVer 2.0 string (`MAJOR.MINOR.PATCH`, with optional pre-release and build suffixes). keramos uses this for ordering, range constraints in dependency declarations, and to populate the release record's `package.version`.

```yaml
version: 1.4.2
version: 2.0.0-rc.1
version: 1.0.0+build.42
```

### `appVersion` (string, optional)

The version of the application packaged inside (independent of the package's own version). Useful when the package itself rarely changes but the upstream image bumps versions every week. Pure metadata — keramos does not parse it.

```yaml
appVersion: "3.7.1"
```

### `description` (string, optional)

A one- or two-sentence description shown by `keramos list`, `keramos show chart`, and registry indexes.

```yaml
description: Eclipse Mosquitto MQTT broker with persistent storage.
```

### `type` (string, optional)

Either `application` (default — installs as a release) or `library` (cannot be installed standalone; only used as a layer by other packages). Library packages are useful for shared scaffolding.

```yaml
type: library
```

### `kubeVersion` (string, optional)

A SemVer range constraint for the Kubernetes server version. `keramos install` and `keramos lint` reject the package when the cluster's version is outside the constraint. Use SemVer constraint syntax: `>=1.27.0 <1.31.0`, `~1.28.0`, `^1.27`.

```yaml
kubeVersion: ">=1.28.0"
```

### `keywords` (string list, optional)

Free-form tags used by `keramos search` and registry indexes. No semantic meaning to keramos itself.

```yaml
keywords:
  - mqtt
  - messaging
  - iot
```

### `maintainers` (object list, optional)

Each maintainer has `name` (string, required) and `email` (string, optional). Pure metadata.

```yaml
maintainers:
  - name: Jane Doe
    email: jane@example.com
  - name: Anonymous
```

### `annotations` (string→string map, optional)

Arbitrary key/value metadata. keramos recognises a small set of well-known annotations under the `keramos.sh/...` prefix (e.g. `keramos.sh/category`, `keramos.sh/icon`); anything else is preserved and surfaced via `keramos show chart` but otherwise inert.

```yaml
annotations:
  keramos.sh/category: messaging
  keramos.sh/icon: https://example.com/icon.png
```

---

## Composition: `layers` and `requires`

Keramos packages compose through **layers** — child packages whose templates and values are merged into the parent. The two list fields, `layers` and `requires`, share the same item schema (`LayerSource`) but mean different things:

- `layers`: composition. Each layer's templates are added to the parent package's render set; values are merged. The result is a single rendered manifest belonging to **one release**. Use this when you want one installable unit composed from reusable building blocks.

- `requires`: co-deployed packages. Each entry must already be installed as its own release before the depending package can be installed. Use this when releases must be **separate** but ordered (e.g. a database release that an app depends on). keramos's workspace and release-orchestration commands honour `requires`.

### `LayerSource` schema

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Local identifier for the layer. Must be unique within `layers` / `requires`. Becomes the layer's directory name in `.keramos/cache/`. |
| `source` | string | yes | Where to find the layer. See *Source schemes* below. |
| `version` | string | no | SemVer constraint. Required for registry sources; ignored for `local` and `git`. |
| `ref` | string | no | Git reference (branch, tag, or commit SHA). Used only when `source` starts with `git::`. |
| `condition` | string | no | Dotted path into merged values. The layer is enabled only when the value at this path is truthy. See *Conditional layers*. |
| `tags` | string list | no | Tag-based enablement. The layer is enabled when **any** named tag is set truthy under `values.tags.<name>`. |
| `alias` | string | no | Alternate name to use for this layer in the parent package's rendering context. Useful when two layers share a `name`. |
| `enabled` | bool | no | Explicit override. When set, ignores `condition` and `tags` entirely. |

### Source schemes

The `source` field accepts these forms; keramos picks the handler by prefix.

- **Local path** (no scheme prefix, just a path):

  ```yaml
  source: ../shared-base
  source: ./packages/sidecar
  ```

  Resolved relative to the `keramos.yaml` containing it. Useful for monorepo layouts.

- **HTTPS chart archive**:

  ```yaml
  source: https://charts.example.com/my-package-1.2.3.tgz
  ```

  Downloads and verifies the archive. The `version` field can be a constraint when the URL is a repository index that resolves to a versioned archive.

- **Git repository** (`git::` prefix):

  ```yaml
  source: git::https://github.com/org/repo.git
  ref: v1.4.2
  source: git::ssh://git@github.com/org/repo.git
  source: git::/absolute/local/path/to/checkout
  ```

  Keramos clones the repository (shallow if `ref` is a tag/branch) and resolves the package path relative to the clone root. The `ref` field selects branch/tag/commit; default is the repository's HEAD.

- **OCI registry** (`oci://` prefix):

  ```yaml
  source: oci://ghcr.io/org/charts/my-package
  version: 1.2.3
  ```

  Pulls via OCI distribution-spec API. Honours credentials stored by `keramos login` (or `keramos registry login`). Set `KERAMOS_OCI_PLAIN_HTTP=true` for non-TLS registries; `KERAMOS_OCI_INSECURE_SKIP_TLS=true` to skip cert validation.

### Conditional layers

The `condition` field gates a layer on a values lookup. The path is dotted; the layer is enabled when the resolved value is truthy.

```yaml
layers:
  - name: redis
    source: oci://ghcr.io/example/redis-layer
    version: 2.0.0
    condition: redis.enabled
```

```yaml
# values.yaml
redis:
  enabled: true   # layer is included
```

If the path doesn't resolve (key missing), the layer is **disabled**. To force a layer on regardless of values, set `enabled: true`.

### Tag-based layers

`tags` enable a layer when any of the named tags is truthy under `values.tags`. This lets a single value flag toggle several related layers at once.

```yaml
layers:
  - name: monitoring
    source: ./layers/monitoring
    tags: [observability]
  - name: logging
    source: ./layers/logging
    tags: [observability]
```

```yaml
# install with --set tags.observability=true to include both
tags:
  observability: false
```

### Layer ordering

Layers are merged in declared order. Later layers override earlier ones for values; for templates, later layers add new files (collisions are resolved by keeping the parent package's version, then the latest declared layer).

---

## Environments

The `environments` field declares named deployment targets — `dev`, `staging`, `prod`, etc. — each with its own value overrides, namespace, and kubeconfig context. Selected by `keramos install --env staging` / `keramos upgrade --env prod`.

### `Environment` schema

| Field | Type | Description |
|---|---|---|
| `inherits` | string | Name of another environment in this `environments` block. The current environment merges *over* the inherited one, allowing layered environment definitions (e.g. `staging` inherits from `dev`, `prod` inherits from `staging`). Cycles are detected and rejected. |
| `valueFiles` | string list | Additional `-f`-style value files merged in declared order. Paths are relative to the package root. |
| `values` | map | Inline value overrides. Merged on top of `valueFiles` and the package's `values.yaml`. |
| `profile` | string | Profile name to activate (see *Profiles*, below). |
| `namespace` | string | Default install namespace for this environment. CLI `-n` overrides if both are given. |
| `cluster` | string | Default kubeconfig context for this environment. CLI `--kube-context` overrides. |

```yaml
environments:
  dev:
    namespace: my-app-dev
    values:
      replicas: 1
      image:
        tag: latest

  staging:
    inherits: dev
    namespace: my-app-staging
    valueFiles:
      - profiles/staging.yaml
    values:
      replicas: 2

  prod:
    inherits: staging
    namespace: my-app
    cluster: prod-cluster
    values:
      replicas: 5
      image:
        tag: 1.4.2
```

The merge order top-down for `prod` is:

1. `values.yaml` at the package root.
2. `dev.values` (inherited transitively).
3. `staging.valueFiles` (`profiles/staging.yaml`), then `staging.values`.
4. `prod.values`.
5. CLI `-f` and `--set` flags (highest priority).

---

## Schema enforcement

The package's `values.schema.json` (a JSON Schema document at the package root) is applied to the merged values before render. Fields like `pattern`, `format`, `enum`, `oneOf`, `$ref`, and `dependentRequired` are all supported. See [`docs/reference/values-schema-json.md`](values-schema-json.md) for the full subset.

---

## Mutation safety: `immutable`

Use `immutable` to declare values that cannot change between revisions of a release once initially set. Each entry is a dotted values path. `keramos upgrade` rejects an upgrade that changes any of these.

```yaml
immutable:
  - storage.persistentVolumeClaim.size
  - postgres.databaseName
```

This is enforced at the keramos layer, before any cluster-side admission webhook runs, so the upgrade fails fast with a clear error.

---

## Deprecated fields

These remain readable for older packages but should not be used in new ones:

- `base` (string) — single-source convenience shorthand. Replaced by adding a single entry to `layers` with `name: base, source: <path>`.
- `dependencies` (object list) — uses the older `name`/`version`/`repository` triplet. Replaced by `layers` (or `requires` for separate releases) with the unified `LayerSource` schema.

When both new and deprecated fields are present, `layers` wins.

---

## Minimal example

```yaml
apiVersion: keramos/v1
name: my-app
version: 1.0.0
description: A small example application.
appVersion: "1.4.2"

maintainers:
  - name: Jane
    email: jane@example.com

layers:
  - name: shared-base
    source: ../shared-base
  - name: redis
    source: oci://ghcr.io/example/redis-layer
    version: ^2.0.0
    condition: redis.enabled

environments:
  dev:
    namespace: my-app-dev
    values:
      replicas: 1
  prod:
    inherits: dev
    namespace: my-app
    cluster: prod-cluster
    values:
      replicas: 5

immutable:
  - storage.size
```
