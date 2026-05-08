# keramos apply

## Synopsis

`keramos apply` executes a plan file produced by `keramos plan`. The plan is consumed via `--plan <file>`; keramos verifies the plan's content hash, confirms the target release name and namespace match what's in the plan, then runs the install-or-upgrade exactly as the plan describes. Edits to the original package directory after the plan was generated have no effect — the plan is self-contained.

## When to use it

Use as the deploy step in plan-then-apply workflows. The plan is the source of truth for what's deployed; the apply step is mechanical and does not re-render. This decoupling lets a CI pipeline produce the artefact in one job and a separate, gated job apply it.

## What happens when you run it

1. Reads the plan file at `--plan`.
2. Verifies the plan's integrity hash.
3. Issues the action the plan describes (install or upgrade) with the same effect as `keramos install` / `keramos upgrade` would have at plan time — running hooks, applying the manifest, recording the new revision.
4. With `--dry-run client`, renders only without contacting the cluster; with `--dry-run server`, sends a server-side dry-run apply for validation without persisting.

## Usage

```
keramos apply [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dry-run` | string | "" | dry-run mode: `client` or `server` |
| `-h, --help` | bool | false | help for apply |
| `--plan` | string | "" | plan file produced by `keramos plan` |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Apply a saved plan:

```sh
keramos apply --plan hello-1.3.plan
```

Server-side dry-run before actually applying (final validation step):

```sh
keramos apply --plan hello-1.3.plan --dry-run server
```

CI pipeline (plan in one job, apply in another):

```sh
# job 1 — generate the plan
keramos plan hello ./my-app --action upgrade -o hello-1.3.plan

# job 2 (after review) — apply the plan
keramos apply --plan hello-1.3.plan
```

## See also

- [`plan`](plan.md) — produce the plan
- [`upgrade`](upgrade.md) — single-step alternative
- [`diff`](diff.md) — preview without committing to a plan
