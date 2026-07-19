---
title: "keramos show readme"
parent: "CLI"
---
{% raw %}
# keramos show readme

`keramos show readme` prints a package's README to stdout.

## When to use it

- Read a package's install notes and configuration guidance from the terminal.
- Pipe the README into a Markdown renderer or pager.

## What happens

1. Looks in `<package-path>` for `README.md`, then `README.txt`, then `README`,
   and prints the first one it finds verbatim.
2. If none exists, exits with the error `no README found in package`.

The path may be a directory or a keramos archive; no cluster is contacted.

## Usage

```
keramos show readme <package-path>
```

## Flags

Inherits the global flags.

## Worked example

**INPUT** — `webapp/README.md` on disk:

```markdown
# webapp

Install notes for the webapp package.
```

**OUTPUT** (`keramos show readme webapp`) — the file's contents, unchanged:

```
# webapp

Install notes for the webapp package.
```

Pipe it through a renderer for nicer display, for example
`keramos show readme webapp | glow -`.

## See also

- [`show`](show.md) — the show command index
- [`show all`](show-all.md) — chart, values, and README together
- [`show chart`](show-chart.md) — the package metadata
{% endraw %}
