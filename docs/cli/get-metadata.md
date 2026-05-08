# keramos get metadata

## Synopsis

`keramos get metadata` prints the high-level descriptor of a release: name, namespace, revision number, status (deployed / pending-upgrade / superseded / failed / uninstalled), the package reference (name, version, app version), first-deployed and last-deployed timestamps, and any labels attached to the release record. The manifest, values, hooks, and notes are NOT included — for those, use the dedicated subcommands (or `get all` for everything in one document).

## When to use it

Use for fast lookups when you just need to know which package version is behind a release, when it was last touched, or whether a previous operation completed cleanly. Cheaper than `get all` because it skips decoding the (potentially large) gzipped manifest.

## What happens when you run it

1. Reads the release-storage Secret for `<release-name>` at the requested revision.
2. Extracts the metadata fields (skipping manifest/values/hooks/notes).
3. Prints them as YAML or JSON.

## Usage

```
keramos get metadata <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for metadata |
| `-o, --output` | string | yaml | output format: json, yaml |
| `--revision` | int | 0 | get metadata from a specific revision (0 = current) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Quick status read for the current revision:

```sh
keramos get metadata hello -n prod
```

JSON for scripting (e.g. extract the package version):

```sh
keramos get metadata hello -n prod -o json | jq -r '.package.version'
```

Look back at revision 3's metadata to confirm what was running before the last upgrade:

```sh
keramos get metadata hello --revision 3 -n prod
```

## See also

- [`get`](get.md)
- [`get all`](get-all.md)
- [`status`](status.md) — current revision plus per-resource readiness
- [`history`](history.md) — every revision's metadata in one table
- [`keramos.yaml` reference](../reference/keramos-yaml.md)
