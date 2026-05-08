# keramos list

## Synopsis

`keramos list` enumerates releases. With no flags, it shows releases in the current namespace. `-A` lists releases across all namespaces. Output columns are name, namespace, revision, status, package name, package version, last-updated time, and the audit user who last touched the release.

## When to use it

Inventory question — "what's installed where?". Combine with `--deployed` / `--failed` / `--pending` to filter by status, `--filter '<regex>'` to filter by name, or `--selector` for label-based queries.

## What happens when you run it

1. Lists release-storage Secrets matching keramos's labelling (`managedBy=keramos`) in the requested namespace (or all namespaces with `-A`).
2. Decodes each release record's metadata (skipping the gzipped manifest for speed).
3. Applies any filters (`--deployed`, `--failed`, `--filter`, `--selector`, etc.).
4. Sorts by `--sort-by` (default name) and applies `--max`/`--offset`.
5. Renders the requested format. With `--short`, prints only release names — useful for shell loops.

## Usage

```
keramos list [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-a, --all` | — | — | show all statuses including superseded and failed |
| `-A, --all-namespaces` | — | — | list across all namespaces |
| `-d, --date` | — | — | shortcut for --sort-by date |
| `--deployed` | — | — | show only deployed releases |
| `--failed` | — | — | show only failed releases |
| `--filter` | string | — | regex filter on release name |
| `-h, --help` | — | — | help for list |
| `-m, --max` | int | — | maximum number of releases to display (0 = unlimited) |
| `--offset` | int | — | skip the first N releases after sorting |
| `-o, --output` | string | "table" | output format: table, json, yaml |
| `--pending` | — | — | show only pending releases |
| `--reverse` | — | — | reverse the sort order |
| `-l, --selector` | string | — | label selector (key=value,...) applied to release labels |
| `-q, --short` | — | — | output release names only |
| `--sort-by` | string | "name" | sort by: name, date, revision |
| `--superseded` | — | — | show only superseded releases |
| `--uninstalled` | — | — | show only uninstalled releases (with --keep-history) |
| `--uninstalling` | — | — | show only releases currently being uninstalled |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

List releases in the current namespace:

```sh
keramos list
```

List every release in the cluster:

```sh
keramos list -A
```

Only failed releases:

```sh
keramos list -A --failed
```

Filter by name with a regex:

```sh
keramos list -A --filter '^api-'
```

Releases matching a label selector:

```sh
keramos list -A --selector tier=backend,env=prod
```

Names-only output for shell loops (e.g. iterate every release in a namespace):

```sh
for r in $(keramos list -n prod --short); do keramos drift $r -n prod; done
```

Sort by most-recently-deployed first:

```sh
keramos list -A --sort-by date --reverse
```

## See also

- [`status`](status.md)
- [`history`](history.md)
- [`get`](get.md)
