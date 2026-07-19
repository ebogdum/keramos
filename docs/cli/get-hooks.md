# keramos get hooks

`keramos get hooks` lists the hooks recorded for a release together with each
hook's last-run result.

## When to use it

- You want to confirm which hooks ran, and whether they succeeded, on the last
  install or upgrade.
- You want the rendered hook manifests as structured output with `-o yaml`/`json`.

## What happens

It loads the stored release record for `<release>` (latest, or `--revision`) and
prints its hook results. If a revision executed no hooks but stored rendered
hook templates (for example a package run with `--no-hooks`), those templates
are listed as `rendered (not yet executed in this revision)` so they stay
visible. If the release has neither, it prints `No hooks found for this
release.`

## Flags

| Flag | Cause | Effect |
|---|---|---|
| `--revision <n>` | you name a stored revision | reads that revision instead of the latest |
| `-o, --output <fmt>` | you pass `table`, `json`, or `yaml` (default `table`) | prints the hooks in that format |

Inherits the global flags (`-n/--namespace`, `--kube-context`, `--kubeconfig`,
`--debug`).

## Usage

```
keramos get hooks <release> [flags]
```

## Worked example

Stored record for `hello`, its `hooks` results:

```yaml
# what keramos recorded after the last upgrade
- name: pre-upgrade-backup
  kind: Job
  status: Succeeded
- name: post-upgrade-verify
  kind: Job
  status: Failed
```

Default table view:

```sh
keramos get hooks hello -n prod
```

```
NAME                  KIND  STATUS
pre-upgrade-backup    Job   Succeeded
post-upgrade-verify   Job   Failed
```

As JSON, filtered to the failed hook:

```sh
keramos get hooks hello -n prod -o json | jq '.[] | select(.status == "Failed")'
```

```json
{
  "name": "post-upgrade-verify",
  "kind": "Job",
  "status": "Failed"
}
```

## See also

- [`get`](get.md) — the parent command
- [`get all`](get-all.md) — hooks plus the rest of the record
- [`history`](history.md) — the revisions whose hooks you can inspect
