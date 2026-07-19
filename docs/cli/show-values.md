---
title: "keramos show values"
parent: "CLI"
---
{% raw %}
# keramos show values

`keramos show values` prints a package's default `values.yaml` unchanged.

## When to use it

- See the raw defaults a package ships, including the author's comments.
- Capture a starting point for your own overrides file.

## What happens

1. Reads `values.yaml` from `<package-path>` (a directory or a keramos archive).
2. Prints it verbatim to stdout. No layers, no merging, no overrides.

## Usage

```
keramos show values <package-path>
```

## Flags

Inherits the global flags.

## Worked example

**INPUT** — `test/fixtures/simple/values.yaml` on disk:

```yaml
name: myapp
replicas: 3
image:
  repository: nginx
  tag: latest
```

**OUTPUT** (`keramos show values test/fixtures/simple`) — byte-for-byte the same
file, defaults and layout preserved:

```yaml
name: myapp
replicas: 3
image:
  repository: nginx
  tag: latest
```

For the values a render actually uses after profiles and `--set` overrides,
use [`keramos values`](values.md) instead.

## See also

- [`show`](show.md) — the show command index
- [`values`](values.md) — merged, override-aware values
- [`show all`](show-all.md) — chart, values, and README together
{% endraw %}
