# `values.yaml` Reference

`values.yaml` is the package's default configuration. Every package must ship one (even if empty); it lives alongside `keramos.yaml` at the package root. Values are the only thing template authors should expect to be configurable from the outside.

Keramos does not impose a fixed schema on `values.yaml` — packages define whatever shape suits the application. What keramos does specify is *how* values are merged, how they are validated, and a small reserved namespace for keramos's own use.

---

## Merge order

When rendering a release, keramos builds the **effective values** by deep-merging in this order (later wins):

1. Each layer's `values.yaml`, in declared layer order.
2. The package's own `values.yaml`.
3. The selected environment's `valueFiles` (in declared order), then `environments.<env>.values`.
4. CLI `-f / --values <file>` arguments, in CLI order.
5. CLI `--set key=value` arguments, in CLI order.
6. CLI `--set-file key=@path` (file contents become the value), in CLI order.
7. CLI `--set-string key=value` (forces string typing), in CLI order.

Every step is a recursive deep-merge for maps and a replace for scalars and lists. To explicitly **null out** a key from an upstream layer, set it to `null` (YAML `~` or empty value).

---

## CLI override syntax

The flags `--set`, `--set-string`, `--set-file`, and `--set-json` follow consistent path semantics:

| Path expression | Meaning |
|---|---|
| `replicas=3` | Sets top-level `replicas`. |
| `image.tag=1.4.2` | Sets nested `image.tag`. Intermediate maps are created. |
| `args[0]=--debug` | Sets array index. Array is created if absent; out-of-range indices auto-extend with `null`. |
| `labels.app\.kubernetes\.io/name=api` | Backslash-escapes literal `.` in keys. |
| `tolerations[0].key=node-role` | Mixed array+map paths. |

`--set-string` forces the value to be a string even if it parses as a number or bool. Use it for `image.tag=2.0` (otherwise becomes `2.0` float, breaking image refs).

`--set-file` reads the file at the given path and uses its contents as the value (useful for cert PEMs, license keys).

`--set-json key=<json>` accepts a JSON document for the value, useful for embedding lists or objects.

---

## Reserved keys

These top-level keys are reserved by keramos and may not be repurposed by package authors:

| Key | Purpose |
|---|---|
| `tags` | Tag-based layer enablement. `tags.<name>: true` enables every layer that lists `<name>` in its `tags`. |
| `global` | A merged-from-everywhere bag. Available as `.Values.global` in *every* layer's templates regardless of nesting; useful for cluster-wide settings (DNS suffix, image registry, etc.). |

`global` is auto-propagated downwards: parent values are visible in child layers under `.Values.global` even when not declared in the child's own `values.yaml`. Conversely, a child's `values.global` is **not** auto-propagated upward.

---

## Layer-scoped values

When a package has layers, each layer's templates render against its **own** namespaced values. By default, a layer's values are nested under the layer's `name`. So:

```yaml
# parent values.yaml
shared-base:
  image:
    repository: registry.example.com/api
redis:
  enabled: true
  password: changeme
```

Inside the `shared-base` layer's templates, `.Values.image.repository` evaluates to `registry.example.com/api`. The layer cannot accidentally read its sibling's values.

The `global` namespace is exempt — every layer sees the merged `global` block at `.Values.global`.

---

## Type validation via `values.schema.json`

If the package ships a `values.schema.json` next to `values.yaml`, keramos validates the merged values against it before rendering. The schema is a standard JSON Schema document. Keramos supports a useful subset:

- Type primitives: `type: object|array|string|number|integer|boolean|null`.
- Required fields: `required: [...]`.
- String constraints: `pattern`, `format` (`email`, `uri`, `uuid`, `ipv4`, `ipv6`, `date`, `date-time`, `hostname`).
- Numeric constraints: `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf`.
- Array constraints: `items`, `minItems`, `maxItems`, `uniqueItems`.
- Object constraints: `properties`, `additionalProperties`, `patternProperties`, `minProperties`, `maxProperties`, `dependentRequired`.
- Combinators: `allOf`, `anyOf`, `oneOf`, `not`, `const`, `enum`.
- Local references: `$ref` to `#/$defs/...` or `#/definitions/...` (no remote `$ref`).

See [`values-schema-json.md`](values-schema-json.md) for the full subset and examples.

When a schema is present, `keramos lint` runs validation as part of its pass. `keramos config` is an interactive walker that uses the schema to prompt the user through a values file.

---

## Common conventions

These aren't keramos-imposed but are widely used in the ecosystem; following them makes packages easier to compose.

```yaml
# Image references — group repository, tag, pull policy, pull secrets.
image:
  repository: nginx
  tag: "1.27.0"
  pullPolicy: IfNotPresent
  pullSecrets: []

# Replicas and resource hints.
replicas: 1
resources:
  requests: { cpu: 100m, memory: 128Mi }
  limits:   { memory: 512Mi }

# Service exposure.
service:
  type: ClusterIP
  port: 80
  annotations: {}

# Ingress.
ingress:
  enabled: false
  className: nginx
  hosts: []
  tls: []

# Persistence.
persistence:
  enabled: true
  storageClass: ""        # empty → cluster default
  size: 10Gi
  accessModes: [ReadWriteOnce]

# Pod-level config.
podAnnotations: {}
podSecurityContext: {}
nodeSelector: {}
tolerations: []
affinity: {}

# Auth / secrets.
existingSecret: ""        # if non-empty, keramos templates should reference this Secret instead of generating one
```

---

## Inspecting effective values

Two related commands:

- `keramos values <package-path>` — resolves the values exactly as `keramos install` would (defaults → layers → environment → -f → --set → --set-file → --set-json → --set-string) and prints the merged result. Operates locally; does not touch the cluster. Useful for previewing what a not-yet-installed release would see.

- `keramos get values <release>` — fetches the merged values that were actually stored when a release was installed/upgraded. Operates against the release record in the cluster.

```sh
# Preview merged values from a package directory (no cluster contact):
keramos values ./my-app

# Layer overrides on top of the package defaults:
keramos values ./my-app -f overrides.yaml --set replicas=5

# Trace one key's resolution chain (which layer / file / flag set it, in order):
keramos values ./my-app --trace replicas

# Effective values stored by the live release in the cluster:
keramos get values my-app -n prod

# Stored values for a historical revision:
keramos get values my-app --revision 3 -n prod
```

`--trace` is invaluable when chasing why a value ended up the way it did across multi-layer packages.
