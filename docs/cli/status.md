# keramos status

## Synopsis

`keramos status` prints the current release's revision, package version, install timestamp, deployment status (`deployed`, `pending-upgrade`, `failed`, etc.), and a per-resource readiness summary. Use it to answer "is the release healthy right now?" without running a full diff or pulling the manifest.

## When to use it

Run after install, upgrade, or rollback to confirm the cluster reached the desired state. Useful in CI as a post-deploy assertion. For historical snapshots, pass `--revision`.

## What happens when you run it

1. Reads the release record at `<release-name>` (current revision unless `--revision` is set).
2. For each resource in the stored manifest, queries the live state in the cluster.
3. Composes a status summary: revision metadata + per-resource readiness.
4. Prints in the requested output format.

## Usage

```
keramos status <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for status |
| `-o, --output` | string | "table" | output format: table, json, yaml |
| `--revision` | int | — | show status of a specific revision |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Show status of a release:

```sh
keramos status my-app -n my-app-prod
```

Status of a specific revision:

```sh
keramos status my-app --revision 3 -n my-app-prod
```

Status as JSON for piping into other tooling:

```sh
keramos status my-app -n my-app-prod -o json | jq '.status'
```

## See also

- [`history`](history.md)
- [`drift`](drift.md)
- [`get`](get.md)
