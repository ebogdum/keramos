# Layers in templates

When a package has layers, every layer's `values.yaml` is deep-merged with the parent's `values.yaml` into a single flat values map. **Both the parent's templates and the layer's templates see the same merged map** at `${values.*}` — there is no per-layer namespace separation. What the parent can do is *override* a layer's contribution via the `layers.<layer-name>.<key>` block in its own values.yaml; the merger consumes that block and applies the overrides before producing the final flat map.

For composition mechanics (how templates merge, how to declare layers), see the [Layers guide](../guides/layers.md).

## How layer values reach the merged map

Imagine a parent that pulls in a `redis` layer:

**Layer (`redis/values.yaml`):**

```yaml
password: from-layer
port: 6379
maxConnections: 1000
```

**Parent (`my-app/keramos.yaml`):**

```yaml
apiVersion: keramos/v1
name: my-app
version: 1.0.0

layers:
  - name: redis
    source: ../redis
```

**Parent (`my-app/values.yaml`):**

```yaml
replicas: 3
layers:
  redis:
    password: from-parent
```

After merge, the values context every template sees is:

```yaml
replicas: 3
password: from-parent          # parent's `layers.redis.password` overrode the layer's value
port: 6379                     # from the layer
maxConnections: 1000           # from the layer
```

Both `my-app/templates/...` and `redis/templates/...` resolve `${values.password}` to `from-parent`, `${values.port}` to `6379`, and `${values.replicas}` to `3`. The `layers` key in the parent's `values.yaml` is consumed by the merger and is not present in the resulting context.

## Forcing precedence with the `!` prefix

Sometimes the parent's `values.yaml` has a section that should win over a child layer's value, regardless of merge order. Prefix the key with `!`:

```yaml
# parent values.yaml
layers:
  redis:
    "!password": forced-by-parent     # always wins, even if a profile re-overrides
```

The `!` marker is stripped from the key during merge; the value is applied at the highest precedence level. Use it sparingly — most overrides don't need it.

## What does NOT exist

A few patterns that look reasonable but aren't supported:

- There is no `${.Layer.Name}` / `${layer.name}` / `${.Layer.Path}` namespace exposing the current layer's identity to its templates.
- There is no `${.Layers.<name>.<key>}` / `${.Subcharts.<name>.<key>}` accessor letting the parent reach into a layer's pre-merged values. Use `${values.<merged-key>}` after the merger has produced the flat map.
- There is no per-layer value scoping at template time. Layers do not see "their own" values separately from siblings — every key from every layer's `values.yaml` ends up in the same flat map, with parent overrides applied.

If you need a layer to see only its own keys without collisions from siblings, namespace them inside the layer's own values:

```yaml
# redis/values.yaml
redis:
  password: from-layer
  port: 6379
```

Then templates inside the layer access `${values.redis.password}`. The parent overrides via:

```yaml
# parent/values.yaml
layers:
  redis:
    redis:
      password: from-parent
```

This is just a YAML naming convention, not a special engine feature.

## The `global` convention

A frequently-used convention is to put cluster-wide settings under `values.global`:

```yaml
# parent values.yaml
global:
  imageRegistry: registry.example.com
  domain: example.com
```

Layer templates read them as `${values.global.imageRegistry}`. Because all values merge flat, `values.global` is automatically visible everywhere — no special scoping rules needed.

## A complete example

**Layer template (`redis/templates/serviceaccount.yaml`):**

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${release.name}-redis
  labels:
    app.kubernetes.io/name: redis
    app.kubernetes.io/instance: ${release.name}
```

**Parent template (`my-app/templates/deployment.yaml`):**

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
              value: ${values.password | quote}
```

**Parent values (`my-app/values.yaml`):**

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
apiVersion: v1
kind: ServiceAccount
metadata:
  name: hello-redis
  labels:
    app.kubernetes.io/name: redis
    app.kubernetes.io/instance: hello
---
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
              value: "hunter2"
```

## Library layers (no installable templates)

A package with `type: library` in its `keramos.yaml` cannot be installed standalone. Library packages typically ship only partials in `_helpers.yaml` that consuming packages `${include}`. This is the right place to put organisation-wide partials (common labels, common pod-spec scaffolding, common ingress wiring) so every consuming package gets the same shape with the same updates.

```yaml
# library keramos.yaml
apiVersion: keramos/v1
name: org-base
version: 1.0.0
type: library
```

```yaml
# org-base/templates/_helpers.yaml — partials only
common.labels:
  app.kubernetes.io/name: ${package.name}
  app.kubernetes.io/instance: ${release.name}
  app.kubernetes.io/version: ${package.version}
  app.kubernetes.io/managed-by: keramos
```

Consumers:

```yaml
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
