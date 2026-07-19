---
title: "keramos history"
parent: "CLI"
---
{% raw %}
# keramos history

`keramos history` prints every stored revision of one release, oldest first, so
you can see how it reached its current state.

## When to use it

- To find the revision number to hand to [`rollback`](rollback.md).
- To review what changed across a release's life — package versions, statuses,
  and the description recorded on each revision.

## What happens

1. Loads every stored revision of `<release-name>` in the target namespace.
2. Sorts them by revision number, ascending — the newest revision is the last
   row.
3. Keeps only the most recent `--max` revisions when that flag is set.
4. Prints the result as a table, `json`, or `yaml`.

This reads stored records only; it does not query live resources. It requires a
reachable cluster to read those records. If the release has no history, it
prints `no history found for release <name>`.

## Usage

```
keramos history <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--max` | int | 0 | keep only the most recent N revisions; 0 shows all of them |
| `-o, --output` | string | "table" | render as `table`, `json`, or `yaml` |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | namespace of the release |

## Worked example

**INPUT — the four stored revisions of release `web` in namespace `apps`.** It
was installed, upgraded twice, then rolled back:

```
revision 1   deployed as web-1.4.0   at 2026-07-18 09:00:00   "Install complete"
revision 2   deployed as web-1.5.0   at 2026-07-18 11:30:00   "Upgrade complete"
revision 3   deployed as web-1.6.0   at 2026-07-18 11:55:00   "Upgrade complete"
revision 4   deployed as web-1.4.0   at 2026-07-18 12:05:00   "Rollback to 1"
```

**Show the last two revisions:**

```sh
keramos history web -n apps --max 2
```

**OUTPUT:**

```
REVISION    STATUS        PACKAGE      UPDATED                DESCRIPTION
3           superseded    web-1.6.0    2026-07-18 11:55:00    Upgrade complete
4           deployed      web-1.4.0    2026-07-18 12:05:00    Rollback to 1
```

**Tracing the output back to the input:**

| Output | Which input it read | Why |
|---|---|---|
| revisions 1 and 2 absent | `--max 2` | only the two most recent revisions are kept |
| row 4 last, not first | ascending sort | newest revision is the bottom row |
| revision 3 `superseded` | it is no longer current | only the newest revision stays `deployed` |
| revision 4 `PACKAGE web-1.4.0` | the rollback's manifest | rev 4 re-applied revision 1, so it carries `web-1.4.0` |
| `DESCRIPTION Rollback to 1` | the description on rev 4 | recorded when the rollback ran |

Drop `--max` and all four revisions print, revision 1 first.

## See also

- [`rollback`](rollback.md) — re-apply one of these revisions
- [`status`](status.md) — the current revision on its own
- [`audit`](audit.md) — who ran each revision and with which flags
- [`get`](get.md) — the manifest or values at a specific revision
{% endraw %}
