# keramos repo update

Re-fetch the index of every registered repository.

## When to use it

- After [`keramos repo add`](repo-add.md), so the new repository's charts become
  searchable and pullable.
- Before searching or pulling, to pick up newly published chart versions.

## What happens

Keramos walks your repository list and, for each entry, fetches its `index.yaml`
into the local cache under `~/.cache/keramos/indexes/`, using the credentials and
TLS material recorded for that repository. It prints one line per repository
and a final `Update complete.` Once the cache is fresh,
[`keramos search`](search.md) and [`keramos pull`](pull.md) see the current set of
charts.

A repository that cannot be reached is reported on its own line but does not
stop the others; pass `--fail-on-repo-update-fail` to make any failure exit
non-zero (useful in CI). With no repositories registered, keramos prints
`No repositories configured.`

## Usage

```
keramos repo update [flags]
```

## Flags

| Flag | Effect |
|---|---|
| `--fail-on-repo-update-fail` | Exit non-zero if any repository fails to update. |

## Worked example

```
$ keramos repo update
...successfully got an update from "my-charts"
...successfully got an update from "private"
Update complete.
```

Refresh, then search the freshly updated catalogues:

```
$ keramos repo update
...successfully got an update from "my-charts"
Update complete.

$ keramos search repo redis
NAME              CHART VERSION   APP VERSION   DESCRIPTION
my-charts/redis   1.4.0           7.2.4         In-memory data store
```

## See also

- [`repo add`](repo-add.md) — register a repository first
- [`search`](search.md) — search the updated indexes
- [`pull`](pull.md) — download a chart
