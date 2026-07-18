# keramos diff

## Synopsis

`keramos diff` is a **purely file-oriented** comparison — it never reads cluster
or release state. It renders and compares local inputs: two package
directories, two rendered manifest files, one package under two value sets, or
one package at two git revisions. Use it to answer "what is different between
these two things on disk", independent of any cluster.

For "what would change against the recorded state" use [`keramos plan`](plan.md);
for "what differs from the live cluster" use [`keramos drift`](drift.md).

## When to use it

- Compare two chart versions before upgrading a dependency.
- See how `staging` values differ from `prod` values for the same package.
- Review what a git change to a package actually does to the rendered output.

## The four modes

| Invocation | Compares |
|---|---|
| `keramos diff ./a ./b` | two package directories (renders both) |
| `keramos diff a.yaml b.yaml` | two rendered manifest files (no rendering) |
| `keramos diff ./pkg --to-set k=v` | one package under two value sets |
| `keramos diff ./pkg --from-ref X --to-ref Y` | one package at two git revisions |

The mode is chosen from the arguments: two directories → mode 1; two files →
mode 2; one directory with `--from-*/--to-*` value flags → mode 3; one
directory with `--from-ref/--to-ref` → mode 4.

## Usage

```
keramos diff <a> [b] [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-f, --values` | stringArray | — | values file applied to BOTH sides (repeatable) |
| `--set` | stringArray | — | key=value applied to BOTH sides (repeatable) |
| `--set-string` | stringArray | — | key=value (string) applied to BOTH sides |
| `--profile` | string | — | profile applied to BOTH sides |
| `--from-values` | stringArray | — | values file for the FROM side only (mode 3) |
| `--to-values` | stringArray | — | values file for the TO side only (mode 3) |
| `--from-set` | stringArray | — | key=value for the FROM side only (mode 3) |
| `--to-set` | stringArray | — | key=value for the TO side only (mode 3) |
| `--from-profile` | string | — | profile for the FROM side only (mode 3) |
| `--to-profile` | string | — | profile for the TO side only (mode 3) |
| `--from-ref` | string | — | git revision for the FROM side (mode 4) |
| `--to-ref` | string | — | git revision for the TO side (mode 4, default: working tree) |
| `--smart` | bool | true | smart per-resource diff; `--smart=false` for raw unified diff |
| `--no-color` | — | — | disable colored output |

## Examples

Compare two package versions (in → out):

```sh
keramos diff ./chart-v1 ./chart-v2
```

```
diff: ./chart-v1 → ./chart-v2

~ update  Deployment/myapp
      ~ spec.template.spec.containers.0.image
          - "nginx:1.24"
          + "nginx:1.25"

Summary: 0 added, 1 changed, 0 removed.
```

Compare two rendered manifest files:

```sh
keramos template ./a > a.yaml
keramos template ./b > b.yaml
keramos diff a.yaml b.yaml
```

Compare the same package under staging vs prod values:

```sh
keramos diff ./chart --from-values staging.yaml --to-values prod.yaml
```

Compare a package across git revisions:

```sh
keramos diff ./chart --from-ref v1.2.0 --to-ref v1.3.0
```

Fall back to a raw line-level unified diff:

```sh
keramos diff ./chart-v1 ./chart-v2 --smart=false
```

## See also

- [`plan`](plan.md) — compare a package against the recorded state
- [`drift`](drift.md) — compare package, state, and the live cluster
- [`template`](template.md) — render a package to a manifest
