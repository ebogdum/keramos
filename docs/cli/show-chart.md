---
title: "keramos show chart"
parent: "CLI"
---
{% raw %}
# keramos show chart

`keramos show chart` prints a package's `keramos.yaml` metadata unchanged.

## When to use it

- Verify a package's name, version, and apiVersion before installing.
- Inspect an unfamiliar or freshly pulled package's manifest from the terminal.

## What happens

1. Reads `keramos.yaml` from `<package-path>` (a directory or a keramos archive).
2. Prints it verbatim to stdout. No layer resolution, no value merging.

## Usage

```
keramos show chart <package-path>
```

## Flags

Inherits the global flags.

## Worked example

**INPUT** — `test/fixtures/simple/keramos.yaml` on disk:

```yaml
apiVersion: keramos/v1
name: simple-app
version: 1.0.0
```

**OUTPUT** (`keramos show chart test/fixtures/simple`) — the same manifest, so you
read the package's identity directly:

```yaml
apiVersion: keramos/v1
name: simple-app
version: 1.0.0
```

Pipe it through `yq` to pull a single field, for example
`keramos show chart test/fixtures/simple | yq '.version'` prints `1.0.0`.

## See also

- [`show`](show.md) — the show command index
- [`show values`](show-values.md) — the package's default values
- [`show all`](show-all.md) — chart, values, and README together
{% endraw %}
