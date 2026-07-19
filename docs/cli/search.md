---
title: "keramos search"
parent: "CLI"
---
{% raw %}
# keramos search

## Synopsis

`keramos search` finds keramos packages to install. It looks in two places, one per
subcommand: the repositories you have already added locally, and the public
Artifact Hub index.

## Subcommands

| Command | What it queries |
|---|---|
| [`search repo`](search-repo.md) | the repositories you added with `keramos repo add` |
| [`search hub`](search-hub.md) | Artifact Hub (or a compatible endpoint) |

## Usage

```
keramos search <command> <keyword>
```

## See also

- [`repo`](repo.md) — add and manage the repositories `search repo` reads
- [`pull`](pull.md) — download a chart you found
- [`install`](install.md) — install a package as a release
{% endraw %}
