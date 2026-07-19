# keramos rollback

`keramos rollback` re-applies a previous revision of a release and records the
result as a new revision, so history only ever grows forward.

## When to use it

- An `upgrade` shipped a bad revision and you want the last known-good state
  back in the cluster now.
- You need the revert itself recorded in history and the audit trail, not a
  silent hand-edit of the cluster.

## What happens

1. Resolves the release named by `<release-name>` in the target namespace and
   loads the manifest of the revision you name, or of the immediately previous
   revision when `[revision]` is omitted.
2. Applies that stored manifest to the cluster, running lifecycle hooks unless
   `--no-hooks` is set.
3. Waits for the applied resources to become ready (unless `--no-wait`), up to
   `--timeout`.
4. Records the result as a brand-new revision — the target revision is copied
   forward, not restored in place — and marks the prior current revision
   `superseded`.

This command mutates the cluster. It requires a reachable cluster and an
existing release with a prior revision to roll back to.

## Usage

```
keramos rollback <release-name> [revision] [flags]
```

`[revision]` is the revision number to roll back **to**. Omit it to target the
revision immediately before the current one.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--cleanup-on-fail` | bool | false | on a failed rollback, delete the resources it partially applied instead of leaving them behind |
| `--description` | string | — | set the description recorded on the new revision (shown by `history` and `status`) |
| `--force` | bool | false | delete and recreate resources so immutable fields can change |
| `--history-max` | int | 0 | cap retained revisions after this rollback; 0 keeps unlimited history |
| `--no-hooks` | bool | false | skip the release's lifecycle hooks for this rollback |
| `--no-wait` | bool | false | return once resources are applied, without waiting for readiness |
| `-o, --output` | string | "table" | render the resulting release as `table`, `json`, or `yaml` |
| `--recreate-pods` | bool | false | trigger a rolling restart of Deployments, StatefulSets, and DaemonSets |
| `--timeout` | duration | 5m0s | how long to wait for readiness before failing the rollback |
| `--wait` | bool | on | wait for resources to be ready; on by default, `--no-wait` turns it off |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | namespace of the release |

## Worked example

**INPUT — the stored history of release `web` before you roll back.** Revision 2
is current and shipped a broken image; revision 1 is the last good state:

```
REVISION    STATUS       PACKAGE      UPDATED                DESCRIPTION
1           superseded   web-1.4.0    2026-07-18 09:00:00    Install complete
2           deployed     web-1.5.0    2026-07-18 11:30:00    Upgrade complete
```

**Roll back to revision 1:**

```sh
keramos rollback web 1
```

**OUTPUT:**

```
release web rolled back to revision 1 (new revision 3)
```

**Resulting history — `keramos history web`:**

```
REVISION    STATUS       PACKAGE      UPDATED                DESCRIPTION
1           superseded   web-1.4.0    2026-07-18 09:00:00    Install complete
2           superseded   web-1.5.0    2026-07-18 11:30:00    Upgrade complete
3           deployed     web-1.4.0    2026-07-18 12:05:00    Rollback to 1
```

**Tracing the output back to the input:**

| Output | Which input it read | Why |
|---|---|---|
| `rolled back to revision 1` | the `1` you passed | the target revision whose manifest is re-applied |
| `new revision 3` | prior current was revision 2 | history grows forward: the next number after 2 is 3 |
| revision 3 `PACKAGE web-1.4.0` | copied from revision 1 | rev 3 re-applies rev 1's manifest, so it carries `web-1.4.0` |
| revision 2 now `superseded` | it was `deployed` before | the previous current revision is displaced by the rollback |

The cluster runs `web-1.4.0` again, and every step is a numbered, audited
revision — nothing was overwritten.

## See also

- [`history`](history.md) — list the revisions you can roll back to
- [`status`](status.md) — confirm the current revision after rolling back
- [`upgrade`](upgrade.md) — the forward operation rollback reverses
- [`audit`](audit.md) — who ran the rollback, when, and with which flags
