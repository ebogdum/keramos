# Expression syntax

Keramos templates are YAML files with `${...}` expressions that you interpolate
into them. Expressions are resolved first, then the resulting text is parsed as
YAML.

This page covers how you write an expression: literals, paths, function calls,
and pipelines. For the function catalogue, see
[Function reference](functions.md). For `$if`, `$each`, `$switch`, and
`$include`, see [Control flow](control-flow.md).

## The `${...}` form

Anything between `${` and `}` is an expression. The whole token is replaced with
the expression's value at render time.

**Template:**

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

When the whole scalar is a single `${...}`, the result keeps its native type. A
list or map is embedded structurally — it becomes the YAML node, not a string:

**Template:**

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

Embed a map or list directly like this whenever you want it to *be* the YAML
node. Only pipe through `toYaml` when the destination expects a *string* that
contains YAML — see [Output types](#output-types).

## Root namespaces

A path starts with one of four root namespaces (lowercase, no leading dot):

| Namespace | Holds | Example |
|---|---|---|
| `values` | merged values map | `${values.image.tag}` |
| `release` | release identity | `${release.name}` |
| `package` | metadata from `keramos.yaml` | `${package.version}` |
| `capabilities` | cluster info | `${capabilities.kubeVersion.Version}` |

A path that doesn't start with one of these is treated as `values.<path>`:

```yaml
${replicas}        # same as ${values.replicas}
${image.tag}       # same as ${values.image.tag}
```

The explicit form reads better; prefer it.

### `release` keys

| Path | Meaning |
|---|---|
| `release.name` | release name (`--release-name`, else the package name) |
| `release.namespace` | install namespace (`-n`; empty under `keramos template` if unset) |
| `release.revision` | integer revision (`1` under `keramos template`) |
| `release.isInstall` / `release.isUpgrade` | bool |

### `package` keys

| Path | Meaning |
|---|---|
| `package.name` | from `keramos.yaml` |
| `package.version` | from `keramos.yaml` |
| `package.appVersion` | from `keramos.yaml` |

### `capabilities` keys

`capabilities.kubeVersion.Version` and `.GitVersion` carry the cluster version;
`.Major` / `.Minor` are populated only against a live cluster. See
[Capabilities](capabilities.md) for the full picture.

## Path indexing

A numeric segment indexes into a list. Both `arr.0` and `arr[0]` work.

**Values:**

```yaml
ports:
  - { name: http,  number: 80 }
  - { name: https, number: 443 }
```

**Template:**

```yaml
first:  ${values.ports.0.name}
second: ${values.ports[1].number}
```

**Output:**

```yaml
first: http
second: 443
```

A missing key or an out-of-range index resolves to nil rather than erroring, so
`default` can supply a fallback.

## Literals

You use literals as function arguments and in conditions:

| Form | Examples |
|---|---|
| String | `"hello"`, `'literal'` |
| Integer | `0`, `42`, `-7` |
| Float | `3.14`, `-0.5` |
| Boolean | `true`, `false` |
| Null | `null`, `nil` |

Prefer single quotes inside expressions — they don't collide with YAML's
double-quoted scalars.

```yaml
ok: ${values.tag | default 'latest'}   # → latest
ok: ${"hello" | upper}                 # → HELLO
```

## Pipelines

The `|` operator chains calls. The value to the **left** of `|` becomes the
**first argument** of the function on the right; tokens after the function name
are **additional literal arguments**.

```yaml
${values.name | upper}          # upper(values.name)          → HELLO
${values.tag  | default 'latest'}  # default(values.tag, 'latest')
${values.replicas | mul 2}      # mul(values.replicas, 2)     → doubled
```

Multi-step pipelines compose left to right:

```yaml
${values.name | lower | quote}  # quote(lower(values.name))
```

## Function calls without a pipeline

When every argument is a literal, call a function space-style. The first token
after the name is the value the function acts on:

```yaml
${add 2 3}                # → 5
${printf "%s-%d" "x" 7}   # → x-7
${"hello" | upper}        # → HELLO   (literal piped in)
```

## Arguments are literals, not paths

Every argument after a function name is a **literal token** (string, number,
bool, or null). The engine does **not** re-evaluate an argument as a path. So a
path in argument position is read as a plain string:

```yaml
${eq values.x "prod"}        # values.x is the literal string "values.x"
${printf "%s" values.name}   # prints the literal "values.name"
```

There is also no `eq` / `ne` function, so a comparison like the first line above
silently resolves to nil — see [Control flow](control-flow.md#comparing-strings)
for the patterns that actually compare values.

To feed a path's value into a function, put the path on the **left** of the
pipe:

```yaml
${values.a | mul 2}                # values.a resolved, 2 literal   → correct
${capabilities.kubeVersion.Version | semverCompare '>=1.27.0-0'}
```

An expression can hold only **one** path lookup (the pipeline input). To combine
two values, compute the result in `values.yaml` and read the single key.

## Truthiness

`$if` and functions like `default`, `empty`, and `ternary` judge a value's
truthiness:

| Value | Truthy? |
|---|---|
| `true`, non-zero number, non-empty string | yes |
| `false`, `0`, `nil` (missing key) | no |
| `""` | no |
| `"false"`, `"False"`, `"FALSE"`, `"0"`, `"no"`, `"No"`, `"NO"` | no (falsy strings) |
| non-empty list/map | yes |
| empty list/map | no |

The falsy-string set matters when a value arrives as text (from `--set` or an
env source): `--set flag=false` is falsy, not "non-empty string, therefore
true".

## Nil handling

| Operation | Behaviour |
|---|---|
| `values.x.y` where `y` is missing | nil |
| `values.x \| upper` where `values.x` is nil | `""` |
| `values.x \| default 'v'` where `values.x` is nil/empty | `"v"` |
| `add nil 1` | error — math rejects nil |
| `empty nil` | `true` |

Paths and string functions are forgiving; arithmetic and type-strict functions
error so a silent wrong value can't slip through.

## Optional fields

A field whose value resolves to nil renders as `key: null` by default. To make
the key *disappear* when the value is absent, pipe through `omitempty`:

**Template:**

```yaml
data:
  always: fixed
  maybe: ${values.optional | omitempty}
```

**Values:** `{ optional: "" }`

**Output:**

```yaml
data:
  always: fixed
```

`omitempty` drops the key when the value is empty: nil, `""`, `false`, `0`, or an
empty list/map. A field-level `$if` with no matching branch omits its key the
same way — see [Control flow](control-flow.md#optional-fields).

## Escaping `${`

To emit a literal `${...}` in the output (so keramos does not evaluate it), double
the dollar sign:

**Template:**

```yaml
data:
  template: '$${VAR}'
```

**Output:**

```yaml
data:
  template: ${VAR}
```

## Output types

| Return type | Output |
|---|---|
| string | inserted into the YAML scalar |
| number / bool | stringified (`3`, `true`) |
| nil | empty string |
| list / map | embedded structurally as the YAML node |

Because a whole-scalar `${map}` embeds structurally, you do **not** need
`toYaml` to place a map under a key:

**Template:**

```yaml
metadata:
  annotations: ${values.podAnnotations}
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

Reach for `toYaml | nindent` only when the destination expects a **string** of
YAML, such as a ConfigMap `data:` field. There, `toYaml` serialises the map to
text and `nindent` indents it as a block scalar:

**Template:**

```yaml
data:
  config.yaml: |
    ${values.config | toYaml | nindent 4}
```

The result is a string containing YAML, not a nested mapping — which is exactly
what a `data:` value must be.

## See also

- [Control flow](control-flow.md) — `$if`, `$each`, `$switch`, `$include`
- [Function reference](functions.md) — the full function catalogue
- [Capabilities](capabilities.md) — the `capabilities` namespace and `lookup`
