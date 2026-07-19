# keramos get values

`keramos get values` prints the values recorded for a release — the user-supplied
overrides by default, or the fully merged values with `--all`.

## When to use it

- You want to confirm what an operator overrode at install or upgrade time.
- You want the merged values the templates actually saw, with `--all`.
- You want to recover a values file from a release with `-o json`/`yaml`.

## What happens

It loads the stored release record for `<release>` (latest, or `--revision`) and
prints one of two maps. By default it prints `userValues` — only the inline
overrides, value files, and `--set` flags the operator passed. With `--all` it
prints the merged `values` — package defaults, layers, profiles, and user
inputs combined, exactly as the templates saw them.

## Flags

| Flag | Cause | Effect |
|---|---|---|
| `--all` | you pass it | prints the merged values instead of just the user-supplied ones |
| `--revision <n>` | you name a stored revision | reads that revision instead of the latest |
| `-o, --output <fmt>` | you pass `json` or `yaml` (default `yaml`) | prints in that format |

Inherits the global flags (`-n/--namespace`, `--kube-context`, `--kubeconfig`,
`--debug`).

## Usage

```
keramos get values <release> [flags]
```

## Worked example

Stored record for `hello`. The operator overrode only the replica count; the
package default `image.tag: 1.5.0` was never overridden:

```yaml
# userValues (what the operator passed)
replicaCount: 3
# values (merged: defaults + userValues)
replicaCount: 3
image:
  repository: registry/hello
  tag: 1.5.0
```

Default — just the user-supplied values:

```sh
keramos get values hello -n prod
```

```yaml
replicaCount: 3
```

With `--all` — the full merged set the templates rendered against:

```sh
keramos get values hello -n prod --all
```

```yaml
image:
  repository: registry/hello
  tag: 1.5.0
replicaCount: 3
```

## See also

- [`get`](get.md) — the parent command
- [`get all`](get-all.md) — values plus the rest of the record
- `get provenance` — where each merged value came from
