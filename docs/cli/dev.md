# keramos dev

## Synopsis

`keramos dev` watches a package directory and re-renders / re-installs on every file change. Pair with `kubectl apply` watching or with a `kind` cluster for an instant feedback loop while authoring templates.

## When to use it

Use during package development. Not for production — `dev` re-applies eagerly without dry-runs or value validation gates.

## Usage

```
keramos dev <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for dev |
| `--interval` | duration | 500ms | poll interval |
| `--profile` | string | — | profile name |
| `--set` | stringArray | — | key=value (repeatable) |
| `-f, --values` | stringArray | — | values file (repeatable) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Live-render a package on every save (re-renders into stdout on each change):

```sh
keramos dev ./my-app -n dev
```

Live-render with values overrides:

```sh
keramos dev ./my-app -f overrides.yaml --set replicas=2 -n dev
```

Slow polling for large packages:

```sh
keramos dev ./my-app -n dev --interval 2s
```

## See also

- [`template`](template.md)
- [`debug`](debug.md)
