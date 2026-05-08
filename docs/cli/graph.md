# keramos graph

## Synopsis

`keramos graph` renders a dependency graph of a release's resources and hooks. Output is a Graphviz DOT file (default) or Mermaid syntax, suitable for piping into a renderer.

## When to use it

Use for visual documentation of a release — what depends on what, which hooks fire when, what owner-references chain.

## Usage

```
keramos graph <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-f, --format` | string | "mermaid" | format: mermaid, dot, ascii |
| `-h, --help` | — | — | help for graph |
| `--revision` | int | — | release revision (default: latest) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Generate DOT and render via Graphviz:

```sh
keramos graph my-app -n prod | dot -Tpng > graph.png
```

Mermaid output:

```sh
keramos graph my-app -n prod --format mermaid
```

## See also

- [`get`](get.md)
