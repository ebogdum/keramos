# keramos dependency build

## Synopsis

`keramos dependency build` resolves every layer and required package declared in `keramos.yaml`, downloads each one to the package's local cache (`./.keramos/layers/`), and verifies the lockfile is consistent. After a successful build, `keramos install` and `keramos template` can render the package fully offline. The command is the materialise-side counterpart to `keramos dependency update` (which writes `keramos.lock`).

## When to use it

Run after cloning a package fresh to a new machine, after `keramos dependency update` to prefetch the new versions, or in CI to pre-populate the layer cache before running `keramos lint` or `keramos template`. Idempotent: re-running with no changes is a fast no-op.

## What happens when you run it

1. Keramos reads `keramos.yaml` and `keramos.lock`.
2. For every layer / require, keramos fetches the source (local copy, HTTPS archive, OCI artifact, git clone) into `./.keramos/layers/<name>/`.
3. Each downloaded archive's digest is checked against `keramos.lock`. With `--verify` set, the digest check fails the build instead of silently regenerating the lock entry.
4. Index caches are reused unless `--no-cache` is set.
5. On success, prints the resolved version of every layer; on failure, names the offending layer.

## Usage

```
keramos dependency build <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for build |
| `--no-cache` | bool | false | clear index cache before resolving |
| `--verify` | bool | false | verify digests of installed dependencies |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Materialise every layer for a package:

```sh
keramos dependency build ./my-app
```

Strict mode for CI: any digest mismatch fails the build:

```sh
keramos dependency build ./my-app --verify
```

Bust the index cache before resolving — useful when an upstream repository's `index.yaml` changed and you want a clean fetch:

```sh
keramos dependency build ./my-app --no-cache
```

## See also

- [`dependency`](dependency.md)
- [`dependency update`](dependency-update.md) — refresh `keramos.lock` before building
- [`dependency list`](dependency-list.md) — show what is declared
- [`dependency tree`](dependency-tree.md) — visualise the composition chain
- [Layers guide](../guides/layers.md)
