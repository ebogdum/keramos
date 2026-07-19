# Values

Values are a package's configurable surface. Every choice you make about an
installation lands in the merged values map and reaches templates as
`${values.x.y.z}`. This guide explains how keramos builds that map, the precedence
between sources, and how to trace where a value came from.

## Where values come from

Keramos builds the **merged values** by deep-merging these sources, later
overriding earlier:

1. Each layer's `values.yaml`, in declared layer order.
2. The package's own `values.yaml`.
3. The selected environment (`--env`): its `valueFiles[]` then inline `values`.
4. The selected profile (`--profile`): `profiles/<name>.yaml`.
5. `-f` / `--values <file>` (in CLI order).
6. `--set key=value` (in CLI order).
7. `--set-file key=@path` (file contents become the value).
8. `--set-string key=value` (force string typing).
9. `--set-json key=<json>` (parse the value as JSON).

Maps merge recursively; lists and scalars are replaced. Setting a key to `null`
removes it from earlier sources.

## Authoring `values.yaml`

A package's `values.yaml` is its public configuration contract:

1. **Default to boring, working values.** Installing with no `--set` should
   produce a working release.
2. **Document every key** with comments.
3. **Keep the shape stable** — renaming a top-level key breaks every release.

```yaml
# values.yaml

# How many replicas of the main Deployment to run.
replicas: 1

# Container image; pin the tag for reproducibility.
image:
  repository: nginx
  tag: "1.27.0"
  pullPolicy: IfNotPresent

# Service exposure.
service:
  type: ClusterIP
  port: 80

# Resource requests/limits. Override in production.
resources:
  requests: { cpu: 100m, memory: 128Mi }
  limits:   { memory: 512Mi }
```

## CLI override syntax

The four `--set*` flags share path semantics:

| Path expression | Meaning |
|---|---|
| `replicas=3` | Set top-level `replicas` to integer 3. |
| `image.tag=1.4.2` | Set nested `image.tag` (auto-typed unless `--set-string`). |
| `args[0]=--debug` | Set an array index (created / auto-extended as needed). |
| `args[]=--debug` | Append to an array. |
| `labels.app\.kubernetes\.io/name=api` | Backslash-escape a literal `.` in a key. |
| `tolerations[0].key=node-role` | Mixed array + map path. |

`--set-string image.tag=2.0` is the fix for "the tag became a float": without
`-string`, keramos parses `2.0` as a number and the image reference breaks.
`--set-file tlsCert=@./server.crt` embeds a file's contents. `--set-json
affinity={...}` accepts an inline JSON value.

See [`keramos install`](../cli/install.md) for the full flag list.

## Tracing a value

When a merged value surprises you, `keramos values` resolves the package exactly as
install would and prints the result:

```sh
keramos values .
```

```yaml
image:
    repository: nginx
    tag: latest
name: hello
replicaCount: 1
```

Add `--trace <dotted.key>` to see the resolution chain for one key — every
contributor in order, with the winner marked `→`:

```sh
keramos values . -f overrides.yaml --set replicas=9 --trace replicas
```

```
replicas:
    package-default (values.yaml) = 5
    values-file (overrides.yaml) = 3
  → set (replicas=9) = 9
```

`keramos values` is offline and takes a **package path**, not a release name. To
trace a value on an already-installed release, use
[`keramos get provenance <release>`](../cli/get.md):

```sh
keramos get provenance hello -n keramos-quickstart
```

```
VALUE               SOURCE
image.repository    package-default (values.yaml)
replicaCount        package-default (values.yaml)
```

See [`keramos values`](../cli/values.md).

## Layer values

Every layer's `values.yaml` is deep-merged with the parent's into one flat map,
so both the parent's templates and the layer's templates read the same values at
`${values.*}`. The parent overrides a layer's contribution either at the top
level or through a `layers.<layer-name>.<key>` block:

```yaml
# parent values.yaml
replicas: 3
layers:
  redis:
    password: hunter2      # overrides the redis layer's default
```

The `layers` block is consumed during merge and is not present in the final
context. To pin a value so a *later layer* cannot override it, prefix the key
with `!`:

```yaml
layers:
  redis:
    "!password": locked-by-a-layer
```

The `!` is stripped during merge and the value wins over other layers'
contributions to that key. Note that CLI `-f` / `--set` overrides and
environment/profile overlays still take precedence over a pinned layer value —
`!` governs layer-vs-layer precedence, not the whole chain.

### The `global` convention

Cluster-wide settings conventionally live under `values.global`:

```yaml
global:
  domain: example.com
  imageRegistry: registry.example.com
```

Because everything merges flat, every template — parent or layer — reads
`${values.global.domain}` with no special scoping. `global` is just an agreed
top-level key. `tags` is similar: a reserved top-level key for layer enablement
(see [Layers](layers.md#tag-based-layers)).

## Schema validation

When `values.schema.json` is present, keramos validates the merged values before
render (during `keramos template`, `keramos install`, and `keramos upgrade`) and aborts
on any violation:

```sh
keramos template . --set replicas=100
```

```
Error: values failed schema validation:
  - $.replicas: 100 greater than maximum 50
```

`keramos lint` does **not** validate values against the schema — it only checks
that the schema file is valid JSON. For a CI gate, run `keramos template` (or
`keramos install --dry-run server`) so the schema check actually runs. See
[Schema validation](schema-validation.md) and the
[`values.schema.json` reference](../reference/values-schema-json.md).

## Environments

Use `environments` in `keramos.yaml` to bake the dev/staging/prod split into the
package instead of side files:

```yaml
# keramos.yaml
environments:
  dev:
    namespace: my-app-dev
    values:
      replicas: 1
  prod:
    inherits: dev               # start from dev's values
    namespace: my-app
    values:
      replicas: 5
```

Activate with `--env prod`:

```sh
keramos template . --env prod
```

renders `replicas: 5` (prod inherits dev, then overrides). See the
[`keramos.yaml` reference](../reference/keramos-yaml.md) for the full environment
schema.

## Profiles

Profiles are simpler than environments — just an overlay file under
`profiles/<name>.yaml`. Use them for orthogonal axes (e.g. `single-node` vs
`ha-3node`):

```yaml
# profiles/ha-3node.yaml
replicas: 3
storage:
  storageClass: ssd-replicated
```

```sh
keramos install pg-ha . --profile ha-3node
```

The profile merges above `values.yaml` and below `-f`/`--set`.

## Working with secrets

`values.yaml` is plain text — do not put secrets in it. Instead:

- **Reference an external Secret**: `existingSecret: my-app-creds`, and in
  templates `valueFrom.secretKeyRef.name: ${values.existingSecret}`.
- **`--set-file`** a secret file at install time: `--set-file tlsCert=@./tls.crt`.
- **External secret operators** (ExternalSecrets, Sealed Secrets, vault
  injector) — the package only references the materialised Secret name.

For ad-hoc generation, the `randAlphaNum`, `genCA`, and `genSelfSignedCert`
template functions can materialise a default, with `existingSecret` as the
production override. Provide fallbacks with `default`, e.g.
`${values.image.tag | default package.version}`. See the
[crypto & secrets functions](../templates/functions/crypto-secrets.md).

## See also

- [Package anatomy](packages.md) — where these files live.
- [Layers](layers.md) — how layer values compose.
- [Schema validation](schema-validation.md) — constraining values.
- [Expressions](../templates/expressions.md) — the `${...}` language.
