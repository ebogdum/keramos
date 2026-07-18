# keramos plan

## Synopsis

`keramos plan` renders a package directory and compares it against the current
stored **state** — the Terraform `plan` for keramos. It shows what applying that
directory would add, change, or destroy, and (by default) *where each change
comes from*: the source template file, and the origin of every changed value.

It also writes a portable, apply-able plan artifact (rendered manifest +
parameters + content hash) that `keramos apply` can execute later, unchanged.

## When to use it

- Before an upgrade, to review exactly what will change and why.
- In change-management flows: CI produces the JSON artifact, a human reviews
  the diff, a deploy stage runs `keramos apply --plan`.

## What happens when you run it

1. Renders `[package-path]` (default `.`) against the merged values
   (`--profile` + `-f` + `--set*`).
2. Derives the release identity from the package's `keramos.yaml` `name` (unless
   `-r/--release` is given) and reads its **latest** stored state.
3. Diffs the render against that state — every field shown, so an edited label
   is never hidden — and annotates each change with its provenance.
4. Optionally writes the apply-able JSON artifact (`--out`, or `--format json`).

No resources are applied. The cluster is read only to fetch the stored state,
and that is best-effort: with no reachable cluster or no prior state, every
resource is reported as a create.

## Usage

```
keramos plan [package-path] [flags]
```

The release name is **not** a positional argument — it comes from `keramos.yaml`.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-r, --release` | string | (from keramos.yaml) | state to compare against; overrides the derived name |
| `--profile` | string | — | profile to apply |
| `-f, --values` | stringArray | — | values file (repeatable) |
| `--set` | stringArray | — | key=value (repeatable) |
| `--set-string` | stringArray | — | key=value forced as string (repeatable) |
| `--action` | string | "install" | action the artifact represents: install or upgrade |
| `-o, --out` | string | "-" | write the JSON plan artifact to this file |
| `--format` | string | "text" | stdout format: `text` (change preview) or `json` (artifact) |
| `--no-color` | — | — | disable colored diff output |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Plan the package in the current directory against its state:

```sh
cd ./mychart
keramos plan
```

Output (in → out):

```
keramos plan: update  mychart / apps  (package .)

~ update  Deployment/mychart-api
      from: deployment.yaml
      ~ spec.replicas
          - 1   (state)
          + 3   ← set (replicas=3)
      ~ spec.template.spec.containers.0.image
          - "registry/api:1.4.0"   (state)
          + "registry/api:1.5.0"   ← values-file (prod.yaml)

Plan: 0 to add, 1 to change, 0 to destroy.
```

Read the origins: `spec.replicas` changed because of `--set replicas=3`; the
image changed because `prod.yaml` set it. `from:` names the template to open.

Compare against a differently-named state:

```sh
keramos plan -r prod-web .
```

Produce the apply-able artifact and apply it later:

```sh
keramos plan --out plan.json
keramos apply --plan plan.json
```

Emit the artifact to stdout for a pipeline:

```sh
keramos plan --format json > plan.json
```

## See also

- [`apply`](apply.md) — execute a plan artifact
- [`diff`](diff.md) — compare two files/versions (no cluster)
- [`drift`](drift.md) — compare package, state, and the live cluster
- [`upgrade`](upgrade.md)
