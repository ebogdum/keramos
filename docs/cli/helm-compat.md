# keramos helm-compat

## Synopsis

`keramos helm-compat` provides Helm chart compatibility helpers. Subcommands export a keramos package as a Helm-compatible artifact (suitable for tooling that expects a Helm chart layout) and report on the compatibility of an existing Helm chart for use with keramos.

## When to use it

Use when the surrounding ecosystem expects Helm artifacts (e.g. CI scanners, registries, GitOps tools that only know Helm). The export is best-effort: keramos's `${...}` expressions may end up as inert literal strings in the exported chart, which is fine for static analysis but not for actual Helm install.

## Usage

```
keramos helm-compat [command]
```

## Subcommands

- [`keramos helm-compat report`](helm-compat-report.md) — Analyse a Helm chart and report which constructs keramos supports natively

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for helm-compat |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Export a keramos package as a Helm chart skeleton:

```sh
keramos helm-compat export ./my-app -d ./helm-export
```

Report on Helm-chart compatibility:

```sh
keramos helm-compat report ./upstream-chart
```

## See also

- [`migrate`](migrate.md)
- [Migration guide](../guides/migration.md)
