---
title: "Control flow"
parent: "Templates"
---
{% raw %}
# Control flow

Keramos's control flow is YAML-native: you write conditionals, loops, and case
selection as `$`-prefixed map keys, not as text-level directives. Control flow
that lives in the YAML structure always parses and never breaks indentation.

The directives are `$if` / `$then` / `$else`, `$each` / `$as` / `$yield`,
`$switch` / `$cases` / `$default`, and `$include`. This page shows each with
input → output pairs.

## One directive per map

A single map is governed by at most one of `$if`, `$each`, or `$switch`, checked
in that order. When one fires, it **replaces the map it sits in** — sibling keys
in that map are discarded. The one exception is a bare `$if` (no `$then` /
`$else`) that is truthy: it keeps its siblings and only removes the `$if` key.

So to gate or transform a single field, put the directive **inside that field's
value**, not next to it. The examples below follow that rule, and
[Combining directives](#combining-directives) shows how to nest them.

## `$if` — conditional inclusion

A document or sub-tree renders when the `$if` expression is truthy (see
[Truthiness](expressions.md#truthiness)).

### Drop a whole document

**Template:**

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

**Values:** `{ ingress: { enabled: false } }` → nothing renders.

**Values:** `{ ingress: { enabled: true, host: my-app.example.com } }`

**Output:**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: hello
spec:
  rules:
    - host: my-app.example.com
```

### Drop a single field

A bare `$if` governs the **whole map it sits in**. To omit just one field, nest
the `$if` under that field's key with a `$then`:

**Template:**

```yaml
spec:
  containers:
    - name: app
      image: ${values.image}
      ports:
        $if: ${values.metrics.enabled}
        $then:
          - { containerPort: 9100, name: metrics }
```

**Values:** `{ image: app:1, metrics: { enabled: false } }`

**Output:**

```yaml
spec:
  containers:
    - name: app
      image: app:1
```

The container stays; only `ports` drops. A `$if`/`$then` with no `$else` that
evaluates false omits its key.

### `$if` / `$then` / `$else`

For either-or branches:

**Template:**

```yaml
$if: ${values.highMem}
$then:
  resources:
    requests: { cpu: 500m, memory: 1Gi }
$else:
  resources:
    requests: { cpu: 100m, memory: 128Mi }
```

**Values:** `{ highMem: true }`

**Output:**

```yaml
resources:
  requests: { cpu: 500m, memory: 1Gi }
```

`$then` is required when you use branches; `$else` is optional. With `$then`
present, the map is replaced by the chosen branch.

## Comparing strings

There is no `eq` / `ne` function, and function arguments are literals rather
than paths (see [Expressions](expressions.md#arguments-are-literals-not-paths)).
Writing `$if: ${eq values.env "prod"}` does **not** compare anything — `eq` is
not a function, so the whole expression silently resolves to nil (falsy) and the
block never renders, with no error.

Compare with `$switch`, or with a boolean flag in values:

```yaml
# Switch on the string
$switch: ${values.env}
$cases:
  prod:
    resources: { requests: { cpu: 500m, memory: 1Gi } }
$default:
  resources: { requests: { cpu: 100m, memory: 128Mi } }
```

```yaml
# Or use a discriminator flag and a plain $if
$if: ${values.isProd}
$then:
  resources: { requests: { cpu: 500m, memory: 1Gi } }
```

## `$each` — looping

`$each` iterates a list or map and always produces a **list** of yielded items.
The loop variable defaults to `$item`; rename it with `$as`.

### Over a list

**Template:**

```yaml
spec:
  ports:
    $each: ${values.exposedPorts}
    $as: port
    $yield:
      name: ${port.name}
      port: ${port.port}
      targetPort: ${port.targetPort}
      protocol: ${port.protocol | default "TCP"}
```

**Values:**

```yaml
exposedPorts:
  - { name: http,  port: 80,  targetPort: 8080 }
  - { name: https, port: 443, targetPort: 8443, protocol: TCP }
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
```

`$each` requires a `$yield` block. When the collection is **missing**, the field
is omitted entirely; when it is an explicit empty list `[]`, you get an empty
list.

### Over a map

Iterating a map binds `$item.key` and `$item.value` (also available as `$key`
and `$value`), sorted by key. The result is still a **list**:

**Template:**

```yaml
env:
  $each: ${values.config}
  $yield:
    name: ${$item.key}
    value: ${$item.value | quote}
```

**Values:** `{ config: { LOG_LEVEL: info, TIMEOUT: "30" } }`

**Output:**

```yaml
env:
  - name: LOG_LEVEL
    value: '"info"'
  - name: TIMEOUT
    value: '"30"'
```

You cannot build a *map* with `$each` — it yields a list, and `${...}` inside a
map **key** is never substituted. When you already have a map in values and want
it as a map in output, embed it directly instead:

**Template:**

```yaml
data: ${values.config}
```

**Output:**

```yaml
data:
  LOG_LEVEL: info
  TIMEOUT: "30"
```

## `$switch` — case selection

`$switch` stringifies its value and picks the matching entry in `$cases`.
`$default` is optional. The matched branch replaces the map.

**Template:**

```yaml
spec:
  $switch: ${values.strategy}
  $cases:
    rolling:
      type: RollingUpdate
      rollingUpdate: { maxUnavailable: 25%, maxSurge: 1 }
    recreate:
      type: Recreate
  $default:
    type: RollingUpdate
```

**Values:** `{ strategy: recreate }`

**Output:**

```yaml
spec:
  type: Recreate
```

When nothing matches and there is no `$default`, the field is omitted.

## `$include` — inserting partials

A partial is a named block in a `_*.yaml` file under `templates/` (conventionally
`_helpers.yaml`). `$include` splices it into the YAML structure.

**`templates/_helpers.yaml`:**

```yaml
common.labels:
  app.kubernetes.io/name: ${package.name}
  app.kubernetes.io/instance: ${release.name}
```

**`templates/service.yaml`:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: ${release.name}
  labels:
    $include: common.labels
```

**Output:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: hello
  labels:
    app.kubernetes.io/name: my-app
    app.kubernetes.io/instance: hello
```

If the partial is a map, its keys merge into the parent; if a list, the list is
inserted; if a scalar, it replaces the `$include` key.

The `include` **function** is the expression-form equivalent — it renders a
partial to a *string*, useful inside a scalar:

**Template:**

```yaml
metadata:
  annotations:
    ns: ${include "namespace.fullname"}
```

**Partial:** `namespace.fullname: ${release.namespace}/${release.name}`

**Output (`-n default`):**

```yaml
metadata:
  annotations:
    ns: |
      default/hello
```

## Combining directives

Directives nest across **different keys**. Because a directive replaces its own
map, you combine them by placing each inside a distinct field's value. Here an
`$each` builds the container list and each item nests an `$if` to add ports
conditionally:

**Template:**

```yaml
spec:
  containers:
    $each: ${values.containers}
    $yield:
      name: ${$item.name}
      image: ${$item.image}
      ports:
        $if: ${$item.metrics}
        $then:
          - { name: metrics, containerPort: 9100 }
```

**Values:**

```yaml
containers:
  - { name: app,     image: app:1,  metrics: true }
  - { name: sidecar, image: side:1, metrics: false }
```

**Output:**

```yaml
spec:
  containers:
    - name: app
      image: app:1
      ports:
        - name: metrics
          containerPort: 9100
    - name: sidecar
      image: side:1
```

The engine resolves includes first, then control flow, then `${...}`
substitution.

## See also

- [Expressions](expressions.md) — `${...}` syntax, truthiness, optional fields
- [Function reference](functions.md) — `default`, `quote`, and the rest
- [Hooks](hooks.md) — the same directives inside lifecycle hooks
{% endraw %}
