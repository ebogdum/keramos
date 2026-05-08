# keramos releases upgrade

## Synopsis

Upgrade every release in `keramos-releases.yaml`; install if missing. One invocation brings the platform graph up the first time and keeps it up-to-date thereafter.

## When to use it

Use as the canonical CI deploy command for the whole platform graph.

## Usage

```
keramos releases upgrade [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--file` | string | "keramos-releases.yaml" | spec file path |
| `-h, --help` | — | — | help for upgrade |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Bring the platform up to declared versions:

```sh
keramos releases upgrade
```

Use a custom-named manifest file:

```sh
keramos releases upgrade --file ./platform.releases.yaml
```

## See also

- [`releases`](releases.md)
- [`upgrade`](upgrade.md)
