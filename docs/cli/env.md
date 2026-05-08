# keramos env

## Synopsis

`keramos env` prints keramos's environment information: the resolved config dir, cache dir, data dir, namespace, kubeconfig path, kubeconfig context, and any overrides supplied via environment variables.

## When to use it

Use when debugging path or auth issues — when you're not sure whether `~/.config/keramos` is being used, when `KUBECONFIG` isn't being read as expected, etc.

## Usage

```
keramos env [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for env |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Print keramos's environment:

```sh
keramos env
```

## See also

- [CLI reference index](README.md)
