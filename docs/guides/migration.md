---
title: "Migrate a Helm chart to a keramos package"
nav_order: 13
parent: "Guides"
---
{% raw %}
# Migrate a Helm chart to a keramos package

`keramos migrate` converts an existing **Helm chart** directory into a **keramos
package**: it walks the chart (`Chart.yaml`, `templates/`, `values.yaml`,
`crds/`, `_helpers.tpl`, `NOTES.txt`) and emits an equivalent keramos package,
rewriting Go-template constructs to keramos's `${...}` expressions, `$if` /
`$each` / `$switch` directives and `$text` blocks. The result is keramos YAML
only — no Helm survives in the output.

The command reference is [`keramos migrate`](../cli/migrate.md). The companion
[`keramos helm-compat`](../cli/helm-compat.md) runs an unmodified Helm chart under
keramos without converting it, and exports a keramos package back into Helm's layout.

## When to migrate

Migrate when you want to **own** an upstream chart as a keramos package long-term
and any of these apply:

- You want keramos's `${...}` expressions instead of Go-templates with sprig.
- You want keramos's ownership labels, drift detection, audit trail, and signing.
- You want to slot the chart into a keramos workspace beside keramos-native packages.

If you only need to run an upstream chart as-is, you do not need migration —
`keramos helm-compat install` renders and installs the unmodified chart under a
keramos release record.

## Size the job first

Before converting, `keramos helm-compat report` counts the Go-template logic in a
chart so you can gauge the work:

```sh
keramos helm-compat report ./redis
```

```json
{
  "chart": "redis",
  "templates": 4,
  "goTemplateBlocks": 36,
  "notes": [
    "_helpers.tpl: 7 Go-template blocks (run 'keramos migrate' to translate)",
    "deployment.yaml: 21 Go-template blocks (run 'keramos migrate' to translate)"
  ],
  "recommendations": [
    "Run 'keramos migrate ./redis' to translate go-template blocks to keramos's ${...} syntax"
  ]
}
```

A chart with few `{{ ... }}` blocks converts with little effort; one packed with
them needs more review afterward.

## What the migrator produces

| Helm input | keramos output |
|---|---|
| `Chart.yaml` | `keramos.yaml` (`apiVersion: keramos/v1`, layers/dependencies translated) |
| `values.yaml` | `values.yaml` |
| `values.schema.json` | `values.schema.json` |
| `templates/*.yaml` | `templates/*.yaml` (template body rewritten where possible) |
| `templates/_helpers.tpl` | `templates/_helpers.yaml` (named-template partials) |
| `templates/NOTES.txt` | `templates/notes.yaml` |
| `templates/tests/*` | `tests/*` |
| `crds/*.yaml` | `crds/*.yaml` |

Subcharts under `charts/` are converted too: each becomes a package under
`charts/<name>` in the output and is declared as a layer, carrying over the
dependency's `condition`, `tags` and `enabled`. A subchart's values are nested
under its own key so `.Values.x` inside it resolves to `values.<subchart>.x`,
while `.Values.global.x` stays global — the same scoping Helm gives subcharts.

Conversion is structural, not textual: the chart's templates are parsed, the
Go-template actions are compiled to `${...}` expressions, and each `if` / `with`
/ `range` block is mapped onto the YAML nodes it actually wraps. Conditional
sibling keys become `$merge` entries, conditional list items become spliced
`$merge` items, and loops become `$each` / `$as` / `$yield`. Control flow that
lives inside a literal block scalar is compiled to inline expressions
(`ternary`, `printf`) instead, because there is no YAML structure there to gate.

Inside templates it rewrites a curated set of Go-template constructs to keramos
expressions — for example:

| Go-template | keramos |
|---|---|
| `{{ .Values.x }}` | `${values.x}` |
| `{{ .Release.Name }}` | `${release.name}` |
| `{{ if .Values.enabled }}` … `{{ end }}` around a key | `$merge: { $if: ${values.enabled}, $then: … }` |
| `{{ range .Values.items }}` … `{{ end }}` | `$each: ${values.items}` / `$as` / `$yield` |
| `{{ $x := … }}` … | `$let: { x: … }` / `$in: …` |
| a loop that builds a config file | `$text:` with `$each` lines |
| `{{ include "named" . }}` | `${include "named"}` |
| `{{ include "named" (dict …) }}` | `${include "named" (dict …)}` — read as `$ctx` in the partial |
| `{{ .Capabilities.APIVersions.Has "x" }}` | `${Capabilities.APIVersions.Has "x"}` |
| `{{ toYaml .Values.x \| nindent 4 }}` | `${values.x \| toYaml \| nindent 4}` |

## What it cannot convert yet

The output contains no Helm. A construct the migrator cannot express in keramos
YAML is reported as a review item that names the file, the line and the
construct — so the gap is visible and closable, never smuggled into the package.

Measured over 100 upstream charts, the constructs still reported are:
templates that render text rather than YAML (validators built from `fail`, name
helpers with branching), `if`/`else` pairs where both branches set the same key,
and a handful of placement cases where a block spans half a YAML node.

## Convert

Point `keramos migrate` at the chart. The package is written into `-o/--output`
(default `<chart-name>-keramos/`); the conversion report prints to stdout — there
is no separate report file:

```sh
keramos migrate ./redis -o ./redis-keramos
```

```
Output: ./redis-keramos
Converted 8 files:
  - keramos.yaml
  - values.yaml
  - templates/_helpers.yaml
  - templates/deployment.yaml
  - templates/service.yaml
  - tests/test-connection.yaml
  - templates/notes.yaml
  - .keramosignore

Migration complete.
```

Constructs with no keramos form are listed before `Migration complete.`, each
with the file, line, reason and the original template text:

```
Items requiring manual review (1):
  templates/configmap.yaml:12 — if/else where both branches set the same key
    {{- if .Values.tls }}port: 6380{{ else }}port: 6379{{ end }}
```

Use `--dry-run` to see the report without writing anything, or `--strict` to
fail the command on anything left for review (useful as a CI gate).

## Review and finish

```sh
ls ./redis-keramos
```

```
keramos.yaml  values.yaml  templates/  tests/  .keramosignore
```

Lint the result, then render it and compare against `helm template` before you
trust it:

```sh
keramos lint ./redis-keramos
keramos template ./redis-keramos --release-name redis
```

The migrator is deterministic and idempotent — re-running on the same input
produces the same output — so the workflow is: migrate, resolve the review
items, commit, then re-migrate when the upstream chart releases a new version
and diff to apply the changes.

## The inverse: expose a keramos package to Helm tooling

`keramos helm-compat` also runs the other direction, for CI or GitOps tools that
only understand Helm:

```sh
# Render an unmodified Helm chart to manifests (like `helm template`):
keramos helm-compat render ./redis

# Install an unmodified Helm chart under a keramos release record:
keramos helm-compat install redis ./redis -n data

# Export a keramos package AS a Helm v3 chart:
keramos helm-compat export ./my-pkg --out ./helm-export
```

```
exported helm-compat chart to ./helm-export
```

`export` writes a `Chart.yaml` (`apiVersion: v2`) plus the copied `values.yaml`
and `templates/` tree. It is best-effort: keramos's `${...}` expressions are copied
verbatim and resolve only under keramos, so the exported chart is suitable for
static analysis, not for `helm install`. To hand Helm fully rendered manifests,
pre-render with `keramos template` and ship the output.

## Limits

`keramos migrate` is **template translation**, not behaviour emulation:

- Release-name interpolation works the same way, so generated resource names
  match — but tooling that scrapes keramos's release records reads a keramos-specific
  schema, not Helm's `sh.helm.release.v1...` secrets.
- Helm's `helm.sh/hook: test` test pattern is rewritten to keramos's lifecycle.

The goal is a working keramos package you then own and maintain — not to run Helm
under keramos forever.

## See also

- [`keramos migrate`](../cli/migrate.md) — command reference
- [`keramos helm-compat`](../cli/helm-compat.md) — render/install/export/report
- [`keramos helm-compat export`](../cli/helm-compat-export.md) ·
  [`keramos helm-compat report`](../cli/helm-compat-report.md)
- [`keramos lint`](../cli/lint.md) — validate the converted package
- [Workspaces](workspaces.md) — slot the migrated package into a workspace
{% endraw %}
