# keramos show

## Synopsis

`keramos show` prints information about a package without installing it: the `keramos.yaml` (`chart`), the `values.yaml` (`values`), the README (`readme`), the CRDs in `crds/` (`crds`), or all of the above (`all`).

## When to use it

Use to inspect a package's structure before installing or before vendoring it. Works against local directories, registered repos, and OCI references.

## Usage

```
keramos show [command]
```

## Subcommands

- [`keramos show chart`](show-chart.md) — Show package metadata (keramos.yaml)
- [`keramos show crds`](show-crds.md) — Show CRDs declared by the package
- [`keramos show readme`](show-readme.md) — Show package README
- [`keramos show values`](show-values.md) — Show default values (values.yaml)

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for show |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Show the package metadata:

```sh
keramos show chart my-charts/my-app
```

Show the default values:

```sh
keramos show values my-charts/my-app
```

Show all package data:

```sh
keramos show all my-charts/my-app -o yaml
```

## See also

- [`pull`](pull.md)
- [`search`](search.md)
