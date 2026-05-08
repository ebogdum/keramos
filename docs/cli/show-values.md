# keramos show values

## Synopsis

`keramos show values` prints the contents of a package's `values.yaml` to stdout — the default configuration values shipped with the package. This is the starting point for any operator override; copy it to a local file and edit, then `keramos install -f <local-values.yaml>`.

## When to use it

Use to inspect what knobs a package exposes before installing or upgrading. The output is the raw `values.yaml`, so any comments the author wrote in the file are preserved — those are typically where you find the per-key documentation.

## What happens when you run it

1. Reads `<package-path>/values.yaml`.
2. Prints the content to stdout, unchanged.
3. No layer composition, no merging — this is just the package's own defaults, not the merged result.
4. No cluster contact, no network.

## Usage

```
keramos show values <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for values |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Show defaults for a local package:

```sh
keramos show values ./my-app
```

Capture the defaults as a starting overrides file:

```sh
keramos show values ./my-app > overrides.yaml
# edit overrides.yaml
keramos install hello ./my-app -f overrides.yaml -n staging
```

Inspect a pulled package's values:

```sh
keramos pull my-app --repo https://charts.example.com --version 1.2.3 -d ./pulled --untar
keramos show values ./pulled/my-app
```

## See also

- [`show`](show.md)
- [`show all`](show-all.md)
- [`values`](values.md) — render-time merge with overrides
- [Values guide](../guides/values.md)
- [`values.yaml` reference](../reference/values-yaml.md)
