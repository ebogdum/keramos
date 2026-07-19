# keramos apply

`keramos apply` executes a plan artifact produced by `keramos plan`, applying the
exact change the plan describes to the cluster.

## When to use it

- As the deploy step of a plan-then-apply workflow: one job runs `keramos plan`,
  a reviewer inspects it, a later job runs `keramos apply`.
- When you want the applied change to be exactly what was reviewed, with no
  re-rendering surprises between review and rollout.

## What happens

1. Reads and parses the JSON plan file named by `--plan` (required).
2. Checks the plan is a supported `keramos/v1` `Plan` for an `install` or
   `upgrade` action, and rejects package paths that are absolute or contain
   `..`.
3. Re-renders the package client-side and recomputes the manifest's SHA-256.
   If it no longer matches the hash in the plan — because the package or a
   values file changed since the plan was generated — the apply is refused.
4. Runs the plan's `install` or `upgrade` against the cluster, atomically
   (rolls back on failure) and waiting up to 5 minutes for readiness.
5. Prints the applied action and the new revision number.

Mutating unless `--dry-run` is set. Requires a reachable cluster.

## Usage

```
keramos apply --plan <file> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--plan` | string | — | path to the JSON plan file from `keramos plan`; required, the apply fails without it |
| `--dry-run` | string | — | `client` renders without contacting the cluster; `server` sends a server-side dry-run for validation without persisting |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Worked example — apply a reviewed plan, and what a stale plan does

**INPUT — a plan written earlier by `keramos plan`:**

```sh
keramos plan ./mychart --action upgrade --out plan.json
# plan written to plan.json
```

`plan.json` carries the rendered manifest and its `manifestSha256` digest.

**Apply it:**

```sh
keramos apply --plan plan.json
```

**OUTPUT:**

```
applied upgrade for mychart revision 4
```

**Now suppose you edited `values.yaml` after generating the plan** and applied
the same file again:

```sh
keramos apply --plan plan.json
# Error: plan integrity check failed: package or values changed since plan
# was generated (expected sha 9f2c…, got 4a71…)
```

**Tracing the output:**

| Output | Cause |
|---|---|
| `applied upgrade` | the plan's `action` was `upgrade` |
| `revision 4` | the upgrade recorded a new revision on the release |
| `plan integrity check failed` | the re-rendered manifest's hash no longer matched the plan's `manifestSha256` — the package changed, so the reviewed plan is stale |

The integrity check guarantees what you apply is exactly what was planned;
regenerate the plan to pick up the edit.

## See also

- [`plan`](plan.md) — produce the plan artifact `apply` consumes
- [`upgrade`](upgrade.md) — single-step upgrade without a plan file
- [`diff`](diff.md) — compare packages, manifests, or revisions
