---
title: "Schema validation"
parent: "Guides"
---
{% raw %}
# Schema validation

`values.schema.json` asserts that a package's merged values are well-formed
before render. Keramos validates the values — after every layer, environment,
profile, and CLI override has been applied — and aborts with a precise error if
validation fails. It runs during `keramos template`, `keramos install`, and
`keramos upgrade`.

This guide covers authoring patterns. The supported keyword subset is in the
[`values.schema.json` reference](../reference/values-schema-json.md).

## Why it pays off

Without a schema, a misconfigured `replicas: "three"` is caught only by the API
server — after the install has already started applying. With a schema:

- Typos (`replicaa: 3`) are rejected before render.
- Type mismatches (`replicas: "three"`) are rejected with a path and reason.
- Missing required fields are listed by full path.
- Out-of-range numbers, bad patterns, and unknown keys are all caught.

The schema also documents the package: `keramos config <pkg>` walks it
interactively, using `description` and `default` for its prompts.

## Authoring a schema

A pragmatic schema for a web app:

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
      "description": "Replica count for the main Deployment."
    },
    "image": { "$ref": "#/$defs/image" },
    "service": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "type": { "enum": ["ClusterIP", "NodePort", "LoadBalancer"], "default": "ClusterIP" },
        "port": { "type": "integer", "minimum": 1, "maximum": 65535 }
      }
    }
  },
  "$defs": {
    "image": {
      "type": "object",
      "required": ["repository"],
      "additionalProperties": false,
      "properties": {
        "repository": { "type": "string", "minLength": 1 },
        "tag":        { "type": "string", "default": "latest" },
        "pullPolicy": { "enum": ["Always", "IfNotPresent", "Never"], "default": "IfNotPresent" }
      }
    }
  }
}
```

Note the moves:

- `additionalProperties: false` on every object catches typos.
- `$ref` factors out repeated shapes.
- `default` values surface in `keramos config`.

## Patterns

### Either / or

A user must supply *either* an existing Secret name *or* an inline password:

```json
{
  "auth": {
    "oneOf": [
      { "type": "object", "required": ["existingSecret"], "additionalProperties": false,
        "properties": { "existingSecret": { "type": "string", "minLength": 1 } } },
      { "type": "object", "required": ["password"], "additionalProperties": false,
        "properties": { "password": { "type": "string", "minLength": 8 } } }
    ]
  }
}
```

Supplying both is rejected — the value matches both branches and `oneOf`
requires exactly one.

### Discriminated union

A `const` key selects between alternative bodies:

```json
{
  "storage": {
    "oneOf": [
      { "type": "object", "required": ["type", "size"], "additionalProperties": false,
        "properties": { "type": { "const": "pvc" }, "size": { "type": "string" } } },
      { "type": "object", "required": ["type", "bucket"], "additionalProperties": false,
        "properties": { "type": { "const": "s3" }, "bucket": { "type": "string" } } }
    ]
  }
}
```

### Cross-field constraint

`dependentRequired` reads as "if `tls` is set, also require `host`":

```json
{
  "tls":  { "type": "boolean", "default": false },
  "host": { "type": "string" },
  "dependentRequired": { "tls": ["host"] }
}
```

### Pattern validation

```json
{
  "tag":    { "type": "string", "pattern": "^[A-Za-z0-9_][A-Za-z0-9._\\-]{0,127}$" },
  "digest": { "type": "string", "pattern": "^sha256:[a-f0-9]{64}$" }
}
```

## What schemas can't enforce

- **Cluster references** ("this must name a Service that exists") — use a
  `pre-install` hook that checks at run time.
- **Complex cross-field rules** beyond `oneOf` / `dependentRequired` — write a
  [policy rule](../cli/policy-check.md) under `policies/`.
- **Derived defaults** — schemas express valid *shapes*, not computed values.
  Use template expressions (`${values.image.tag | default package.version}`).

## Composition with layers

When a parent has layers, each layer's `values.schema.json` is loaded and merged
into the parent's schema under the layer's namespace, so a layer's `image`
becomes a valid path at `<layer-name>.image`.

## Errors

Validation errors carry the full JSON-pointer path, the violating value, and the
keyword, one per line:

```sh
keramos template . --set replicas=100 --set service.type=Bad
```

```
Error: values failed schema validation:
  - $.service.type: value not in enum
  - $.replicas: 100 greater than maximum 50
```

Other typical messages:

```
  - $.image.repository: required property missing
  - $.replicas: expected integer, got string
  - $.replicaa: additional property not allowed
  - $.tag: string does not match pattern "^[a-z0-9.]+$"
  - $: property "tls" requires "host" to be set
  - $.storage: value matches 0 of oneOf (expected exactly 1)
```

## Inspecting and gating

```sh
keramos config .        # interactive walker that builds a values file from the schema
```

```
Walking schema. Press <enter> to keep defaults.
image.repository (string) *: ...
```

`keramos lint` does **not** run schema validation — it only checks that the schema
file is valid JSON. To gate values in CI, render them so the check runs:

```sh
keramos template . -f ci-values.yaml     # non-zero exit on any violation
```

`keramos install --dry-run server` does the same against a live API server.

## See also

- [`values.schema.json` reference](../reference/values-schema-json.md) — the
  supported keyword subset.
- [Values](values.md) — how the values being validated are assembled.
- [`keramos config`](../cli/config.md) and [`keramos lint`](../cli/lint.md).
{% endraw %}
