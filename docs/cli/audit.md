---
title: "keramos audit"
parent: "CLI"
---
{% raw %}
# keramos audit

`keramos audit` prints the chronological change log for one release — every
revision, who made it, what action it was, and when.

## When to use it

- To answer "who changed this release, and when?" for a SOC 2 / SLSA review,
  an incident, or a change-management sign-off.
- Before a rollback, to see which revision you want to return to.
- To feed the trail into another tool with `--output json` or `--output yaml`.

## What happens

1. You name a release. keramos reads its full revision history from the cluster.
2. Each revision prints as a row: revision number, action
   (install / upgrade / rollback), the user who ran it, its status, and the
   timestamp.
3. With `--revision N`, only that one revision is shown; the rest are dropped.
4. With `--output json` or `yaml`, the same records are emitted as structured
   data — including the recorded flags, value files, kubeconfig context, and
   hostname — instead of the table.
5. If the release has no history, keramos prints `no history for <name>` and
   exits cleanly.

## Usage

```
keramos audit <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-o, --output` | string | `table` | pick the rendering: `table` (summary rows), `json`, or `yaml` (full provenance) |
| `--revision` | int | `0` | show only this revision; `0` shows every revision |

### Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Worked example

Show the full trail for the `web-api` release:

```sh
keramos audit web-api -n prod
```

Output:

```
REVISION   ACTION     USER          STATUS       TIMESTAMP
1          install    alice@corp    superseded   2026-07-10 09:14:02
2          upgrade    bob@corp      superseded   2026-07-12 16:40:55
3          rollback   alice@corp    deployed     2026-07-15 11:22:07
```

Read one revision as JSON, with the flags and value files that produced it:

```sh
keramos audit web-api --revision 2 -n prod --output json
```

Output:

```json
[
  {
    "revision": 2,
    "action": "upgrade",
    "user": "bob@corp",
    "hostname": "ci-runner-7",
    "context": "prod",
    "flags": ["--set", "replicas=3"],
    "valueFiles": ["prod.yaml"],
    "parentRev": 1,
    "status": "superseded",
    "timestamp": "2026-07-12T16:40:55Z"
  }
]
```

A release that was never deployed prints:

```
no history for web-api
```

## See also

- [`history`](history.md) — shorter revision list for a release
- [`get`](get.md) — inspect the values, manifest, and notes of a revision
- [`rollback`](rollback.md) — return a release to an earlier revision
{% endraw %}
