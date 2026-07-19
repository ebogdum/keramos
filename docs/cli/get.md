# keramos get

`keramos get` reads back what keramos recorded for a release. Each subcommand prints
one slice of the stored release record — its values, rendered manifest, notes,
hooks, metadata, provenance, or the whole record at once. Nothing is
re-rendered; every subcommand loads the stored state and prints part of it.

## When to use it

- You want to see exactly what keramos applied for a release, without re-rendering
  the package.
- You need one part of the record — just the manifest, just the values — for a
  script or an audit.
- You want to read an older revision with `--revision <n>`.

## Subcommands

| Command | Returns |
|---|---|
| [`get values`](get-values.md) | the user-supplied values, or all merged values with `--all` |
| [`get manifest`](get-manifest.md) | the stored rendered Kubernetes manifest |
| [`get notes`](get-notes.md) | the stored NOTES text |
| [`get hooks`](get-hooks.md) | each hook and its last-run result |
| [`get metadata`](get-metadata.md) | metadata only — name, namespace, revision, status, package, labels |
| [`get all`](get-all.md) | the full record — metadata, values, manifest, hooks, and notes in one document |
| `get provenance` | where each stored value came from (default, values file, layer, profile, or `--set`) |

## Usage

```
keramos get <subcommand> <release> [flags]
```

The release name is a positional argument. Every subcommand accepts
`--revision <n>` to read a specific stored revision instead of the latest.

## Flags

`keramos get` itself has no flags beyond `-h/--help`; pick a subcommand. All
subcommands inherit the global flags (`-n/--namespace`, `--kube-context`,
`--kubeconfig`, `--debug`).

## See also

- [`history`](history.md) — list a release's revisions
- [`status`](status.md) — current status of a release
- [`releases`](releases.md) — list installed releases
