# Control flow

Keramos's control flow is **YAML-native**. Conditionals, loops, and case selection are expressed as `$`-prefixed map keys in the template — not as text-level template directives. This is intentional: control flow that's part of the YAML structure is always parseable, never breaks indentation, and copy-pastes cleanly.

The directives are: `$if` / `$then` / `$else`, `$each` / `$as` / `$yield`, `$switch` / `$cases` / `$default`, and `$include`. This page covers each with input/output pairs.

## `$if` — conditional inclusion

A document or sub-tree is rendered when the `$if` expression is truthy. Truthy means: non-empty string, non-zero number, true, non-empty list/map. Falsy means everything else (empty, zero, nil, false).

### Document-level: drop the whole document

**Input:**

```yaml
$if: ${values.ingress.enabled}

apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ${release.name}
spec:
  rules:
    - host: ${values.ingress.host}
```

**Values: `{ ingress: { enabled: false } }`**

**Output: (document omitted entirely; nothing rendered)**

**Values: `{ ingress: { enabled: true, host: my-app.example.com } }`**

**Output:**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
spec:
  rules:
    - host: my-app.example.com
```

### Sub-tree: drop the immediate sub-block

When `$if` is on a non-root map, the parent decides whether the *block* is kept. The block is removed from its parent if the condition is falsy.

**Input:**

```yaml
spec:
  template:
    spec:
      containers:
        - name: app
          image: ${values.image}
          $if: ${values.metrics.enabled}
          ports:
            - containerPort: 9100
              name: metrics
```

The `$if` here governs the `ports` field on the container.

### `$if` / `$then` / `$else`

For "either-or" branches:

**Input:**

```yaml
$if: ${eq values.env "prod"}
$then:
  resources:
    requests: { cpu: 500m, memory: 1Gi }
    limits:   { memory: 2Gi }
$else:
  resources:
    requests: { cpu: 100m, memory: 128Mi }
    limits:   { memory: 512Mi }
```

**Values: `{ env: prod }`**

**Output:**

```yaml
resources:
  requests: { cpu: 500m, memory: 1Gi }
  limits:   { memory: 2Gi }
```

`$then` is required when both branches exist. `$else` is optional. When the parent is a map and `$if` is falsy without `$else`, the entire map node is dropped.

## `$each` — looping over a list or map

`$each` iterates a list or map and produces a list of yielded items. The bound variable defaults to `$item`; rename with `$as`.

### Iterating a list

**Input:**

```yaml
spec:
  ports:
    $each: ${values.exposedPorts}
    $yield:
      name: ${$item.name}
      port: ${$item.port}
      targetPort: ${$item.targetPort}
      protocol: ${$item.protocol | default "TCP"}
```

**Values:**

```yaml
exposedPorts:
  - { name: http,  port: 80,  targetPort: 8080 }
  - { name: https, port: 443, targetPort: 8443, protocol: TCP }
  - { name: udp,   port: 53,  targetPort: 53, protocol: UDP }
```

**Output:**

```yaml
spec:
  ports:
    - name: http
      port: 80
      targetPort: 8080
      protocol: TCP
    - name: https
      port: 443
      targetPort: 8443
      protocol: TCP
    - name: udp
      port: 53
      targetPort: 53
      protocol: UDP
```

> **Note**: function arguments inside `${...}` are parsed as **literals** (strings, numbers, bools, nil). They are not re-evaluated as paths. So `${$item.targetPort | default $item.port}` will not fall back to `$item.port` — `$item.port` is treated as the literal string `"$item.port"`. To compute defaults that reference other fields, ensure the input data already contains the desired value, or use `$if`/`$switch` to choose the field.

### Renaming the loop variable with `$as`

```yaml
spec:
  ports:
    $each: ${values.exposedPorts}
    $as: port
    $yield:
      name: ${port.name}
      port: ${port.port}
```

`$as` is useful when the body needs both the loop variable and unrelated `${$item}`-named values from elsewhere.

### Iterating a map

`$each` over a map binds two names: `$item.key` and `$item.value`. (With `$as foo`, those become `foo.key` and `foo.value`.)

**Input:**

```yaml
data:
  $each: ${values.config}
  $yield:
    ${$item.key}: ${$item.value | quote}
```

Wait — `data:` is a map, not a list, so `$each` for map-shaped output works differently. For a map output, prefer this pattern:

**Input:**

```yaml
data:
  $each: ${values.config}
  $as: kv
  $yield:
    key: ${kv.key}
    value: ${kv.value | quote}
```

The above produces a list of `{key, value}` pairs. To produce an actual map, structure the surrounding YAML so each yielded item is an inline `{ "key": value }` and the engine merges them. The simplest pattern is to put the map inside a ConfigMap-style structure where the engine treats it naturally:

```yaml
apiVersion: v1
kind: ConfigMap
data:
  $each: ${values.config}
  $yield: { ${$item.key}: ${$item.value | quote} }
```

For complex map composition, see the `dict` and `merge` functions in [Functions](functions.md#misc).

## `$switch` — case selection

`$switch` evaluates an expression and picks the matching `$case`. `$default` is optional.

**Input:**

```yaml
spec:
  $switch: ${values.deployment.strategy}
  $cases:
    rolling:
      strategy:
        type: RollingUpdate
        rollingUpdate:
          maxUnavailable: 25%
          maxSurge: 1
    recreate:
      strategy:
        type: Recreate
    blue-green:
      strategy:
        type: RollingUpdate
        rollingUpdate:
          maxUnavailable: 0
          maxSurge: 100%
  $default:
    strategy:
      type: RollingUpdate
```

**Values: `{ deployment: { strategy: blue-green } }`**

**Output:**

```yaml
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 100%
```

When no case matches and `$default` is absent, the whole `$switch` block resolves to nothing (the parent map gets the keys it had outside `$switch`/`$cases`/`$default`).

## `$include` — inserting partials

A partial is an entry in `_helpers.yaml` (or any `_*.yaml` file under `templates/`). Use `$include` to splice a partial into the YAML structure.

**`templates/_helpers.yaml`:**

```yaml
common.labels:
  app.kubernetes.io/name: ${package.name}
  app.kubernetes.io/instance: ${release.name}
  app.kubernetes.io/version: ${package.version}
  app.kubernetes.io/managed-by: keramos
```

**`templates/deployment.yaml`:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${release.name}
  labels:
    $include: common.labels
spec:
  template:
    metadata:
      labels:
        $include: common.labels
```

**Output:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello
  labels:
    app.kubernetes.io/name: my-app
    app.kubernetes.io/instance: hello
    app.kubernetes.io/version: 1.0.0
    app.kubernetes.io/managed-by: keramos
spec:
  template:
    metadata:
      labels:
        app.kubernetes.io/name: my-app
        app.kubernetes.io/instance: hello
        app.kubernetes.io/version: 1.0.0
        app.kubernetes.io/managed-by: keramos
```

`$include` splices the partial in place. If the partial is a map, its keys merge into the parent; if a list, the list is inserted; if a scalar, the scalar replaces the `$include` key entirely.

The `include` *function* (in `${...}` expressions) is the scalar-style equivalent — useful when you want a partial as a string.

**Input:**

```yaml
metadata:
  annotations:
    namespace: ${include "namespace.fullname"}
```

**Partial `namespace.fullname`:**

```yaml
namespace.fullname: ${release.namespace}/${release.name}
```

**Output:**

```yaml
metadata:
  annotations:
    namespace: default/hello
```

## Combining directives

The directives nest. A `$switch` can hold `$cases` containing `$if` blocks; a `$yield` body can use `$include`; an `$each` body can use `$switch`. The engine applies them in order: includes first (so the partial body is resolved into place), then control flow, then `${...}` substitution.

```yaml
spec:
  containers:
    $each: ${values.containers}
    $yield:
      name: ${$item.name}
      image: ${$item.image}
      $if: ${$item.metrics.enabled}
      ports:
        - { name: metrics, containerPort: 9100 }
      $switch: ${$item.shape}
      $cases:
        small:
          resources:
            requests: { cpu: 50m,  memory: 64Mi }
        medium:
          resources:
            requests: { cpu: 200m, memory: 256Mi }
        large:
          resources:
            requests: { cpu: 500m, memory: 1Gi }
      $default:
        resources: {}
```

## Truthiness rules

| Value | Truthy? |
|---|---|
| `true` | yes |
| `false` | no |
| non-zero number | yes |
| `0` | no |
| non-empty string | yes |
| `""` | no |
| `"false"` | no (string `"false"` treated specially) |
| non-empty list/map | yes |
| empty list/map | no |
| `nil` (missing key) | no |

These rules apply to `$if` evaluation. Inside `${...}` expressions, the same logic governs `default`, `empty`, and `ternary`.
