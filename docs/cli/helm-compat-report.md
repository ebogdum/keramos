# keramos helm-compat report

## Synopsis

`keramos helm-compat report` walks a Helm chart directory and prints which template constructs keramos supports natively, which ones the `keramos migrate` translator can rewrite automatically, and which ones will need manual review after migration. The output is a per-file inventory: file → construct count → translation outcome. The command is read-only and does not produce a keramos package — that's `keramos migrate`'s job.

## When to use it

Run before `keramos migrate` to understand the scope of post-migration cleanup. A chart with mostly simple `{{ .Values.x }}` references and standard sprig calls migrates with little manual work; one with elaborate `{{ with $foo := ... }}` blocks, custom Go template helpers, or unusual `range` patterns will produce more review items. The report tells you which.

## What happens when you run it

1. Reads the Helm chart at `<chart-path>`.
2. Walks every `.tpl` and `.yaml` template file in `templates/`.
3. For each, parses the Go-template AST and classifies each node: native-supported, auto-translatable, or manual-review.
4. Prints a summary table to stdout.

## Usage

```
keramos helm-compat report <chart-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for report |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Report on a vendored upstream chart:

```sh
keramos helm-compat report ./vendor/postgresql
```

Pull a chart from an OCI registry, then report on it:

```sh
keramos registry pull oci://registry-1.docker.io/bitnamicharts/postgresql:15.0.0 -d ./pulled
keramos helm-compat report ./pulled/postgresql
```

Use the report to decide whether migration is worth it:

```sh
keramos helm-compat report ./vendor/postgresql > report.txt
grep -c "manual-review" report.txt
```

## See also

- [`helm-compat`](helm-compat.md)
- [`helm-compat export`](helm-compat-export.md)
- [`migrate`](migrate.md) — actually translate the chart
- [Migration guide](../guides/migration.md)
