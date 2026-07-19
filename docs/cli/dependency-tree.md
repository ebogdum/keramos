# keramos dependency tree

`keramos dependency tree` prints the composition chain of a package: every layer
and required package, and recursively their own layers and requires.

## When to use it

- To see the full shape of a package that is more than one level deep — which
  layer pulls in which.
- To tell layers (`[layer]`, merged into your package) apart from requires
  (`[requires]`, installed alongside) at a glance.

## What happens

keramos resolves each `source` to its package, reads that package's version and
its own dependencies, and prints an indented tree rooted at your package.
Every node shows `[kind] name@version (source)`:

- `[layer]` — merged into the parent at render time.
- `[requires]` — a co-deployed package.

The tree descends into each resolved package, so a layer that itself declares
layers appears with its children nested beneath it. Resolving remote sources
(`git::`, registry) fetches them into keramos's local cache; local paths are read
in place.

## Usage

```
keramos dependency tree <package-path>
```

## Flags

Inherits the global flags.

## Worked example

**INPUT — `./web/keramos.yaml`** with two layers and one required package:

```yaml
apiVersion: keramos/v1
name: web
version: 0.3.0
layers:
  - name: base-layer
    source: ../base-layer     # base-layer/keramos.yaml → version 1.0.0
  - name: common
    source: ../common-layer   # common-layer/keramos.yaml → version 1.4.0
requires:
  - name: redis
    source: ../redis-req      # redis-req/keramos.yaml → version 7.2.0
```

**Command:**

```sh
keramos dependency tree ./web
```

**OUTPUT:**

```
web@0.3.0
├── [layer] base-layer@1.0.0 (/abs/path/to/base-layer)
├── [layer] common@1.4.0 (/abs/path/to/common-layer)
└── [requires] redis@7.2.0 (/abs/path/to/redis-req)
```

**Tracing each input to its output line:**

| `keramos.yaml` entry | Output line | Why |
|---|---|---|
| root package | `web@0.3.0` | the tree root is your package's `name@version` |
| `layers[0]` `base-layer` | `[layer] base-layer@1.0.0` | resolved `../base-layer`, read version `1.0.0` from its `keramos.yaml` |
| `layers[1]` `common` | `[layer] common@1.4.0` | resolved `../common-layer`, version `1.4.0` |
| `requires[0]` `redis` | `[requires] redis@7.2.0` | a `requires:` entry → `[requires]` label; version `7.2.0` |

Each version comes from the *resolved* package's `keramos.yaml`, not from
`keramos.yaml`'s source line — that is how the tree confirms what a source
actually points at.

## See also

- [`dependency list`](dependency-list.md) — the flat table view with lock status
- [`dependency build`](dependency-build.md) — download the packages in this tree
- [`template`](template.md) — render the package with its layers merged in
