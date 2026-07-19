# Layers in templates

When a package pulls in layers, every layer's `values.yaml` is deep-merged with
the parent's into a single **flat** values map. Both the parent's templates and
the layers' templates read that same map at `${values.*}` — there is no
per-layer namespace. What the parent controls is which values win, using the
`layers.<name>` block in its own `values.yaml`.

For how you declare and compose layers, see the [Layers guide](../guides/layers.md).

## How layer values reach the merged map

A parent that pulls in a `redis` layer:

**Layer — `redis/values.yaml`:**

```yaml
password: from-layer
port: 6379
```

**Parent — `my-app/keramos.yaml`:**

```yaml
apiVersion: keramos/v1
name: my-app
version: 1.0.0
layers:
  - name: redis
    source: ../redis
```

**Parent — `my-app/values.yaml`:**

```yaml
replicas: 3
layers:
  redis:
    password: from-parent
```

After the merge, every template sees:

```yaml
replicas: 3
password: from-parent   # parent's layers.redis.password overrode the layer
port: 6379              # from the layer
```

Both `my-app/templates/*` and `redis/templates/*` resolve `${values.password}`
to `from-parent`, `${values.port}` to `6379`, and `${values.replicas}` to `3`.
The `layers` key is consumed by the merger and is **not** present in the values
templates see.

Precedence: deeper layers merge first, shallower layers next, and the parent
last (highest). A parent key therefore wins over a layer key of the same name.

## Overriding a layer's values

Put overrides for a layer under `layers.<name>` in the parent's `values.yaml`;
the merger applies them to that layer's values before flattening. In the example
above, `layers.redis.password` replaced the layer's `password`.

## Forcing precedence with `!`

To make a key win regardless of merge order — even over a profile that would
otherwise re-override it — prefix it with `!`:

```yaml
# parent values.yaml
layers:
  redis:
    "!password": forced-by-parent
```

The `!` is stripped during merge and the value is applied at the highest
precedence. Use it sparingly.

## What does not exist

- No `${layer.name}` / `${.Layer.Path}` namespace exposing the current layer's
  identity to its templates.
- No `${.Layers.<name>.<key>}` accessor to reach a layer's pre-merge values.
  Read `${values.<merged-key>}` after the merge.
- No per-layer value scoping at render time. Every layer's keys land in the same
  flat map.

If a layer needs keys that won't collide with siblings, namespace them inside
the layer's own values (`redis: { password: ... }`, read as
`${values.redis.password}`) and override via `layers.redis.redis.password`. This
is a YAML convention, not an engine feature.

## The `global` convention

Cluster-wide settings conventionally live under `values.global`:

```yaml
# parent values.yaml
global:
  imageRegistry: registry.example.com
```

Because everything merges flat, `values.global` is visible everywhere — layer
templates read `${values.global.imageRegistry}` with no special scoping.

## A complete example

**Layer — `redis/templates/serviceaccount.yaml`:**

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${release.name}-redis
  labels:
    app.kubernetes.io/name: redis
    app.kubernetes.io/instance: ${release.name}
```

**Parent — `my-app/templates/deployment.yaml`:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${release.name}
spec:
  replicas: ${values.replicas}
  template:
    spec:
      containers:
        - name: app
          image: ${values.global.imageRegistry}/${values.image.name}:${values.image.tag}
          env:
            - name: REDIS_PASSWORD
              value: ${values.password}
```

**Parent — `my-app/values.yaml`:**

```yaml
replicas: 3
global:
  imageRegistry: registry.example.com
image:
  name: my-app
  tag: 1.4.2
layers:
  redis:
    password: hunter2
```

**Output of `keramos template ./my-app --release-name hello`:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: app
          image: registry.example.com/my-app:1.4.2
          env:
            - name: REDIS_PASSWORD
              value: hunter2
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: hello-redis
  labels:
    app.kubernetes.io/name: redis
    app.kubernetes.io/instance: hello
```

## Library layers

A package with `type: library` in its `keramos.yaml` can't be installed standalone.
A library ships partials in `_helpers.yaml` that consuming packages `$include`,
so every consumer gets the same shape:

```yaml
# org-base/keramos.yaml
apiVersion: keramos/v1
name: org-base
version: 1.0.0
type: library
```

```yaml
# org-base/templates/_helpers.yaml
common.labels:
  app.kubernetes.io/name: ${package.name}
  app.kubernetes.io/instance: ${release.name}
  app.kubernetes.io/managed-by: keramos
```

Consumers add it as a layer and include the partial:

```yaml
# consumer keramos.yaml
layers:
  - name: org-base
    source: oci://ghcr.io/example/org-base
    version: ^1.0.0
```

```yaml
# consumer template
metadata:
  labels:
    $include: common.labels
```

## See also

- [Layers guide](../guides/layers.md) — declaring layers, sources, conditions
- [Control flow](control-flow.md) — `$include` and the directives
- [Expressions](expressions.md) — reading merged values with `${values.*}`
