---
title: "keramos status"
parent: "CLI"
---
{% raw %}
# keramos status

`keramos status` prints the recorded metadata for one release — its status,
current revision, package version, last-deployed time, and notes.

## When to use it

- Right after an install, upgrade, or rollback, to confirm which revision is
  now current and what status it settled on.
- As a quick single-release check when `list` gives you too much and a full
  `get` gives you too much.

## What happens

1. Loads the stored release record for `<release-name>` in the target
   namespace — the latest revision, or the one named by `--revision`.
2. Prints its status, revision, package, last-deployed timestamp, description
   (if any), and notes (if any).

This reads the stored release record only; it does not query live resources or
compare against the cluster. It requires a reachable cluster to read that
record. To compare the record against what is actually running, use
[`drift`](drift.md).

## Usage

```
keramos status <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-o, --output` | string | "table" | render as `table` (the labelled block below), `json`, or `yaml` |
| `--revision` | int | 0 | show a specific revision instead of the latest; 0 means latest |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | namespace of the release |

## Worked example

**INPUT — the stored record for the latest revision of release `web`.** This is
what keramos wrote at the last upgrade:

```
name:        web
namespace:   apps
status:      deployed
revision:    3
package:     web / 1.4.0
lastDeployed: 2026-07-18 12:05:00
description: Rollback to 1
notes:       "Browse to http://web.apps.svc:8080"
```

**Show its status:**

```sh
keramos status web -n apps
```

**OUTPUT:**

```
NAME:       web
NAMESPACE:  apps
STATUS:     deployed
REVISION:   3
PACKAGE:    web-1.4.0
UPDATED:    2026-07-18 12:05:00
DESCRIPTION: Rollback to 1

NOTES:
Browse to http://web.apps.svc:8080
```

**Tracing each line back to the record:**

| Output line | Field it read | Note |
|---|---|---|
| `STATUS: deployed` | `status` | the state of this revision |
| `REVISION: 3` | `revision` | latest, because no `--revision` was passed |
| `PACKAGE: web-1.4.0` | `package` name + version | joined as `name-version` |
| `UPDATED: 2026-07-18 12:05:00` | `lastDeployed` | when this revision was applied |
| `DESCRIPTION: Rollback to 1` | `description` | printed only when non-empty |
| `NOTES:` block | `notes` | printed only when non-empty |

Pass `--revision 2` and the same fields are read from revision 2 instead, so
`REVISION` and `PACKAGE` would report that older revision.

## See also

- [`list`](list.md) — the same fields for every release at once
- [`history`](history.md) — every revision of this release, not just one
- [`drift`](drift.md) — compare this record against the live cluster
- [`get`](get.md) — the full record: values, manifest, hooks, notes
{% endraw %}
