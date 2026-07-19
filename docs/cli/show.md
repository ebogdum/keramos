---
title: "keramos show"
parent: "CLI"
---
{% raw %}
# keramos show

`keramos show` prints a package's metadata, values, README, or CRDs without
installing it.

## Subcommands

| Command | Displays |
|---|---|
| [`keramos show chart`](show-chart.md) | the package metadata from `keramos.yaml` |
| [`keramos show values`](show-values.md) | the default `values.yaml` |
| [`keramos show readme`](show-readme.md) | the package's README file |
| [`keramos show crds`](show-crds.md) | the CRDs under the package's `crds/` directory |
| [`keramos show all`](show-all.md) | chart, values, and README in one document |

Each subcommand takes a `<package-path>` that is a directory or a keramos archive
(`.keramos.tgz`, `.tgz`, `.tar.gz`) and reads the file straight from disk.

## Usage

```
keramos show <chart|values|readme|crds|all> <package-path> [flags]
```

## See also

- [`values`](values.md) — resolve and trace the merged values
- [`template`](template.md) — render the package's manifests
- [`lint`](lint.md) — validate a package
{% endraw %}
