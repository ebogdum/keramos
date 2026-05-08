# keramos releases install

## Synopsis

Install every release declared in `keramos-releases.yaml` in topological order. The graph is computed from each entry's `dependsOn` list; releases at the same topological level have no inter-dependencies among themselves.

## When to use it

Use to bring up a fresh fleet of related releases with one command — typically a platform bootstrap that combines locally-developed packages with upstream OCI releases.

## Usage

```
keramos releases install [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--file` | string | "keramos-releases.yaml" | spec file path |
| `-h, --help` | — | — | help for install |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Install every release in `./keramos-releases.yaml`:

```sh
keramos releases install
```

Install from a custom-named file:

```sh
keramos releases install --file ./platform.releases.yaml
```

## See also

- [`releases`](releases.md)
