---
title: "keramos show crds"
parent: "CLI"
---
{% raw %}
# keramos show crds

`keramos show crds` prints the CRD YAML files a package ships under its `crds/`
directory.

## When to use it

- Audit the cluster-scoped CustomResourceDefinitions a package would install
  before you commit to them.
- Pipe the CRDs into a validator or linter without installing the package.

## What happens

1. Looks for a `crds/` directory under `<package-path>`. If none exists, prints
   `(no crds/ directory)` and stops.
2. Walks `crds/` for `.yaml` / `.yml` files, skipping symlinks, and sorts them
   by path.
3. Prints each file verbatim, joining them with a `---` separator.

No templating or merging is applied; the files pass through unchanged.

## Usage

```
keramos show crds <package-path>
```

## Flags

Inherits the global flags.

## Worked example

**INPUT** — a package whose `crds/` holds two files:

```
webapp/crds/gadgets.yaml
webapp/crds/widgets.yaml
```

**OUTPUT** (`keramos show crds webapp`) — files sorted by name (`gadgets` before
`widgets`) and separated by `---`:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gadgets.example.com
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  scope: Namespaced
```

A package with no `crds/` directory instead prints `(no crds/ directory)`.

## See also

- [`show`](show.md) — the show command index
- [`show chart`](show-chart.md) — the package metadata
- [`template`](template.md) — render the package's non-CRD manifests
{% endraw %}
