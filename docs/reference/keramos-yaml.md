# keramos.yaml

The package manifest at the root of every keramos package. It declares the
package's identity and version and wires in composition layers, required
co-deployed packages, and named environments. Without a `keramos.yaml`, a
directory is not a keramos package and `keramos lint`, `keramos install`,
`keramos template`, and related commands refuse to run.

## Minimal example

```yaml
apiVersion: keramos/v1
name: my-app
version: 0.1.0
```

`apiVersion`, `name`, and `version` are the only required fields. `keramos create`
generates exactly this plus a `description`.

## Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `apiVersion` | string | yes | — | Manifest format identifier. Use `keramos/v1`. An unrecognised value is rejected rather than guessed. |
| `name` | string | yes | — | Package name. Must match `^(@scope/)?name$` where each part is lowercase alphanumerics, `-`, and `.` (scoped names like `@team/api` are allowed). Used as the default release name. |
| `version` | string | yes | — | Package version. Required and non-empty; use SemVer (`1.4.2`, `2.0.0-rc.1`) so registry range constraints and ordering work. |
| `appVersion` | string | no | — | Version of the application shipped inside, independent of `version`. Pure metadata; keramos does not parse it. |
| `description` | string | no | — | One-line human summary. Metadata only. |
| `type` | string | no | — | `application` or `library`, Helm-style. Metadata only. |
| `kubeVersion` | string | no | — | SemVer constraint on the target cluster's Kubernetes version, recorded as compatibility metadata. |
| `layers` | list | no | — | Composition layers merged into this one release. See [LayerSource](#layersource-fields). |
| `requires` | list | no | — | Sibling packages that must be co-deployed with this one. Same shape as `layers`, but each stays a separate release rather than merging in. See [LayerSource](#layersource-fields). |
| `immutable` | string list | no | — | Resource identifiers the package marks as immutable. Accepted by the manifest; not enforced by any current command. |
| `maintainers` | list | no | — | Package owners. Each has `name` (required) and `email`. Metadata only. |
| `keywords` | string list | no | — | Search/classification keywords. Metadata only. |
| `annotations` | string→string map | no | — | Arbitrary key/value metadata carried with the package. |
| `environments` | map | no | — | Named deployment targets (dev, staging, prod, …) with per-env value overrides and inheritance. See [Environment](#environment-fields). |
| `base` | string | no | — | Deprecated. A single layer source; equivalent to one `layers` entry named `base`. Use `layers`. |
| `dependencies` | list | no | — | Deprecated. Legacy Helm-style dependencies; each becomes a `layers` entry. Use `layers`. |

If `layers` is set it wins; otherwise `base` and `dependencies` are converted
to layers for backward compatibility.

### LayerSource fields

Each entry of `layers` and `requires` is a layer source.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | string | yes | — | Local identifier for the layer, unique within the list. |
| `source` | string | yes | — | Where the layer comes from: a local path, an `https://` URL, or a `git::` URL. Registry sources resolve `version`. |
| `version` | string | no | — | SemVer constraint for registry sources. Ignored for local and git sources. |
| `ref` | string | no | — | Git ref (branch, tag, or commit) when `source` is a `git::` URL. |
| `condition` | string | no | — | Dotted path into merged values. The layer is enabled only when the value at that path is truthy. |
| `tags` | string list | no | — | Tag names. The layer is enabled when any listed tag is truthy under `values.tags.<name>`. |
| `alias` | string | no | — | Alternate name for the layer in this package, e.g. to include the same source twice. |
| `enabled` | bool | no | follow `condition`/`tags` | Explicit on/off override. When set it wins over `condition` and `tags`; when unset, enablement follows `condition`, then `tags`, else on. |

### Environment fields

Each value under `environments` is a named target.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `inherits` | string | no | — | Name of another environment this one extends. The child merges over the parent; inheritance cycles are rejected. |
| `valueFiles` | string list | no | — | Extra value files (like repeated `-f`) merged in declared order. |
| `values` | map | no | — | Inline value overrides, merged on top of `valueFiles`. |
| `profile` | string | no | — | Profile to activate for this environment. |
| `namespace` | string | no | — | Default install namespace. A CLI `-n` wins over it. |
| `cluster` | string | no | — | Default kubeconfig context. A CLI `--kube-context` wins over it. |

## Full example

```yaml
apiVersion: keramos/v1
name: platform-api
version: 1.4.2
appVersion: "2025.7"
description: Public API for the platform
type: application
kubeVersion: ">=1.27.0"

maintainers:
  - name: Platform Team
    email: platform@example.com
keywords: [api, http, platform]
annotations:
  team: platform

# Composition layers merge into this one release.
layers:
  - name: base
    source: ../shared/base            # local path
  - name: redis
    source: oci://registry.example.com/layers/redis
    version: ">=7.0.0 <8.0.0"
    condition: cache.enabled          # on only when values.cache.enabled is truthy
  - name: metrics
    source: git::https://github.com/example/layers.git//metrics
    ref: v1.2.0
    tags: [observability]             # on when values.tags.observability is truthy

# Co-deployed sibling packages (separate releases).
requires:
  - name: cert-manager
    source: oci://registry.example.com/pkgs/cert-manager
    version: ">=1.14.0"

# Named targets: `keramos install --env prod` folds these overrides in.
environments:
  dev:
    namespace: platform-dev
    values:
      replicaCount: 1
  prod:
    inherits: dev                     # prod = dev overrides + these
    namespace: platform-prod
    profile: production
    values:
      replicaCount: 5
      cache:
        enabled: true
```

With `values.cache.enabled: true` and `--env prod`, the `redis` layer activates
and its templates render into the single `platform-api` release, while
`cert-manager` is resolved as a separate required release.

## See also

- [`keramos create`](../cli/create.md) — scaffold a package with a starter manifest.
- [`keramos lint`](../cli/lint.md) — validate the manifest and package.
- [`keramos install`](../cli/install.md) / [`keramos template`](../cli/template.md) — render and apply using the manifest.
- [`keramos env`](../cli/env.md) — work with declared environments.
- [Layers guide](../guides/layers.md), [Packages guide](../guides/packages.md).
