---
title: "keramos migrate"
parent: "CLI"
---
{% raw %}
# keramos migrate

Convert a Helm chart directory on disk into an equivalent keramos package
directory.

## When to use it

- Adopting an upstream Helm chart as a keramos-owned package you will maintain
  going forward.
- Moving your own charts off Helm and onto keramos, one chart at a time.

The output is a keramos package you then own and edit. It contains no Helm:
whatever the migrator cannot express in keramos YAML is reported as a review
item naming the file, line and construct, never copied through. To keep running
an unmodified Helm chart as-is instead of converting it, use
[`keramos helm-compat`](helm-compat.md).

## What happens

1. Reads the Helm chart at `<helm-chart-path>` — its `Chart.yaml`, `templates/`
   (including subdirectories), `values.yaml`, `crds/` and `charts/`.
2. Parses every template, compiles the Go-template actions to `${...}`
   expressions, and attaches each `if` / `with` / `range` to the YAML nodes it
   wraps — conditional keys become `$merge`, loops become `$each`, text built
   from loops becomes `$text`, named helpers become partials in
   `templates/_helpers.yaml`.
3. Converts each subchart under `charts/` into a layer with its values nested
   under the subchart's key.
4. Writes the result to a new package directory — `<chart-name>-keramos/` by
   default, or wherever `-o/--output` points. With `--dry-run` nothing is
   written and the conversion is only reported.
5. Prints the output path, the list of converted files, any items that need
   manual review (file, line, reason and the original template text), and any
   warnings. `--strict` turns review items into a non-zero exit.

This works entirely on local files. It reads no cluster and installs nothing.

## Usage

```
keramos migrate <helm-chart-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-o, --output` | string | `<chart-name>-keramos/` | directory to write the converted package to |
| `--dry-run` | — | false | show what would be converted without writing anything |
| `--strict` | — | false | fail on any template that cannot be fully auto-converted |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Worked example

**INPUT — an upstream Helm chart directory:**

```sh
ls ./redis
```

```
Chart.yaml  values.yaml  templates/
```

```
./redis/templates/
  deployment.yaml
  service.yaml
  _helpers.tpl
  NOTES.txt
  tests/test-connection.yaml
```

**Convert it:**

```sh
keramos migrate ./redis -o ./redis-keramos
```

**OUTPUT — a keramos package plus a conversion report:**

```
Output: ./redis-keramos
Converted 8 files:
  - keramos.yaml
  - values.yaml
  - templates/deployment.yaml
  - templates/service.yaml
  - tests/test-connection.yaml
  - templates/notes.yaml
  - templates/_helpers.yaml
  - .keramosignore

Migration complete.
```

A construct with no keramos form is listed before `Migration complete.`, with
the original template text under it:

```
Items requiring manual review (1):
  templates/deployment.yaml:24 — if/else where both branches set the same key
    {{- if .Values.tls }}port: 6380{{ else }}port: 6379{{ end }}
```

```sh
ls -a ./redis-keramos
```

```
.keramosignore  keramos.yaml  values.yaml  templates/  tests/
```

Lint the result, then fix anything flagged for manual review:

```sh
keramos lint ./redis-keramos
```

## See also

- [`helm-compat`](helm-compat.md) — run an unmodified Helm chart without converting it
- [`adopt`](adopt.md) — bring existing in-cluster resources under keramos management
- [`lint`](lint.md) — validate the converted package
{% endraw %}
