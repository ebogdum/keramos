# keramos show all

`keramos show all` prints a package's chart metadata, default values, and README
in one document.

## When to use it

- Survey an unfamiliar package's identity, configuration surface, and docs in a
  single command.
- Review a freshly pulled package before installing it.

## What happens

1. Loads the package metadata from `keramos.yaml` and prints it under a `# Chart`
   heading.
2. Prints `values.yaml`, when present, under a `# Values` heading.
3. Prints the first of `README.md` / `README.txt` / `README`, when present,
   under a `# README` heading.

Missing values or README sections are skipped; only `keramos.yaml` is required.
No cluster is contacted.

## Usage

```
keramos show all <package-path>
```

## Flags

Inherits the global flags.

## Worked example

**INPUT** — the `webapp` package on disk:

```
webapp/keramos.yaml     apiVersion: keramos/v1, name: webapp, version: 2.1.0
webapp/values.yaml   name: webapp, replicas: 2
webapp/README.md     "# webapp\n\nInstall notes for the webapp package."
```

**OUTPUT** (`keramos show all webapp`) — the three files combined under headings:

```
# Chart
apiVersion: keramos/v1
name: webapp
version: 2.1.0

# Values
name: webapp
replicas: 2

# README
# webapp

Install notes for the webapp package.
```

Each heading maps to one source file, so you read the whole package top to
bottom. For a single slice, use the dedicated subcommand.

## See also

- [`show`](show.md) — the show command index
- [`show chart`](show-chart.md) — chart metadata only
- [`show values`](show-values.md) — default values only
- [`show readme`](show-readme.md) — README only
