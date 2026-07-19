# keramos scan

`keramos scan` looks across a directory of keramos packages, finds the values and
templates they have in common, and extracts them into a shared base layer.

## When to use it

- When several packages have copy-pasted the same values or boilerplate
  templates and you want the shared parts factored into one base layer.
- As a one-shot refactor: run it once with `--dry-run` to see what it would
  pull out, then run it for real to write the base and rewrite each package.

Note: this command refactors packages. It does not scan for vulnerabilities —
for a supply-chain document use [`keramos sbom`](sbom.md), and for policy checks
use [`keramos policy`](policy.md).

## What happens

1. You point it at a directory. keramos finds the keramos packages inside it (at
   least two are required — commonality needs something to compare).
2. It loads each package's values and templates and computes what they share.
3. It prints a report: how many packages were scanned, how many common values
   and common templates it found.
4. If nothing is shared, it says so and stops without writing anything.
5. With `--dry-run`, it lists the base layer it would create and the packages
   it would update, but writes no files.
6. Without `--dry-run`, it writes the base layer (into `--output`, or beside
   the input by default) and rewrites each package to reference it.

## Usage

```
keramos scan <directory> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dry-run` | — | `false` | print the report and the planned changes, but write no files |
| `-o, --output` | string | (input dir) | write the generated `base` layer here instead of beside the input |

Inherits the global flags (`--debug`, `--kube-context`, `--kubeconfig`,
`-n/--namespace`); scan reads only local files, so they have no effect.

## Worked example

Preview what scan would extract from a directory of packages:

```sh
keramos scan ./packages --dry-run
```

Output:

```
Keramos Scan Report
========================================

Packages scanned: 4
Common values found: 6
Common templates found: 2

[DRY RUN] Would create base layer at: /home/you/packages/base
[DRY RUN] Would update packages:
  - web-api (/home/you/packages/web-api)
  - web-ui (/home/you/packages/web-ui)
  - worker (/home/you/packages/worker)
  - cron (/home/you/packages/cron)
```

Run it for real and write the base into a dedicated directory:

```sh
keramos scan ./packages -o ./layers
```

Output:

```
Keramos Scan Report
========================================

Packages scanned: 4
Common values found: 6
Common templates found: 2

Created base layer at: /home/you/layers/base
Updated package: web-api (/home/you/packages/web-api)
Updated package: web-ui (/home/you/packages/web-ui)
Updated package: worker (/home/you/packages/worker)
Updated package: cron (/home/you/packages/cron)
```

Then confirm each rewritten package still lints:

```sh
for p in ./packages/*/; do keramos lint "$p"; done
```

## See also

- [`lint`](lint.md) — validate the rewritten packages
- [`dependency`](dependency.md) — manage the layers scan produces
