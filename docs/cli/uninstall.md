# keramos uninstall

`keramos uninstall` deletes a release's resources from the cluster, keeping its
revision history for auditing unless you ask to purge it.

## When to use it

- To tear down a release you no longer need.
- To remove the resources but keep the history so you can still audit or
  reinstall (the default).
- To wipe the release completely, history included, with `--purge`.

## What happens

1. Reads the release's latest revision (the input state). With
   `--ignore-not-found`, a missing release exits zero instead of erroring.
2. Marks the release uninstalling, then runs pre-delete hooks unless `--no-hooks`.
3. Deletes the release's manifest from the cluster, in reverse apply order.
4. Waits until every deleted resource is actually gone unless `--no-wait`.
5. Runs post-delete hooks, then either keeps the stored history (default,
   status becomes superseded) or deletes it entirely with `--purge`.

Requires a reachable cluster. Purging history removes the record `rollback` and
`history` rely on.

## Usage

```
keramos uninstall <release-name> [flags]
```

The release name is the only positional argument; there is no package path.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--purge` | — | — | also delete the stored history (default keeps it) |
| `--keep-history` | — | on | keep the stored history (the default; explicit positive form) |
| `--no-hooks` | — | — | skip pre-delete and post-delete hooks |
| `--wait` | — | on | block until every deleted resource is gone (default behaviour) |
| `--no-wait` | — | — | return once deletion is requested, without confirming removal |
| `--timeout` | duration | 5m0s | how long the deletion wait may run before failing |
| `--ignore-not-found` | — | — | exit zero when the release doesn't exist |
| `--description` | string | — | free-text note recorded against the uninstall revision |
| `-o, --output` | string | table | result format: `table`, `json`, or `yaml` |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | namespace of the release |

## Worked example — the input state and what remains after

**INPUT 1 — the stored release.** `web` is deployed at revision 2 in namespace
`apps`:

```yaml
# keramos get web -n apps  →  before uninstall
name: web
namespace: apps
revision: 2
status: deployed
```

**INPUT 2 — the command.** You uninstall without `--purge`:

```sh
keramos uninstall web -n apps
```

**OUTPUT:**

```
release web uninstalled
release history kept (use --purge to remove)
```

**State after** (resources gone, history retained):

```yaml
# keramos history web -n apps  →  still lists the revisions
# keramos get web -n apps      →  status: superseded (record kept for audit)
name: web
status: superseded
```

**Tracing every line back to the inputs:**

| Output / state | Which input it came from | Why |
|---|---|---|
| `release web uninstalled` | INPUT 2 `<release-name>` | the named release's resources were deleted |
| `release history kept` | INPUT 2 (no `--purge`) | history is kept by default |
| resources gone from the cluster | `--wait` (default) | the command blocked until every resource was removed |
| `status: superseded` | history retained | the record stays for audit and possible reinstall |

Wipe the release completely, history included:

```sh
keramos uninstall web -n apps --purge
```

Make teardown idempotent in CI — exit zero even if it's already gone:

```sh
keramos uninstall web -n apps --ignore-not-found
```

## See also

- [`install`](install.md) — create a release
- [`purge`](purge.md) — remove keramos-managed resources in bulk
- [`history`](history.md) — list a release's revisions
- [`rollback`](rollback.md) — restore an earlier revision (needs kept history)
