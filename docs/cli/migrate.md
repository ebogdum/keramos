# keramos migrate

## Synopsis

`keramos migrate` translates a Helm chart directory into a keramos package. It walks the chart's structure (`Chart.yaml`, `templates/`, `values.yaml`, `crds/`, `_helpers.tpl`, `NOTES.txt`, `requirements.yaml`/`Chart.lock`) and emits an equivalent keramos package, rewriting go-template constructs to keramos `${...}` expressions where possible. Constructs that cannot be cleanly translated are flagged in a `keramos-migration.md` report inside the output directory.

## When to use it

Use when adopting an upstream Helm chart as a keramos-owned package. The output is a starting point that you then own and edit; the migrator is a translation tool, not a 1:1 emulator.

## Usage

```
keramos migrate <helm-chart-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dry-run` | — | — | show what would be converted without writing |
| `-h, --help` | — | — | help for migrate |
| `-o, --output` | string | — | output directory (default: <chart-name>-keramos/) |
| `--strict` | — | — | fail on any template that cannot be fully auto-converted |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Migrate an upstream Helm chart:

```sh
keramos migrate ./upstream-chart -d ./migrated/
```

Lint the migrated package:

```sh
keramos lint ./migrated/<chart-name>
```

## See also

- [Migration guide](../guides/migration.md)
- [`helm-compat`](helm-compat.md)
