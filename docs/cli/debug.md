# keramos debug

## Synopsis

`keramos debug` renders templates with verbose tracing — every expression evaluation, every value lookup, every layer composition step is reported. The output is voluminous; pipe it to `less` or grep for the value you're chasing.

## When to use it

Use when a template rendered to a surprising value and `keramos values --trace` didn't pinpoint the cause (template-time logic, not values-time). Example: an `${if}` block that didn't fire when expected.

## Usage

```
keramos debug <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for debug |
| `--profile` | string | — | profile name to apply |
| `--set` | stringArray | — | set key=value overrides (repeatable) |
| `--trace` | — | — | enable step-by-step rendering trace |
| `-f, --values` | stringArray | — | values file overrides (repeatable) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Debug a single template file:

```sh
keramos debug ./my-app templates/deployment.yaml -f overrides.yaml
```

Debug everything:

```sh
keramos debug ./my-app | less
```

## See also

- [`template`](template.md)
- [`values`](values.md)
- [Template expressions](../templates/expressions.md)
