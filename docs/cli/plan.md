# keramos plan

## Synopsis

`keramos plan` renders an upgrade and persists it to a portable plan file (rendered manifest + parameters + content hash). The plan can later be applied with `keramos apply` exactly as planned, even on a different machine. Plans bind to a specific release name and namespace; you cannot apply a plan elsewhere by accident.

## When to use it

Use in change-management workflows where the "what" is reviewed and approved separately from the "do it". CI generates the plan as an artifact, a human reviewer signs off, then a deploy stage applies the plan via `keramos apply --plan <file>`.

## What happens when you run it

1. Renders `<package-path>` against the merged values (`--profile` + `-f` + `--set*`).
2. Resolves which action the plan represents (`install` or `upgrade`).
3. Captures the rendered manifest, the merged values, the package metadata, and a content hash into a structured plan file.
4. Writes to `--out` (`-` = stdout, default).
5. No cluster contact, no resources applied.

## Usage

```
keramos plan <release> <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--action` | string | "install" | action the plan represents: install or upgrade |
| `-h, --help` | — | — | help for plan |
| `--labels` | stringArray | — | label key=value (repeatable) |
| `-o, --out` | string | "-" | plan output file (- for stdout) |
| `--profile` | string | — | profile name to apply |
| `--set` | stringArray | — | key=value (repeatable) |
| `--set-string` | stringArray | — | key=value forced as string (repeatable) |
| `-f, --values` | stringArray | — | values file (repeatable) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Generate a plan for a fresh install:

```sh
keramos plan hello ./my-app -n prod -o hello-1.3.plan
```

Generate a plan for an upgrade with overrides applied:

```sh
keramos plan hello ./my-app --action upgrade -f overrides.yaml --set image.tag=1.3.0 -o hello-1.3.plan
```

Apply the saved plan in a separate (review-gated) step:

```sh
keramos apply --plan hello-1.3.plan
```

Pipe stdout to file via shell redirection (equivalent to `-o`):

```sh
keramos plan hello ./my-app > hello-1.3.plan
```

## See also

- [`apply`](apply.md)
- [`diff`](diff.md)
- [`upgrade`](upgrade.md)
