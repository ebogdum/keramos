# keramos rollback

## Synopsis

`keramos rollback` re-applies a previous revision of a release. Keramos retrieves the stored manifest from that revision, runs the revision's `pre-rollback` hooks (re-rendered from the version that originally shipped them, not from the current state), applies the manifest, then runs the revision's `post-rollback` hooks. The rollback itself becomes a new revision with `pendingRollback` → `deployed` status; the chronological history is preserved.

## When to use it

Use this when a recent upgrade caused a problem and you need to revert. Inspect `keramos history` to find the revision number to roll back to. Rollbacks are idempotent — rolling back to a revision that already matches current state is a no-op (a new revision is still recorded for the audit trail).

## What happens when you run it

1. Reads the release record and identifies the target revision (default: the previous revision; explicit `[revision]` overrides).
2. Marks the release status `pending-rollback`.
3. Runs the target revision's `pre-rollback` hooks (re-rendered from the hooks-as-stored at that revision).
4. Server-side applies the target revision's stored manifest. Resources currently present but absent in the target revision are deleted.
5. With the default `--wait`, blocks until workloads converge to Ready.
6. Runs the target revision's `post-rollback` hooks.
7. Records a new revision with the rollback metadata (parent revision, action `rollback`); status is `deployed`.

## Usage

```
keramos rollback <release-name> [revision] [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--cleanup-on-fail` | — | — | delete partially-applied resources if rollback fails |
| `--description` | string | — | rollback description |
| `--force` | — | — | delete and recreate resources to force update of immutable fields |
| `-h, --help` | — | — | help for rollback |
| `--history-max` | int | — | maximum revisions to retain in history (0 = unlimited) |
| `--no-hooks` | — | — | skip lifecycle hooks for this operation |
| `--no-wait` | — | — | don't wait for resources to be ready |
| `-o, --output` | string | "table" | output format: table, json, yaml |
| `--recreate-pods` | — | — | trigger a rolling restart of Deployments/StatefulSets/DaemonSets |
| `--timeout` | duration | 5m0s | timeout for readiness wait |
| `--wait` | — | — | wait for resources to be ready (default) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Roll back the most recent change (to revision 1 step prior):

```sh
keramos rollback my-app -n my-app-prod
```

Roll back to a specific revision:

```sh
keramos rollback my-app 5 -n my-app-prod
```

Roll back without re-running hooks (use only when hooks would interfere with revert):

```sh
keramos rollback hello 5 --no-hooks -n prod
```

Roll back and force replacement of immutable fields:

```sh
keramos rollback hello 5 --force -n prod
```

Roll back with a description recorded in the audit trail:

```sh
keramos rollback hello 5 --description "revert after CVE-2026-1234" -n prod
```

## See also

- [`history`](history.md)
- [`audit`](audit.md)
- [`upgrade`](upgrade.md)
- [Hooks guide](../guides/hooks.md)
