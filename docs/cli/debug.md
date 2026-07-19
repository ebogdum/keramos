---
title: "keramos debug"
parent: "CLI"
---
{% raw %}
# keramos debug

`keramos debug` renders a package and reports how it was resolved — the merged
values, the template files, and the final manifest — so you can see why a
value came out the way it did.

## When to use it

- A template rendered a surprising value and you want to see the merged
  values and template set behind it.
- You want the rendered manifest plus a short summary (package, template
  count, value count, warnings) in one command.
- Add `--trace` for a step-by-step breakdown of resolution and the merge.

## What happens

1. Resolves the package (metadata, layers, profile) and the merged values
   from package defaults plus any `-f`/`--set` overrides.
2. Renders the templates to a manifest and prints it.
3. Without `--trace`, appends a summary footer: the package `name-version`,
   the template count, the merged value count, and any warnings (for
   example, a missing `appVersion` or `notes.yaml`).
4. With `--trace`, prints labelled sections before the manifest —
   `PACKAGE RESOLUTION`, `VALUES MERGE`, `FINAL VALUES`, and
   `TEMPLATE FILES` — so you can trace each stage.

## Usage

```
keramos debug <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--trace` | bool | false | print step-by-step resolution sections before the rendered manifest |
| `--profile` | string | — | profile to apply before rendering |
| `-f, --values` | stringArray | — | values file override (repeatable) |
| `--set` | stringArray | — | `key=value` override (repeatable) |

## Worked example

Trace how `./myapp` resolves:

```sh
keramos debug ./myapp --trace
```

**OUTPUT (abridged):**

```
=== PACKAGE RESOLUTION ===
Package:     myapp
Version:     0.1.0

=== VALUES MERGE ===
Package defaults: 4 top-level keys
Value files:      0
Set overrides:    0
Final values:     4 top-level keys

=== FINAL VALUES ===
image:
    repository: nginx
    tag: latest
name: myapp
replicaCount: 1
service:
    port: 80

=== TEMPLATE FILES ===
  - deployment.yaml
  - notes.yaml
  - service.yaml

=== RENDERED OUTPUT ===
apiVersion: apps/v1
kind: Deployment
...
```

Read it top to bottom: the package resolves to `myapp 0.1.0`, no override
files or `--set` were supplied so the 4 package defaults survive unchanged
(`FINAL VALUES`), three templates render, and the manifest that follows uses
exactly those values — `image: nginx:latest`, `replicas: 1`.

Add a `--set` to see the merge shift:

```sh
keramos debug ./myapp --set replicaCount=3 --trace
```

`VALUES MERGE` then reports `Set overrides: 1` and `FINAL VALUES` shows
`replicaCount: 3`, which the rendered Deployment carries into
`spec.replicas`. Drop `--trace` for just the manifest plus the summary
footer.

## See also

- [`template`](template.md) — render without the resolution detail
- [`dev`](dev.md) — re-render continuously while you edit
- [`lint`](lint.md) — validate the package
- [`values`](values.md) — how values files and `--set` overrides merge
{% endraw %}
