# keramos dependency update

## Synopsis

`keramos dependency update` re-resolves every layer and required package against its source (HTTP repo index, OCI registry tag list, git ref, local path) and rewrites `keramos.lock` with the freshest pinned digests. The lockfile is the source of truth for `keramos install`, `keramos template`, and `keramos dependency build` — they all consult `keramos.lock` first and only fall back to `keramos.yaml`'s constraint if the lock is missing or stale.

## When to use it

Run whenever you edit `keramos.yaml`'s `layers:` or `requires:`, or when you want to bump a layer to the highest version still satisfying its constraint. Pass an optional `[name]` argument to update a single layer in place; without it, every layer is re-resolved. Always commit the resulting `keramos.lock` to source control — without it, two builds of the same package can pick up different layer versions if the constraint allows it.

## What happens when you run it

1. Reads `keramos.yaml` from `<package-path>`.
2. Refreshes upstream indexes (HTTP repo `index.yaml`, OCI registry tag list) unless `--skip-refresh` is set.
3. For each layer / require, resolves the constraint (`version:`, `ref:`) to a specific tag/commit/digest.
4. Writes `keramos.lock` next to `keramos.yaml`.
5. Prints a summary of changed layers (added, upgraded, removed) on stdout.

## Usage

```
keramos dependency update <package-path> [name] [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for update |
| `--skip-refresh` | bool | false | skip repository index refresh before resolving |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Re-resolve every layer and rewrite `keramos.lock`:

```sh
keramos dependency update ./my-app
```

Update a single layer by name (other layers remain at their locked versions):

```sh
keramos dependency update ./my-app shared-base
```

Skip the index refresh — useful in CI when an earlier step has already populated the cache:

```sh
keramos dependency update ./my-app --skip-refresh
```

Pair with `dependency build` to actually fetch the resolved versions:

```sh
keramos dependency update ./my-app
keramos dependency build  ./my-app
```

## See also

- [`dependency`](dependency.md)
- [`dependency build`](dependency-build.md) — materialise the cache
- [`dependency list`](dependency-list.md) — inspect the lock
- [`dependency tree`](dependency-tree.md) — visualise the composition chain
- [Layers guide](../guides/layers.md)
