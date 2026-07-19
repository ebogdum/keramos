# keramos helm-compat

## Synopsis

`keramos helm-compat` bridges keramos and Helm in both directions. It renders and
installs unmodified upstream Helm charts under a keramos release record, exports a
keramos package into Helm's chart layout so Helm-only tooling can read it, and
reports how much of a Helm chart keramos would have to translate to adopt it.

## Subcommands

| Command | What it does |
|---|---|
| [`export`](helm-compat-export.md) | write a keramos package out as a Helm v3 chart |
| [`report`](helm-compat-report.md) | analyse a Helm chart and report its Go-template usage |
| `render` | render an unmodified Helm chart to manifests (like `helm template`) |
| `install` | render an unmodified Helm chart and apply it under a keramos release |

## Usage

```
keramos helm-compat <command> <path>
```

## See also

- [`migrate`](migrate.md) — convert a Helm chart into a native keramos package
- [`template`](template.md) — render a keramos package to manifests
