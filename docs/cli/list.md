# keramos list

`keramos list` enumerates releases in a namespace, one row per release, showing
only the latest revision of each.

## When to use it

- To answer "what is installed where?" across a namespace or the whole cluster.
- To narrow an inventory by status, name, or label before acting on it.
- To feed release names into a shell loop with `--short`.

## What happens

1. Reads the stored release records in the target namespace, or across all
   namespaces with `-A`.
2. Collapses each release to its latest revision.
3. Filters the set: by default it hides `superseded` and `failed`; `--all`
   shows every status, and the per-status flags (`--deployed`, `--failed`,
   `--pending`, `--superseded`, `--uninstalling`, `--uninstalled`) restrict to
   those statuses. `--filter` matches the name by regex, `--selector` matches
   release labels.
4. Sorts by `--sort-by` (`name`, `date`, or `revision`), optionally reversed,
   then applies `--offset` and `--max`.
5. Prints the result — a table, `json`, `yaml`, or bare names with `--short`.

This reads stored records only; it does not query live resources. It requires a
reachable cluster to read those records.

## Usage

```
keramos list [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-a, --all` | bool | false | include `superseded` and `failed` releases that are hidden by default |
| `-A, --all-namespaces` | bool | false | list across every namespace instead of just the current one |
| `-d, --date` | bool | false | shortcut for `--sort-by date` |
| `--deployed` | bool | false | keep only releases whose status is `deployed` |
| `--failed` | bool | false | keep only releases whose status is `failed` |
| `--filter` | string | — | keep only releases whose name matches this regular expression |
| `-m, --max` | int | 0 | print at most this many releases after sorting; 0 means unlimited |
| `--offset` | int | 0 | skip the first N releases after sorting (for paging) |
| `-o, --output` | string | "table" | render as `table`, `json`, or `yaml` |
| `--pending` | bool | false | keep only pending releases (pending install, upgrade, or rollback) |
| `--reverse` | bool | false | reverse the sort order |
| `-l, --selector` | string | — | keep only releases whose labels match this `key=value,...` selector |
| `-q, --short` | bool | false | print release names only, one per line |
| `--sort-by` | string | "name" | order rows by `name`, `date`, or `revision` |
| `--superseded` | bool | false | keep only `superseded` releases |
| `--uninstalled` | bool | false | keep only uninstalled releases whose history was kept |
| `--uninstalling` | bool | false | keep only releases currently being uninstalled |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | namespace to list (ignored with `-A`) |

## Worked example

**INPUT — the latest revision of every release stored in namespace `apps`:**

```
web        revision 3   deployed   web-1.4.0     updated 2026-07-18 12:05:00
api        revision 7   deployed   api-2.1.0     updated 2026-07-18 08:15:00
cache      revision 2   failed     cache-0.9.0   updated 2026-07-17 22:40:00
```

**List the namespace:**

```sh
keramos list -n apps
```

**OUTPUT** (note `cache` is absent — `failed` is hidden by default):

```
NAME    NAMESPACE    REVISION    STATUS      PACKAGE    VERSION    UPDATED
api     apps         7           deployed    api        2.1.0      2026-07-18 08:15:00
web     apps         3           deployed    web        1.4.0      2026-07-18 12:05:00
```

**Tracing the output back to the input:**

| Output | Which input it read | Why |
|---|---|---|
| `cache` missing | its status is `failed` | default filter hides `failed` and `superseded`; add `--all` or `--failed` to see it |
| `api` printed before `web` | default sort is by `name` | `api` sorts before `web` alphabetically |
| `web` `REVISION 3` | web's latest revision | list collapses each release to its newest revision |
| `PACKAGE api` + `VERSION 2.1.0` | `api-2.1.0` | the stored `name-version` is split into two columns |

Add `--all` and the `cache` row appears; add `--sort-by date --reverse` and
`web` (12:05) sorts above `api` (08:15).

## See also

- [`status`](status.md) — the fuller record for a single release
- [`history`](history.md) — every revision of one release
- [`get`](get.md) — values, manifest, and notes for a release
