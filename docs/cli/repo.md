---
title: "keramos repo"
parent: "CLI"
---
{% raw %}
# keramos repo

`keramos repo` manages the list of HTTP package repositories that keramos searches
and pulls charts from.

You register a repository under a short name and URL, refresh its index so
searches and pulls can see its charts, and remove it when you no longer need
it. `keramos repo index` covers the other side: building an `index.yaml` for a
directory of archives you want to serve as a repository.

The repository list lives at `~/.config/keramos/repositories.yaml`. Fetched
indexes are cached under `~/.cache/keramos/indexes/`.

## Subcommands

| Command | What it does |
|---|---|
| [`add`](repo-add.md) | Register a repository under a name and URL. |
| [`index`](repo-index.md) | Build an `index.yaml` for a directory of archives. |
| [`list`](repo-list.md) | Show the repositories you have registered. |
| [`remove`](repo-remove.md) | Drop a repository from your list. |
| [`update`](repo-update.md) | Re-fetch the index of every registered repository. |

## Usage

```
keramos repo [command] [flags]
```

## See also

- [`search`](search.md) — find charts across your registered repositories
- [`pull`](pull.md) — download a chart from a repository
- [`install`](install.md) — install a package as a release
{% endraw %}
