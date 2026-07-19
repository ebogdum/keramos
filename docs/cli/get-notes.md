# keramos get notes

`keramos get notes` prints the rendered NOTES text that keramos stored for a release
at install or upgrade time.

## When to use it

- You want to re-read the post-install message after it scrolled off screen.
- You want an older revision's notes with `--revision`.

## What happens

It loads the stored release record for `<release>` (latest, or `--revision`) and
prints the record's `notes` field — the NOTES template resolved against the
merged values at install time. If the release stored no notes, it prints `No
notes available for this release.` instead.

## Flags

| Flag | Cause | Effect |
|---|---|---|
| `--revision <n>` | you name a stored revision | prints that revision's notes instead of the latest |
| `-o, --output <fmt>` | you pass `raw`, `json`, or `yaml` | accepted for parity with sibling commands, but ignored — notes are always printed raw |

Inherits the global flags (`-n/--namespace`, `--kube-context`, `--kubeconfig`,
`--debug`).

## Usage

```
keramos get notes <release> [flags]
```

## Worked example

Stored record for `hello`, its `notes` field:

```
# what keramos recorded at install
Hello is deployed.

Reach it at:  http://hello.prod.svc.cluster.local:8080
Tail logs:    kubectl logs -n prod deploy/hello -f
```

Run it:

```sh
keramos get notes hello -n prod
```

Output — the stored notes, printed as-is:

```
Hello is deployed.

Reach it at:  http://hello.prod.svc.cluster.local:8080
Tail logs:    kubectl logs -n prod deploy/hello -f
```

A release with no notes prints:

```
No notes available for this release.
```

## See also

- [`get`](get.md) — the parent command
- [`get all`](get-all.md) — notes plus the rest of the record
