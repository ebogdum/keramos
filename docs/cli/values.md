# keramos values

## Synopsis

`keramos values` resolves and prints the effective values for a **package directory** as `keramos install` would compute them: defaults → layers → environment → profile → `-f` files → `--set*` overrides. The output is the merged result, ready to feed into another tool. With `--trace <dotted.key>`, keramos instead prints the resolution chain for a single key — every contributor in the order it was applied, with the winning value marked. This answers the universal operator question "where did `image.tag=dev` come from?".

For values stored on an actual installed release, use `keramos get values <release>` — that reads from the cluster's release record, while `keramos values` is a render-time, offline computation.

## When to use it

Use during package authoring or during operator triage to confirm what values a render would see, especially when overrides come from many sources (multiple layers, environments, profiles, value files, CLI flags). `--trace` is invaluable when one specific key has the wrong value and you need to find the contributor that set it.

## What happens when you run it

1. Reads `<package-path>` and resolves layers (using `keramos.lock` if present).
2. Merges values: layer defaults → package `values.yaml` → `--profile` → `-f` files → `--set*` flags.
3. With `--trace`, records every contribution to the named key.
4. Prints the merged map (or the trace).
5. No cluster contact, no resources modified.

## Usage

```
keramos values <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for values |
| `-o, --output` | string | yaml | output format: yaml, json (ignored when `--trace` is set) |
| `--profile` | string | "" | profile to apply |
| `--set` | stringArray | — | set key=value overrides (repeatable) |
| `--set-file` | stringArray | — | set key=path; value is read from path |
| `--set-json` | stringArray | — | set key=<json>; value parsed as JSON |
| `--set-string` | stringArray | — | set key=value forcing string interpretation |
| `--trace` | string | "" | dotted key path; show only its resolution chain |
| `-f, --values` | stringArray | — | values file overrides (repeatable) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Merged values from a package directory using the package's defaults only:

```sh
keramos values ./my-app
```

Same package with overrides applied (mirrors what `keramos install` would see):

```sh
keramos values ./my-app -f overrides.yaml --set replicas=5 --profile prod
```

Resolution trace for one key — see every contributor:

```sh
keramos values ./my-app -f overrides.yaml --trace image.tag
```

JSON for piping:

```sh
keramos values ./my-app -o json | jq '.image'
```

## See also

- [`get values`](get-values.md) — values stored on a live release record
- [`debug`](debug.md)
- [Values guide](../guides/values.md)
- [`values.yaml` reference](../reference/values-yaml.md)
