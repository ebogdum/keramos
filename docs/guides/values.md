# Values

Values are the package's configurable surface — every choice you make about an installation lands in the merged values map and propagates through templates as `.Values.x.y.z`. This guide explains how keramos builds the merged values, the precedence rules between sources, and the patterns that keep packages composable.

## Where values come from

Keramos builds the **effective values** by deep-merging from these sources, in this order (later overrides earlier):

1. Each layer's `values.yaml`, in declared layer order.
2. The package's own `values.yaml`.
3. The selected environment's overlays:
   - `environments.<env>.valueFiles[]` (in declared order).
   - `environments.<env>.values` (inline).
4. The selected profile's file (`profiles/<name>.yaml`).
5. CLI `-f / --values <file>` arguments (in CLI order).
6. CLI `--set key=value` arguments (in CLI order).
7. CLI `--set-file key=@path` (file contents become the value).
8. CLI `--set-string key=value` (force string typing).
9. CLI `--set-json key=<json>` (parse the value as JSON).

Maps are merged recursively. Lists are replaced (not concatenated). Scalars are replaced. Setting a value to `null` removes the key from any earlier source.

`keramos values <release> --trace` prints the full per-key resolution: which layer or file or flag set each leaf, in chronological order, so you can answer "why did `image.tag` end up as `dev`?".

## Authoring `values.yaml`

A package's `values.yaml` is its public configuration contract. Three principles:

1. **Default to the safest, most boring values.** A user who installs the package without any `--set` should get a working release.
2. **Document every key.** Use comments. Many tools (and `keramos config`) honour them.
3. **Keep the shape stable.** Renaming a top-level key is a breaking change for every existing release.

```yaml
# values.yaml

# How many replicas of the main Deployment to run.
replicas: 1

# Container image; tag must be pinned for reproducibility.
image:
  repository: nginx
  tag: "1.27.0"
  pullPolicy: IfNotPresent

# Service exposure.
service:
  type: ClusterIP
  port: 80

# Resource requests/limits. Liberal defaults for kicking the tires;
# production users should override.
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    memory: 512Mi

# Pod-level affinity / scheduling. Empty by default.
nodeSelector: {}
tolerations: []
affinity: {}

# Pull secrets. Empty list means "use the SA's defaults".
imagePullSecrets: []
```

## CLI override syntax

The four `--set*` flags have consistent path semantics:

| Path expression | Meaning |
|---|---|
| `replicas=3` | Sets top-level `replicas` to integer 3. |
| `image.tag=1.4.2` | Sets nested `image.tag` to string `"1.4.2"` (auto-typed unless --set-string). |
| `args[0]=--debug` | Sets array index. Array is created if absent; out-of-range indices auto-extend with `null`. |
| `args[]=--debug` | Append to array. |
| `labels.app\.kubernetes\.io/name=api` | Backslash-escape literal `.` in keys. |
| `tolerations[0].key=node-role` | Mixed array+map paths. |

`--set-string image.tag=2.0` is the canonical fix for "image tag became a float". Without `-string`, keramos parses `2.0` as a number, and the resulting image reference is broken.

`--set-file tlsCert=@./server.crt` reads the file and embeds its contents. Useful for cert PEMs, private keys, license blobs.

`--set-json affinity={"nodeAffinity":{"required":...}}` accepts an inline JSON value. Useful for embedding lists or objects without crafting an `-f` file.

## Layer values

Every layer's `values.yaml` is deep-merged with the parent's `values.yaml` into a single flat map. Both the parent's templates and the layer's templates see the same merged values at `${values.*}`. The parent can override a layer's contribution via the `layers.<layer-name>.<key>` block in its own `values.yaml`; the merger consumes that block before producing the final flat map.

```yaml
# layer (redis/values.yaml)
password: changeme         # default, may be overridden
port: 6379

# parent (my-app/values.yaml)
replicas: 3
layers:
  redis:
    password: hunter2      # overrides the layer's "changeme"
```

After merge, every template (parent's and layer's) sees:

```yaml
replicas: 3
password: hunter2          # parent override won
port: 6379                 # layer's value
```

The `layers` block is consumed by the merger and is not present in the resulting context.

For a key that should always win regardless of merge order (including over profiles or environment overlays loaded later), prefix it with `!`:

```yaml
# parent values.yaml
layers:
  redis:
    "!password": locked-by-policy
```

The `!` is stripped from the key during merge; the value is applied at the highest precedence level.

### The `global` convention

A widely-used convention is to put cluster-wide settings under `values.global`:

```yaml
# parent values.yaml
global:
  domain: example.com
  imageRegistry: registry.example.com
```

Because all values merge flat, every template — parent or layer — reads them as `${values.global.domain}` without any special scoping rules. `global` is just a top-level key that the ecosystem agrees to put cross-cutting settings in.

`tags` is similar — a top-level key reserved for layer enablement (see [Layers](layers.md)).

## Schema validation

When `values.schema.json` is present, keramos validates the merged values before render. This catches:

- **Missing required keys** with their full path: `auth.password is required`.
- **Type mismatches**: `replicas: expected integer, got string`.
- **Out-of-range values**: `replicas: must be <= 50`.
- **Unknown keys** (when the schema sets `additionalProperties: false`): `top-level key 'reples' not in schema (did you mean 'replicas'?)`.

`keramos lint` runs validation. `keramos install`/`keramos upgrade`/`keramos template` all validate before render. See [`values.schema.json` reference](../reference/values-schema-json.md) for the supported subset.

## Environments

Use `environments` in `keramos.yaml` to bake the dev/staging/prod split into the package itself, instead of side files like `values-dev.yaml`:

```yaml
# keramos.yaml
environments:
  dev:
    namespace: my-app-dev
    values:
      replicas: 1
      image:
        tag: latest
      debug: true

  staging:
    inherits: dev               # base on dev's values
    namespace: my-app-staging
    valueFiles:
      - profiles/staging.yaml
    values:
      replicas: 2
      debug: false

  prod:
    inherits: staging
    namespace: my-app
    cluster: prod-cluster       # default kubeconfig context
    values:
      replicas: 5
      image:
        tag: 1.4.2
```

Activate with `--env staging`. Keramos merges:

```
package's values.yaml  →  inherited dev.values  →  staging.valueFiles  →  staging.values  →  CLI flags
```

The `cluster` field is the default kubeconfig context for that environment; `--kube-context` on the CLI overrides if both are present.

## Profiles

Profiles are simpler than environments — just a values overlay file under `profiles/<name>.yaml`. Use them for orthogonal axes that aren't the dev/staging/prod axis (e.g. `single-node` vs `ha-3node`, `mariadb-backed` vs `postgres-backed`).

```yaml
# profiles/ha-3node.yaml
replicas: 3
podAntiAffinity:
  required: true
storage:
  storageClass: ssd-replicated
```

```sh
keramos install pg-ha . --profile ha-3node
```

Profiles compose with environments — a `prod` environment can `profile: ha-3node` to apply both layers.

## Tracing

When the merged values surprise you, `--trace` is the answer:

```sh
keramos values my-app --trace
```

```
replicas
  values.yaml                  → 1
  environments.prod.values     → 5
  -f overrides.yaml            → 3
  --set replicas=4             → 4   ← effective

image.tag
  values.yaml                  → "1.27.0"
  --set-string image.tag=dev   → "dev"  ← effective

global.domain
  values.yaml                  → "example.com"
                                          ← effective (no overrides)
```

`--trace` makes long debugging sessions short.

## Working with secrets

`values.yaml` is plain text. Don't put secrets in it. Patterns that work:

- **External secret references**: `existingSecret: my-app-creds` and templates `valueFrom.secretKeyRef.name: ${values.existingSecret}`.
- **`--set-file`** with a secret file at install time: `--set-file tlsCert=@./tls.crt`.
- **External secret operators**: ExternalSecrets, Sealed Secrets, vault-injector — the package only references the materialised Secret name.

For ad-hoc generation (the package wants to materialise a Secret with a default password): use the `randAlphaNum` / `genCA` / `genSelfSignedCert` template functions and let users override via `existingSecret` for production.
