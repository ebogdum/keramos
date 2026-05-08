# Expression syntax

Keramos templates are YAML files with `${...}` expressions interpolated into them. The expression engine is independent of YAML parsing — expressions are resolved first, then the resulting text is parsed as YAML.

This page covers how an expression is structured: literals, paths, function calls, pipelines, and the engine's constraints. For the function catalogue, see [Function reference](functions.md). For control flow (`$if`, `$each`, `$switch`, `$include`), see [Control flow](control-flow.md).

## The `${...}` form

Anything between `${` and `}` is an expression. The whole `${...}` token is replaced with the expression's evaluated value at render time.

**Input template:**

```yaml
metadata:
  name: ${values.name}-config
```

**Values:**

```yaml
name: my-app
```

**Output:**

```yaml
metadata:
  name: my-app-config
```

If an expression returns a non-string value (number, list, map), it's embedded structurally — the substituted value becomes the YAML node, not a string scalar:

**Input:**

```yaml
metadata:
  labels: ${values.labels}
```

**Values:**

```yaml
labels:
  app: my-app
  tier: backend
```

**Output:**

```yaml
metadata:
  labels:
    app: my-app
    tier: backend
```

For string-form embedding (e.g. inside a `data:` field of a ConfigMap that expects a YAML/JSON string), pipe through `toYaml` or `toJson`:

```yaml
data:
  config.yaml: |
    ${values.config | toYaml | nindent 4}
```

## Root namespaces

A path starts with one of four root namespaces (lowercase, no leading dot):

| Namespace | Holds | Example |
|---|---|---|
| `values` | merged values map | `${values.image.tag}` |
| `release` | release identity | `${release.name}` |
| `package` | package metadata from `keramos.yaml` | `${package.version}` |
| `capabilities` | cluster info | `${capabilities.kubeVersion.Major}` |

A path that doesn't start with one of these is treated as `values.<path>`:

```yaml
${replicas}        # equivalent to ${values.replicas}
${image.tag}       # equivalent to ${values.image.tag}
```

The explicit-namespace form is recommended for readability.

### `release` keys

| Path | Meaning |
|---|---|
| `release.name` | the release name |
| `release.namespace` | install namespace |
| `release.revision` | integer revision |
| `release.isInstall` / `release.isUpgrade` / `release.isRollback` | bool |
| `release.service` | always `"keramos"` |

### `package` keys

| Path | Meaning |
|---|---|
| `package.name` | from `keramos.yaml` |
| `package.version` | from `keramos.yaml` |
| `package.appVersion` | from `keramos.yaml` |
| `package.apiVersion` | always `"keramos/v1"` |

### `capabilities` keys

| Path | Meaning |
|---|---|
| `capabilities.kubeVersion.Major` | cluster's major version |
| `capabilities.kubeVersion.Minor` | cluster's minor version |
| `capabilities.kubeVersion.Version` | full version string |
| `capabilities.apiVersions.has("apps/v1")` | check if a GVK exists in the cluster |

## Path indexing

Numeric segments index into a slice. Both `arr.0` and `arr[0]` work.

**Values:**

```yaml
ports:
  - name: http
    number: 80
  - name: https
    number: 443
```

**Input:**

```yaml
first: ${values.ports.0.name}
second: ${values.ports[1].number}
```

**Output:**

```yaml
first: http
second: 443
```

## Literals

Used as function arguments and in conditions:

| Form | Examples |
|---|---|
| String | `"hello"`, `'literal'` |
| Integer | `0`, `42`, `-7` |
| Float | `3.14`, `-0.5` |
| Boolean | `true`, `false` |
| Null | `null`, `nil` |

**Tip:** prefer single quotes inside expressions. YAML readers do not strip backslash-escaped double quotes consistently:

```yaml
ok:    ${values.tag | default 'latest'}      # → latest
ok:    ${"hello" | upper}                    # → HELLO
broken: ${values.tag | default \"latest\"}   # output may include literal backslashes
```

## Pipelines: the only way to compose with paths

The `|` operator chains calls. The value to the **left** of `|` becomes the **first argument** of the function call to the right; subsequent tokens after the function name become **additional literal arguments**.

```yaml
${values.name | upper}
# = upper(values.name)  → "HELLO"

${values.tag | default 'latest'}
# = default(values.tag, 'latest')  → "latest" if tag is empty/nil

${values.replicas | mul 2}
# = mul(values.replicas, 2)  → twice the replica count
```

Multi-step pipelines compose left-to-right:

```yaml
${values.name | lower | quote}
# = quote(lower(values.name))  → "hello"
```

## Function calls without pipelines

When a function takes only literal arguments (no path lookups), call it space-style or paren-style:

```yaml
${add 2 3}                   # → 5
${printf "%s-%d" "x" 7}      # → x-7
${"hello" | upper}           # → HELLO  (literal as pipeline input)
```

## **Important: function arguments are literals, not paths**

Keramos's expression parser treats every argument after a function name as a **literal token** (string, number, bool, or null). It does **not** re-evaluate arguments as paths. So:

```yaml
${add values.a values.b}     # ✗ both args are literal strings → error
${eq values.x "prod"}        # ✗ values.x is the literal string "values.x"
${printf "%s" values.name}   # ✗ outputs literal "values.name"
```

The way to use a path's value with a function is to put it on the **left** of the pipe:

```yaml
${values.a | mul 2}                  # ✓ values.a is the resolved value, 2 is literal
${values.name | printf "name=%s"}    # ✗ unhelpful — values.name becomes the format string
```

For operations that need *two* path values, keramos's expression engine cannot do it in one expression. Pre-compute a value into a partial:

**`templates/_helpers.yaml`:**

```yaml
total: ${values.shards | mul values.replicasPerShard}     # ✗ won't work
```

Instead, do the computation in the values themselves, or in the YAML structure:

```yaml
# values.yaml
shards: 3
replicasPerShard: 4
total: 12     # computed by the operator at value-authoring time

# template
spec:
  parallelism: ${values.total}
```

This is a real engine constraint, not a doc oversight. Plan accordingly.

## Truthy `$if` evaluation

For conditionals, `$if` evaluates the expression's result for truthiness:

```yaml
$if: ${values.ingress.enabled}        # truthy when bool true / non-empty / non-zero
```

Truthy/falsy rules:

| Value | Truthy? |
|---|---|
| `true` | yes |
| `false` | no |
| non-zero number | yes |
| `0` | no |
| non-empty string | yes |
| `""` | no |
| `"false"` | no (string `"false"` treated as falsy) |
| non-empty list/map | yes |
| empty list/map | no |
| `nil` (missing key) | no |

For string-equality conditionals, keramos does not have a runtime `eq` function. Either:

- Restructure as a flag in values (`values.isProd: true`) and use `$if: ${values.isProd}`.
- Use `$switch` (which does compare strings) — see [Control flow](control-flow.md).

```yaml
# Cleanest pattern: a discriminator key
$if: ${values.isProd}
$then:
  resources: { requests: { cpu: 500m, memory: 1Gi } }
$else:
  resources: { requests: { cpu: 100m, memory: 128Mi } }
```

```yaml
# Or use $switch on a string:
$switch: ${values.env}
$cases:
  prod:
    resources: { requests: { cpu: 500m, memory: 1Gi } }
  staging:
    resources: { requests: { cpu: 200m, memory: 512Mi } }
$default:
  resources: { requests: { cpu: 100m, memory: 128Mi } }
```

## Quotes and escaping

Strings inside the expression are double- or single-quoted. Use **single quotes** when in doubt — they don't conflict with YAML's double-quote scalars.

To embed a literal `${` in YAML output (so it's not interpreted), double the dollar sign:

```yaml
data:
  template: '$${VAR}'        # rendered as ${VAR}
```

## Nil handling

| Operation | Behaviour |
|---|---|
| `values.x.y.z` where `y` is missing | returns nil |
| `values.x | upper` where `values.x` is nil | returns "" |
| `values.x | default 'fallback'` where x is nil/empty | returns "fallback" |
| `add nil 1` | error — math rejects nil |
| `empty nil` | true |
| `$if: ${values.x}` where x is missing | branch evaluates falsy |

The expression engine is forgiving with paths and string operations; arithmetic and type-strict functions raise errors so silent wrong outputs don't slip through.

## Output behaviour

| Return type | Output |
|---|---|
| string | inserted verbatim into the YAML scalar |
| number / bool | converted to string ("3", "true") |
| nil | rendered as empty string |
| list / map | embedded structurally — the substituted value replaces the YAML node |

Embedding a structured value works directly — no `toYaml | nindent` needed when the value should *be* the YAML node:

**Input:**

```yaml
spec:
  template:
    metadata:
      labels: ${values.labels}
```

**Values:**

```yaml
labels:
  app: my-app
  tier: backend
```

**Output:**

```yaml
spec:
  template:
    metadata:
      labels:
        app: my-app
        tier: backend
```

Use `toYaml | nindent` only when the destination expects a *string* containing YAML (e.g. a ConfigMap `data:` field):

```yaml
data:
  config.yaml: |
    ${values.config | toYaml | nindent 4}
```

Here the `|` block-scalar marker tells YAML "the contents below are a string" — and the rendered string IS valid YAML, just embedded as a string in the larger document.

## End-to-end examples that work

### Computed name

**Input:**

```yaml
metadata:
  name: ${release.name}-${values.role}
```

**Values:** `{ role: api }`, **`--release-name hello`**

**Output:**

```yaml
metadata:
  name: hello-api
```

### Default + required

```yaml
spec:
  replicas: ${values.replicas | default 1}
  serviceAccountName: ${values.sa | required 'sa must be set'}
```

### Conditional value via ternary

```yaml
spec:
  type: ${values.exposed | ternary 'LoadBalancer' 'ClusterIP'}
```

### Pipeline with one path and a literal

**Input:**

```yaml
spec:
  parallelism: ${values.totalWorkers | div 4}
```

**Values:** `{ totalWorkers: 12 }`

**Output:**

```yaml
spec:
  parallelism: 3
```

### Embedding a map

**Input:**

```yaml
metadata:
  annotations: ${values.podAnnotations | toYaml | nindent 4}
```

**Values:**

```yaml
podAnnotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "9100"
```

**Output:**

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "9100"
```

For control-flow constructs (`$if`, `$each`, `$switch`) and named partials (`$include`), see [Control flow](control-flow.md).
