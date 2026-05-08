# keramos show chart

## Synopsis

`keramos show chart` prints the contents of a package's `keramos.yaml` to stdout — the package manifest containing name, version, apiVersion, declared layers, requires, environments, immutables, and metadata. Read-only; operates entirely on the package directory.

## When to use it

Use to verify a package's identity, version, declared composition, or environment definitions before installing. Especially useful after pulling an unfamiliar package: a quick `show chart` tells you what you're about to deploy.

## What happens when you run it

1. Reads `<package-path>/keramos.yaml`.
2. Prints it to stdout, unchanged.
3. No layer resolution, no value merging, no cluster contact.

## Usage

```
keramos show chart <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for chart |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Show metadata for a local package:

```sh
keramos show chart ./my-app
```

Inspect a pulled package directory:

```sh
keramos pull my-app --repo https://charts.example.com --version 1.2.3 -d ./pulled --untar
keramos show chart ./pulled/my-app
```

Pipe through `yq` to extract one field:

```sh
keramos show chart ./my-app | yq '.version'
```

## See also

- [`show`](show.md)
- [`show all`](show-all.md)
- [`show values`](show-values.md)
- [`show readme`](show-readme.md)
- [`keramos.yaml` reference](../reference/keramos-yaml.md)
