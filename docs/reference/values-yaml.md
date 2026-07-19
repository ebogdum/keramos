# values.yaml

The package's default configuration, sitting beside `keramos.yaml` at the package
root. Its contents are an arbitrary YAML map that your templates read via
`${values.<path>}`. Keramos imposes no fixed schema — you define whatever shape
suits the application — but it does define how values merge and reserves one
key, `tags`, for layer selection.

## Minimal example

```yaml
name: my-app
replicaCount: 1
image:
  repository: nginx
  tag: latest
service:
  port: 80
```

A template then references these as `${values.image.repository}`,
`${values.service.port}`, and so on. `keramos create` generates this file.

## Fields

`values.yaml` has no required keys and no fixed field set. Define keys freely to
match your templates. The table below covers only the keys keramos itself gives
meaning to; every other key is passed through untouched.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `tags` | map of string→bool | no | `{}` | Reserved. Each `tags.<name>` toggles the layers in `keramos.yaml` that list `<name>` under their `tags:`. A truthy value enables those layers. |
| *(any other key)* | any | no | — | Application configuration. Read in templates via `${values.<path>}` and usable in a layer's `condition` (a dotted path such as `cache.enabled`). |

## Merge order

At render time keramos computes the *effective values* by deep-merging these
sources, later winning (maps merge recursively; scalars and lists replace):

1. Each enabled layer's `values.yaml`, in layer order.
2. This package's `values.yaml`.
3. The active profile's values (`--profile`, if any).
4. The selected environment's `valueFiles` then inline `values` (`--env`).
5. `-f / --values <file>` arguments, in CLI order.
6. `--set` arguments.
7. `--set-string` arguments (forces string typing).
8. `--set-file` arguments (file contents become the value).
9. `--set-json` arguments.

Set a key to `null` (YAML `~`) to null it out from a lower-precedence source.
The effective values are validated against `values.schema.json` if that file
is present.

### --set path syntax

`--set`, `--set-string`, `--set-file`, and `--set-json` share the same path
grammar:

| Path expression | Meaning |
|---|---|
| `replicaCount=3` | Set a top-level key. |
| `image.tag=1.4.2` | Set a nested key; intermediate maps are created. |
| `args[0]=--debug` | Set an array index; the array is created if absent. |
| `label\.io/name=api` | Backslash-escape a literal `.` inside a key. |

Use `--set-string image.tag=2.0` when a numeric-looking value must stay a
string.

## Full example

`values.yaml`:

```yaml
# Application defaults. Templates read these as ${values.<path>}.
name: platform-api
replicaCount: 2

image:
  repository: registry.example.com/platform-api
  tag: "1.4.2"

service:
  type: ClusterIP
  port: 8080

resources:
  requests:
    cpu: 100m
    memory: 128Mi

# Feature flag also usable as a layer `condition: cache.enabled`.
cache:
  enabled: false

# Reserved: toggles keramos.yaml layers that declare `tags: [observability]`.
tags:
  observability: false
```

A `templates/deployment.yaml` reading these values:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "${values.name}"
spec:
  replicas: ${values.replicaCount}
  template:
    spec:
      containers:
        - name: app
          image: "${values.image.repository}:${values.image.tag}"
```

Rendering with `--set replicaCount=5 --set cache.enabled=true` produces a
Deployment with `replicas: 5` and, because `cache.enabled` is now truthy,
activates any layer whose `condition` is `cache.enabled`.

## See also

- [values.schema.json](values-schema-json.md) — validate the effective values.
- [`keramos template`](../cli/template.md) / [`keramos install`](../cli/install.md) — render values into manifests.
- [`keramos show values`](../cli/show-values.md), [`keramos get values`](../cli/get-values.md) — inspect defaults and effective values.
- [Values guide](../guides/values.md), [Layers guide](../guides/layers.md).
