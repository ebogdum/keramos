# keramos values

`keramos values` resolves a package's values exactly as `install` would and prints
the merged result, or traces where a single key's value came from.

## When to use it

- Confirm the final values a package renders with before you install or upgrade.
- Answer "where did `replicas=5` come from?" by tracing one key across
  defaults, layers, values files, and `--set`.
- Export the merged values as JSON to feed another tool.

## What happens

1. Resolves the package at `<package-path>` and applies `--profile` if given.
2. Merges values in install order: package defaults (`values.yaml`) → imported
   layers → `-f` files → `--set` / `--set-string` / `--set-file` / `--set-json`.
   Later sources win.
3. With `--trace <key>`, prints only that key's contributors in apply order and
   marks the winner with `→`. Otherwise prints the merged tree as YAML
   (default) or JSON (`-o json`).

No cluster is contacted; this reads the package on disk only.

## Usage

```
keramos values <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-f, --values` | stringArray | — | merge this values file over defaults (repeatable) |
| `--set` | stringArray | — | override `key=value`, type-inferred (repeatable) |
| `--set-string` | stringArray | — | override `key=value`, forced to string |
| `--set-file` | stringArray | — | override `key=path`; value is read from the file |
| `--set-json` | stringArray | — | override `key=<json>`; value parsed as JSON |
| `--profile` | string | — | apply this profile before merging overrides |
| `--trace` | string | — | dotted key path; print only its resolution chain |
| `-o, --output` | string | "yaml" | merged output format: `yaml` or `json` (ignored with `--trace`) |

## Worked example

**INPUT** — `test/fixtures/simple/values.yaml`:

```yaml
name: myapp
replicas: 3
image:
  repository: nginx
  tag: latest
```

**Merged output** (`keramos values test/fixtures/simple`):

```yaml
image:
    repository: nginx
    tag: latest
name: myapp
replicas: 3
```

`replicas: 3` comes straight from the file because nothing overrides it.

**Override and trace it** (`keramos values test/fixtures/simple --set replicas=5
--trace replicas`):

```
replicas:
    package-default (values.yaml) = 3
  → set (replicas=5) = 5
```

The `--set` beats the `values.yaml` default, so the winner marked `→` is `5`.
Drop `--trace` and the merged tree now shows `replicas: 5`.

## See also

- [`config`](config.md) — build a values file interactively from the schema
- [`show values`](show-values.md) — print the raw default `values.yaml`
- [`template`](template.md) — render manifests using the resolved values
