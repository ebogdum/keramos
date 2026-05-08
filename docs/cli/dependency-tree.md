# keramos dependency tree

## Synopsis

`keramos dependency tree` prints an indented, ASCII-art view of a package's composition chain: each layer and required package, plus their own layers and requires recursively. It's the visual companion to `keramos dependency list` — useful when a package is more than two levels deep and you need to see at a glance who's pulling what.

## When to use it

Use when the package has nested layers (a layer that itself depends on layers) and you want to confirm the full composition resolves the way you expect. Also handy for documenting a package — pasting the tree output into a README gives readers the dependency picture in one block.

## What happens when you run it

1. Reads `keramos.yaml` from `<package-path>` and any locked layer metadata in `keramos.lock`.
2. For each layer / require, recursively reads the layer's own `keramos.yaml` (from cache if materialised, otherwise warns).
3. Prints an indented tree to stdout.
4. Read-only; no cluster contact, no network access, no file writes.

## Usage

```
keramos dependency tree <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for tree |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Render the composition tree of a package:

```sh
keramos dependency tree ./my-app
```

Render and capture in a file for documentation:

```sh
keramos dependency tree ./my-app > docs/composition.txt
```

Run from inside the package directory:

```sh
cd ./my-app && keramos dependency tree .
```

## See also

- [`dependency`](dependency.md)
- [`dependency list`](dependency-list.md) — flat tabular view
- [`dependency update`](dependency-update.md) — refresh the lock before drawing the tree
- [Layers guide](../guides/layers.md)
