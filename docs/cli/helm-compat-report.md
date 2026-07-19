---
title: "keramos helm-compat report"
parent: "CLI"
---
{% raw %}
# keramos helm-compat report

`keramos helm-compat report` analyses a Helm chart and reports how much
Go-template logic it contains, so you can gauge the work of moving it to keramos.

## When to use it

Run it before [`migrate`](migrate.md) to size the job. A chart with few
`{{ ... }}` blocks converts with little effort; one packed with them will need
more review after translation. The report is read-only — it inspects the chart
but produces no keramos package.

## What happens

1. keramos walks the `templates/` directory of `<chart-path>`, visiting every
   `.yaml`, `.yml`, and `.tpl` file.
2. It counts the template files and, in each, the number of Go-template
   blocks (occurrences of `{{`).
3. For every file that contains at least one block it adds a note with that
   file's block count.
4. It recommends running `keramos migrate` when any blocks are present, and notes
   how sub-charts are handled.
5. It prints the whole report to stdout as JSON.

## Usage

```
keramos helm-compat report <chart-path> [flags]
```

## Flags

Inherits the global flags.

## Worked example

Report on a vendored PostgreSQL chart:

```sh
keramos helm-compat report ./vendor/postgresql
```

Output:

```json
{
  "chart": "postgresql",
  "templates": 12,
  "goTemplateBlocks": 348,
  "notes": [
    "statefulset.yaml: 96 Go-template blocks (run 'keramos migrate' to translate)",
    "secrets.yaml: 41 Go-template blocks (run 'keramos migrate' to translate)"
  ],
  "recommendations": [
    "Run 'keramos migrate ./vendor/postgresql' to translate go-template blocks to keramos's ${...} syntax",
    "Sub-charts: drop them under <keramos-pkg>/charts/ unchanged; keramos layer-resolves them via dependencies in keramos.yaml"
  ]
}
```

Read it back to the chart: the `{{ ... }}` blocks in `templates/statefulset.yaml`
produce the `statefulset.yaml: 96 Go-template blocks` note, and the 348 across
all 12 files is the total translation surface. Because that total is above
zero, the first recommendation tells you the exact `keramos migrate` command to
run next.

## See also

- [`helm-compat`](helm-compat.md)
- [`helm-compat export`](helm-compat-export.md)
- [`migrate`](migrate.md) — actually translate the chart to a keramos package
- [`template`](template.md) — render a keramos package to manifests
{% endraw %}
