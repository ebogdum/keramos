---
title: "keramos get metadata"
parent: "CLI"
---
{% raw %}
# keramos get metadata

`keramos get metadata` prints a release's high-level metadata only — name,
namespace, revision, status, package, info, and labels — without the manifest,
values, hooks, or notes.

## When to use it

- You just need the package version, status, or timestamps for a release.
- You want a cheap lookup that skips the large manifest and values payloads.

## What happens

It loads the stored release record for `<release>` (latest, or `--revision`) and
prints only its metadata fields: `name`, `namespace`, `revision`, `status`,
`package`, `info` (timestamps and description), and `labels`. The manifest,
values, hooks, and notes are omitted — use [`get all`](get-all.md) for those.

## Flags

| Flag | Cause | Effect |
|---|---|---|
| `--revision <n>` | you name a stored revision | reads that revision instead of the latest |
| `-o, --output <fmt>` | you pass `json` or `yaml` (default `yaml`) | prints in that format |

Inherits the global flags (`-n/--namespace`, `--kube-context`, `--kubeconfig`,
`--debug`).

## Usage

```
keramos get metadata <release> [flags]
```

## Worked example

Stored record for `hello` (revision 4). Only the metadata fields matter here:

```yaml
name: hello
namespace: prod
revision: 4
status: deployed
package:
  name: hello
  version: 1.5.0
info:
  firstDeployed: "2026-06-01T09:00:00Z"
  lastDeployed:  "2026-07-18T14:22:00Z"
labels:
  team: platform
# ...plus manifest, values, hooks, notes (not printed by this command)
```

Run it:

```sh
keramos get metadata hello -n prod
```

Output — metadata only:

```yaml
info:
  firstDeployed: "2026-06-01T09:00:00Z"
  lastDeployed: "2026-07-18T14:22:00Z"
labels:
  team: platform
name: hello
namespace: prod
package:
  name: hello
  version: 1.5.0
revision: 4
status: deployed
```

Extract just the package version:

```sh
keramos get metadata hello -n prod -o json | jq -r '.package.version'
```

```
1.5.0
```

## See also

- [`get`](get.md) — the parent command
- [`get all`](get-all.md) — metadata plus the full record
- [`status`](status.md) — current revision plus per-resource readiness
- [`history`](history.md) — every revision's metadata in one table
{% endraw %}
