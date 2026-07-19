# values.schema.json

An optional file at the package root that declares the expected shape of your
values. When present, keramos validates the *effective values* (defaults plus all
overrides) against it before rendering, catching typos, wrong types, missing
required keys, and out-of-range numbers before manifests reach the cluster. The
file is a standard JSON Schema Draft 2020-12 document; keramos honours the
practical subset listed below and silently ignores keywords it does not know.

## Minimal example

```json
{
  "type": "object",
  "required": ["image"],
  "properties": {
    "replicaCount": { "type": "integer", "minimum": 1 },
    "image": {
      "type": "object",
      "required": ["repository"],
      "properties": {
        "repository": { "type": "string" },
        "tag": { "type": "string" }
      }
    }
  }
}
```

## Where validation runs

- `keramos install`, `keramos upgrade`, and `keramos template` validate the effective
  values before rendering; a violation aborts with the failing path and reason.
- `keramos lint` checks that the file is valid JSON and validates the package's
  own values against it.
- `keramos config` walks the schema to build a values file interactively.

## Supported keywords

The validator recognises the keywords below. Anything else in the document is
ignored (not an error), so richer schemas still load — they just are not
enforced.

| Group | Keywords | Notes |
|---|---|---|
| Type | `type` | One of `object`, `array`, `string`, `integer`, `number`, `boolean`, `null`, or a list of these for a union (`["string","null"]`). |
| Object | `properties`, `required`, `additionalProperties`, `patternProperties`, `minProperties`, `maxProperties`, `dependentRequired` | `additionalProperties` may be `false` or a subschema. `patternProperties` keys are regexes (capped at 512 bytes). |
| Array | `items`, `minItems`, `maxItems`, `uniqueItems` | `items` is a single subschema applied to every element. |
| String | `minLength`, `maxLength`, `pattern`, `format` | `pattern` is a regex (capped at 512 bytes). |
| Number | `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf` | `integer` additionally requires a whole number. |
| Combinators | `allOf`, `anyOf`, `oneOf`, `not` | `oneOf` must match exactly one subschema. |
| References | `$ref`, `$defs`, `definitions` | Only same-document JSON pointers (`#/$defs/...`); external refs are not resolved. Recursion is capped at 32 levels. |
| Values | `enum`, `const` | Exact-value constraints; numbers compare by value. |

When a schema omits `type`, keramos infers intent from the keywords present, so a
root object with only `properties`/`required` still validates as an object.

### format values

`format` is enforced for: `email`, `uri`, `uri-reference`, `uuid`, `ipv4`,
`ipv6`, `hostname`, `date`, `date-time`, `time`. Any other format is accepted
without checking.

## Full example

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["image", "service"],
  "additionalProperties": true,
  "properties": {
    "replicaCount": { "type": "integer", "minimum": 1, "maximum": 10 },
    "image": {
      "type": "object",
      "required": ["repository"],
      "properties": {
        "repository": { "type": "string", "minLength": 1 },
        "tag": { "type": "string" },
        "pullPolicy": { "enum": ["Always", "IfNotPresent", "Never"] }
      }
    },
    "service": {
      "type": "object",
      "properties": {
        "type": { "enum": ["ClusterIP", "NodePort", "LoadBalancer"] },
        "port": { "type": "integer", "minimum": 1, "maximum": 65535 }
      }
    },
    "ingress": { "$ref": "#/$defs/ingress" }
  },
  "$defs": {
    "ingress": {
      "type": "object",
      "properties": {
        "enabled": { "type": "boolean" },
        "host": { "type": "string", "format": "hostname" }
      }
    }
  }
}
```

Against these values validation fails with two errors — the string tag and the
out-of-range port:

```yaml
replicaCount: "two"     # $.replicaCount: expected integer, got string
image:
  repository: nginx
service:
  port: 99999           # $.service.port: 99999 greater than maximum 65535
```

## See also

- [values.yaml](values-yaml.md) — the file this schema constrains.
- [`keramos lint`](../cli/lint.md) — validate the schema and values.
- [`keramos config`](../cli/config.md) — build values from the schema.
- [Schema validation guide](../guides/schema-validation.md).
