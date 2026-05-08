# keramos show crds

## Synopsis

`keramos show crds` prints every `CustomResourceDefinition` YAML the package ships under its `crds/` directory. CRDs in this directory are applied **before** the rest of the manifest at install time and waited for `Established=true`; this command lets you inspect what cluster-API extensions you'd be granting before doing so.

## When to use it

Use to audit a package's CRD footprint before installing — particularly important for packages that introduce new operators, custom resources, or admission-webhook configurations. CRDs are cluster-scoped and outlive their installing release, so installing one is a higher-stakes commitment than installing a regular workload.

## What happens when you run it

1. Reads every YAML file under `<package-path>/crds/`.
2. Concatenates them with `---` separators.
3. Prints to stdout.
4. No layer resolution, no template rendering — CRDs are passed through verbatim.

## Usage

```
keramos show crds <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for crds |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Print the CRDs a package would install:

```sh
keramos show crds ./my-app
```

Lint a package's CRDs with `kubectl`'s offline validator:

```sh
keramos show crds ./my-app | kubectl apply --dry-run=client -f -
```

Pipe to `kube-linter` for a security review:

```sh
keramos show crds ./my-app | kube-linter lint -
```

## See also

- [`show`](show.md)
- [`show all`](show-all.md)
- [`show chart`](show-chart.md)
- [Package anatomy: crds](../guides/packages.md)
