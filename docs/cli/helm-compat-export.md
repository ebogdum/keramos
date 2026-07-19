---
title: "keramos helm-compat export"
parent: "CLI"
---
{% raw %}
# keramos helm-compat export

`keramos helm-compat export` writes a keramos package out as a Helm v3 chart so that
tooling which only understands Helm's layout can read it.

## When to use it

Use it when something downstream expects a `Chart.yaml` — a registry that
scans for Helm charts, a linter, a GitOps tool. Note that keramos's `${...}`
expressions are copied verbatim: they resolve only under keramos, so the export
is for sharing static structure, not for running `helm install`. To hand Helm
fully rendered manifests instead, pre-render with `keramos template` and ship the
output.

## What happens

1. keramos reads `keramos.yaml` from `<keramos-package-path>` and writes a `Chart.yaml`
   with `apiVersion: v2` and the package's `name`, `version`, `description`,
   and `appVersion`.
2. It copies `values.yaml` across if the package has one.
3. It copies the `templates/` tree verbatim into the output directory. It
   refuses to follow any symlink under `templates/` and stops with an error if
   it finds one.
4. It writes everything under `--out`. If you omit `--out`, it writes to a
   temporary directory named `keramos-helm-export-<package>`.
5. It prints `exported helm-compat chart to <dir>`.

## Usage

```
keramos helm-compat export <keramos-package-path> [flags]
```

## Flags

| Flag | Type | Default | Effect |
|---|---|---|---|
| `--out` | string | temp dir | write the Helm chart into this directory instead of a temporary one |

Also inherits the global flags.

## Worked example

Export the package in `./my-app` into `./helm-export`:

```sh
keramos helm-compat export ./my-app --out ./helm-export
```

Output:

```
exported helm-compat chart to ./helm-export
```

Given `./my-app/keramos.yaml`:

```yaml
name: my-app
version: 1.4.0
appVersion: "2.1.0"
description: Example web app
```

keramos writes `./helm-export/Chart.yaml`:

```yaml
apiVersion: v2
appVersion: "2.1.0"
description: Example web app
name: my-app
version: 1.4.0
```

Each field traces straight from `keramos.yaml`; `apiVersion: v2` is added so Helm
recognises the chart. Alongside it sit the copied `values.yaml` and the
`templates/` tree.

## See also

- [`helm-compat`](helm-compat.md)
- [`helm-compat report`](helm-compat-report.md)
- [`migrate`](migrate.md) — the reverse direction: Helm chart to keramos package
- [`template`](template.md) — render a keramos package to static manifests
{% endraw %}
