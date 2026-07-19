# keramos prune

`keramos prune` deletes old superseded revisions from a release's history, keeping
only the most recent N so stored state does not grow without bound.

## When to use it

- A long-lived release under heavy CD churn has piled up dozens of revisions
  and you want to reclaim stored history.
- You want to trim every release in a namespace to a fixed rewind window in
  one command.
- Run it with `--dry-run` first to see exactly which revisions would go.

## What happens

1. Requires `--keep` to be at least 1; otherwise it errors.
2. Builds the list of releases to process: the one named by `--release`, or
   every release in the namespace when `--release` is empty.
3. For each release, sorts revisions newest-first and keeps the most recent
   `--keep`. The currently deployed revision is always kept, even if it falls
   outside that window.
4. Deletes every remaining older revision record. With `--dry-run` it prints
   what it would delete and removes nothing.
5. Prints each pruned revision and a final total.

This trims stored revision history only; live cluster resources are never
touched. Mutating unless `--dry-run` is set. Requires a reachable cluster to
read and delete the stored state.

## Usage

```
keramos prune [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--keep` | int | 10 | number of most-recent revisions to retain per release; must be at least 1 |
| `--release` | string | — | prune only this release; empty prunes every release in the namespace |
| `--dry-run` | — | false | list the revisions that would be deleted without deleting them |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Worked example — trim a release with a long history

**INPUT — a release with many revisions,** the newest deployed:

```sh
keramos history mychart -n apps
# REVISION  STATUS       ...
# 15        deployed
# 14        superseded
# 13        superseded
# ...
# 1         superseded
```

**Preview a prune down to the last 3, without deleting anything:**

```sh
keramos prune --release mychart --keep 3 --dry-run -n apps
```

**OUTPUT (dry run):**

```
would prune mychart revision 12 (status=superseded)
would prune mychart revision 11 (status=superseded)
...
would prune mychart revision 1 (status=superseded)
12 revision(s) pruned
```

**Run it for real:**

```sh
keramos prune --release mychart --keep 3 -n apps
```

**OUTPUT:**

```
pruned mychart revision 12
pruned mychart revision 11
...
pruned mychart revision 1
12 revision(s) pruned
```

**Tracing the output:**

| Output | Cause |
|---|---|
| revisions 15, 14, 13 not listed | `--keep 3` retains the three newest |
| revision 15 kept | it is the deployed revision — always retained |
| `pruned … revision 12` down to `1` | older superseded revisions beyond the keep window |
| `12 revision(s) pruned` | 15 revisions minus the 3 kept |
| `would prune …` (dry run) | `--dry-run` reported the same set but deleted nothing |

## See also

- [`history`](history.md) — list the revisions prune trims
- [`uninstall`](uninstall.md) — remove a release and its resources entirely
- [`rollback`](rollback.md) — revert to a retained revision
