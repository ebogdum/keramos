# Schema validation

`values.schema.json` is keramos's mechanism for asserting that a package's effective values are well-formed before render. The schema document is authored alongside `values.yaml`; keramos validates the merged values (after every layer, environment, profile, and CLI override has been applied) and aborts the operation with a precise error if validation fails.

This guide covers patterns and idioms. The reference for the supported subset is at [`values.schema.json` reference](../reference/values-schema-json.md).

## Why schema validation pays off

Without a schema, the only thing that catches a misconfigured `replicas: "three"` is the cluster's API server — which means the install reaches the apply stage, possibly half-applies a manifest, and rolls back. With a schema:

- Typos (`replicaa: 3`) are rejected before render.
- Type mismatches (`replicas: "three"`) are rejected with a precise path and reason.
- Missing required fields are listed by full path.
- Out-of-range numbers, malformed hostnames, invalid regex patterns are all caught.

The schema also doubles as **documentation**. `keramos config <pkg>` walks it interactively, and `description` fields surface in `keramos show schema <pkg>`.

## Authoring a schema

A pragmatic schema for a typical web app looks like this:

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
    "image": {
      "$ref": "#/$defs/image"
    },
    "service": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "type": {
          "enum": ["ClusterIP", "NodePort", "LoadBalancer"],
          "default": "ClusterIP"
        },
        "port": { "type": "integer", "minimum": 1, "maximum": 65535 }
      }
    },
    "ingress": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "enabled": { "type": "boolean", "default": false },
        "host":    { "type": "string", "format": "hostname" },
        "tls":     { "type": "boolean", "default": false }
      },
      "dependentRequired": {
        "tls": ["host"]
      }
    },
    "resources": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "requests": { "$ref": "#/$defs/resourceQuantities" },
        "limits":   { "$ref": "#/$defs/resourceQuantities" }
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
    },
    "resourceQuantities": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "cpu":    { "type": "string", "pattern": "^[0-9]+(m|\\.[0-9]+)?$" },
        "memory": { "type": "string", "pattern": "^[0-9]+(Ki|Mi|Gi|Ti|Pi|Ei|K|M|G|T|P|E)?$" }
      }
    }
  }
}
```

Note the patterns:

- `additionalProperties: false` on every object — typos are caught.
- `$ref` to factor out repeated shapes (`image`, `resourceQuantities`).
- `dependentRequired` on `ingress.tls` — turning on TLS demands a `host`.
- `default` values surface in `keramos config` and `keramos show schema`.

## Patterns

### Either / or

A user must supply *either* an existing Secret name *or* an inline password — not both, not neither.

```json
{
  "auth": {
    "oneOf": [
      {
        "type": "object",
        "required": ["existingSecret"],
        "additionalProperties": false,
        "properties": {
          "existingSecret": { "type": "string", "minLength": 1 }
        }
      },
      {
        "type": "object",
        "required": ["password"],
        "additionalProperties": false,
        "properties": {
          "password": { "type": "string", "minLength": 8 }
        }
      }
    ]
  }
}
```

A user supplying `auth: { existingSecret: my-creds, password: hunter2 }` is rejected: the value matches both branches and `oneOf` requires exactly one.

### Discriminated union

When a single key (`type`) selects between several alternative bodies:

```json
{
  "storage": {
    "oneOf": [
      {
        "type": "object",
        "required": ["type", "size"],
        "additionalProperties": false,
        "properties": {
          "type":  { "const": "pvc" },
          "size":  { "type": "string", "pattern": "^[0-9]+(Mi|Gi|Ti)$" },
          "class": { "type": "string" }
        }
      },
      {
        "type": "object",
        "required": ["type", "bucket"],
        "additionalProperties": false,
        "properties": {
          "type":   { "const": "s3" },
          "bucket": { "type": "string", "minLength": 3 },
          "region": { "type": "string", "default": "us-east-1" }
        }
      }
    ]
  }
}
```

`keramos config` recognises this pattern and prompts for `type` first, then offers fields specific to the chosen branch.

### Constrained enum with a fallback

For an open enum where you want to permit unknown values too:

```json
{
  "deploymentStrategy": {
    "anyOf": [
      { "enum": ["RollingUpdate", "Recreate"] },
      { "type": "string", "minLength": 1 }
    ]
  }
}
```

`anyOf` accepts the well-known values *or* any non-empty string. The first branch surfaces in `keramos config` as a dropdown; the second is the escape hatch.

### Pattern validation for image refs

```json
{
  "image": {
    "type": "object",
    "properties": {
      "repository": {
        "type": "string",
        "pattern": "^[a-z0-9]+([._\\-/a-z0-9]+)*$"
      },
      "tag": {
        "type": "string",
        "pattern": "^[A-Za-z0-9_][A-Za-z0-9._\\-]{0,127}$"
      },
      "digest": {
        "type": "string",
        "pattern": "^sha256:[a-f0-9]{64}$"
      }
    }
  }
}
```

Three patterns: a Docker repo path, a valid tag, and a SHA-256 content digest.

### Cross-field constraint via `dependentRequired`

```json
{
  "tls": { "type": "boolean", "default": false },
  "tlsCert": { "type": "string" },
  "tlsKey":  { "type": "string" },
  "dependentRequired": {
    "tls": ["tlsCert", "tlsKey"]
  }
}
```

Reads as "if `tls` is set, also require `tlsCert` and `tlsKey`". `dependentRequired` is much simpler than `if/then/else` for the common patterns.

## What schemas can't enforce

Some constraints are out of scope. Examples:

- **Cross-package references**: "this value must be a Service that exists in the cluster". For these, use a `pre-install` hook that validates at run time.
- **Mutual exclusion across distant fields**: schemas can express it via `oneOf` but it's verbose. For complex cross-field rules, write a keramos policy rule under `policies/`.
- **Conditional defaults**: schemas express *valid shapes*, not *derived values*. For derived defaults, use template expressions (`${values.image.tag | default .Chart.AppVersion}`).

## Composition with layers

When a parent package has layers, every layer's `values.schema.json` is loaded and the parent's schema's `properties` are merged with each layer's namespaced properties. So:

```
parent/
├── keramos.yaml      # layers: [shared-base]
├── values.schema.json
│   {
│     "type": "object",
│     "properties": {
│       "replicas": { "type": "integer" }
│     }
│   }
└── ...

shared-base/
├── keramos.yaml
└── values.schema.json
    {
      "type": "object",
      "properties": {
        "image": { "type": "object", ... }
      }
    }
```

The effective parent schema treats `shared-base.image` as a valid path because the layer's schema is namespaced under the layer name.

## Inspecting

```sh
keramos show schema <pkg>             # print the (composed) schema
keramos lint <pkg>                    # validates package values + schema
keramos config <pkg>                  # interactive walker; uses schema for prompts
```

## Errors

```
$ keramos install my-app . --set replicas=100
Error: schema validation failed:
  replicas: 100 exceeds maximum 50
  service.type: "Anycast" is not one of [ClusterIP, NodePort, LoadBalancer]
  ingress.tls: required when ingress.tls is set, but ingress.host is missing
```

Errors include the full dotted path, the violating value, and the violated keyword. Use `keramos lint` in CI to catch these before they reach a cluster.
