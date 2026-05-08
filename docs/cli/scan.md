# keramos scan

## Synopsis

`keramos scan` walks a directory of keramos packages, finds values and templates that are duplicated across them, and proposes (and optionally writes) a base layer that captures the shared content. Each scanned package is rewritten to reference the new base layer, so duplication is eliminated by composition rather than copy-paste.

## When to use it

Use as a one-shot refactor when you have several packages with copy-pasted values, helpers, or boilerplate templates and want to extract the common parts. Run with `--dry-run` first to preview what would change before letting keramos rewrite files in place.

## What happens when you run it

1. Reads every package directly under `<directory>` (one level of subdirectories).
2. Cross-references their `values.yaml` and `templates/*.yaml` to find common content.
3. Composes a base layer containing the shared values and templates.
4. Rewrites each scanned package's `keramos.yaml` to add the base layer as a `layers:` entry, and trims duplicated content from each package's local files.
5. With `--dry-run`, prints the proposed changes to stdout without modifying any file. Without it, files are written.

## Usage

```
keramos scan <directory> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dry-run` | bool | false | show what would be changed without writing files |
| `-h, --help` | bool | false | help for scan |
| `-o, --output` | string | "" | output directory for generated base (default: same as input) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Preview what scan would do without modifying anything:

```sh
keramos scan ./packages --dry-run
```

Run scan and write the base layer to a sibling directory:

```sh
keramos scan ./packages -o ./layers/common-base
```

Scan, then verify each rewritten package still lints:

```sh
keramos scan ./packages
for p in ./packages/*/; do keramos lint "$p"; done
```

## See also

- [`lint`](lint.md)
- [`dependency`](dependency.md)
- [Layers guide](../guides/layers.md)
