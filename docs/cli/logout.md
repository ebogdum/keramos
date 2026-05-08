# keramos logout

## Synopsis

`keramos logout` removes stored credentials for a registry. The repo registration itself is preserved; only the saved username/password is dropped.

## When to use it

Use when rotating or revoking credentials.

## Usage

```
keramos logout <host> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for logout |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Log out of a registry:

```sh
keramos logout charts.example.com
```

## See also

- [`login`](login.md)
