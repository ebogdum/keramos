# `values.schema.json` Reference

`values.schema.json` is an optional file at the package root that declares the expected shape of `values.yaml` (and any merged overrides). When present, keramos validates the **effective values** against the schema before rendering. This catches typos (`replicaa: 3`), wrong types (`replicas: "three"`), missing required keys, and out-of-range numbers — long before the manifest reaches the cluster.

The file is a standard JSON Schema Draft 2020-12 document. Keramos supports a practical subset; this page enumerates exactly what is and is not honoured, and gives examples of each construct.

---

## Where validation runs

- `keramos lint` validates as part of its pass.
- `keramos install`, `keramos upgrade`, `keramos template`, `keramos diff`, `keramos plan` validate before render.
- `keramos config` walks the schema interactively to produce a values file from prompts.

A schema violation aborts the operation with a precise message: which path, what was found, what was expected.

---

## Supported keywords

### Type primitives

```json
{
  "type": "object",
  "properties": {
    "replicas":  { "type": "integer" },
    "image":     { "type": "object" },
    "args":      { "type": "array" },
    "name":      { "type": "string" },
    "enabled":   { "type": "boolean" },
    "weight":    { "type": "number" },
    "extra":     { "type": "null" }
  }
}
```

A `type` may also be a list to permit unions: `"type": ["string", "null"]`.

### Required fields

```json
{
  "type": "object",
  "required": ["name", "image"],
  "properties": {
    "name":  { "type": "string" },
    "image": { "type": "object" }
  }
}
```

A missing required field at any nested level is reported with its full dotted path.

### String constraints

| Keyword | Meaning |
|---|---|
| `minLength` / `maxLength` | inclusive length bounds |
| `pattern` | RE2 regular expression (Go regexp; not full PCRE) |
| `enum` | allowed values |
| `const` | the only allowed value |
| `format` | well-known formats: `email`, `uri`, `uuid`, `ipv4`, `ipv6`, `date`, `date-time`, `hostname` |

```json
{
  "type": "string",
  "pattern": "^[a-z][a-z0-9-]{0,62}$",
  "format": "hostname",
  "minLength": 1
}
```

### Numeric constraints

| Keyword | Meaning |
|---|---|
| `minimum` / `maximum` | inclusive bounds |
| `exclusiveMinimum` / `exclusiveMaximum` | exclusive bounds |
| `multipleOf` | must be evenly divisible |

```json
{
  "type": "integer",
  "minimum": 1,
  "maximum": 100,
  "multipleOf": 1
}
```

### Array constraints

| Keyword | Meaning |
|---|---|
| `items` | schema applied to every element |
| `minItems` / `maxItems` | length bounds |
| `uniqueItems` | reject duplicates (deep-equal comparison) |

```json
{
  "type": "array",
  "items": { "type": "string", "format": "uri" },
  "minItems": 1,
  "uniqueItems": true
}
```

### Object constraints

| Keyword | Meaning |
|---|---|
| `properties` | per-key schemas |
| `additionalProperties` | `false` rejects unknown keys; a schema validates them |
| `patternProperties` | per-regex schemas applied to matching keys |
| `minProperties` / `maxProperties` | key-count bounds |
| `dependentRequired` | when key X exists, also require Y |

```json
{
  "type": "object",
  "properties": {
    "tls":      { "type": "boolean" },
    "tlsCert":  { "type": "string" },
    "tlsKey":   { "type": "string" }
  },
  "additionalProperties": false,
  "dependentRequired": {
    "tls": ["tlsCert", "tlsKey"]
  }
}
```

### Combinators

| Keyword | Meaning |
|---|---|
| `allOf` | must satisfy every subschema |
| `anyOf` | must satisfy at least one |
| `oneOf` | must satisfy exactly one |
| `not` | must NOT satisfy the subschema |

```json
{
  "oneOf": [
    { "type": "object", "required": ["existingSecret"] },
    { "type": "object", "required": ["password"] }
  ]
}
```

### References

Local references only. Keramos resolves `$ref` against the same document, supporting both `#/$defs/...` and `#/definitions/...`. **Remote refs (`https://example.com/schema.json#/...`) are not supported** — the schema document must be self-contained.

```json
{
  "$defs": {
    "image": {
      "type": "object",
      "required": ["repository"],
      "properties": {
        "repository": { "type": "string" },
        "tag":        { "type": "string" },
        "pullPolicy": { "enum": ["Always", "IfNotPresent", "Never"] }
      }
    }
  },
  "type": "object",
  "properties": {
    "image":    { "$ref": "#/$defs/image" },
    "sidecar":  { "$ref": "#/$defs/image" }
  }
}
```

Self-referential cycles are detected and rejected (depth limit 32) so a malformed schema cannot send keramos into an infinite loop.

---

## What is *not* supported

These JSON Schema keywords are recognised by the file but ignored at validation time:

- `$schema` (descriptive only)
- `$id`
- `title`, `description`, `examples`, `default` — kept and surfaced by `keramos config` and `keramos show schema`
- Remote `$ref`
- `if` / `then` / `else` (conditional schemas)
- `unevaluatedProperties` / `unevaluatedItems`
- `contentEncoding`, `contentMediaType`, `contentSchema`

Use `oneOf`/`anyOf` plus `dependentRequired` to express conditional rules without `if/then/else`.

---

## A complete example

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "my-app values",
  "type": "object",
  "required": ["image"],
  "additionalProperties": false,
  "properties": {
    "replicas": {
      "type": "integer",
      "minimum": 1,
      "maximum": 50,
      "default": 1,
      "description": "Replica count."
    },
    "image": {
      "$ref": "#/$defs/image"
    },
    "service": {
      "type": "object",
      "properties": {
        "type": { "enum": ["ClusterIP", "NodePort", "LoadBalancer"] },
        "port": { "type": "integer", "minimum": 1, "maximum": 65535 }
      },
      "additionalProperties": false
    },
    "ingress": {
      "type": "object",
      "properties": {
        "enabled": { "type": "boolean" },
        "host":    { "type": "string", "format": "hostname" },
        "tls":     { "type": "boolean" }
      },
      "dependentRequired": {
        "tls": ["host"]
      }
    },
    "auth": {
      "oneOf": [
        { "type": "object", "required": ["existingSecret"], "properties": { "existingSecret": { "type": "string" } } },
        { "type": "object", "required": ["password"], "properties": { "password": { "type": "string", "minLength": 8 } } }
      ]
    }
  },
  "$defs": {
    "image": {
      "type": "object",
      "required": ["repository"],
      "properties": {
        "repository": { "type": "string", "minLength": 1 },
        "tag":        { "type": "string", "default": "latest" },
        "pullPolicy": { "enum": ["Always", "IfNotPresent", "Never"], "default": "IfNotPresent" }
      },
      "additionalProperties": false
    }
  }
}
```

Validation rejects values like:

```yaml
replicas: 100              # exceeds maximum
image:
  repository: ""           # fails minLength
service:
  type: Invalid            # not in enum
ingress:
  tls: true                # tls requires host (dependentRequired)
auth:
  password: short          # fails minLength: 8 in the password branch of oneOf
```

with messages naming the exact path and the violated keyword.

---

## Generating values from the schema

`keramos config` walks the schema interactively, prompting for each property, honouring `default`, `enum`, and `description`, and writing the resulting object to the path of your choice (default: `values.local.yaml`):

```
keramos config <package-dir> -o values.dev.yaml
```

`keramos show schema <package>` prints the schema (resolved through layers — children's schemas merge into the parent's `properties`).
