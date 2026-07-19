---
title: "keramos repo list"
parent: "CLI"
---
{% raw %}
# keramos repo list

Show the repositories you have registered.

## When to use it

- To confirm a [`keramos repo add`](repo-add.md) took effect.
- To look up a repository's URL, for example to pass to `keramos pull --repo`.

## What happens

Keramos reads your repository list at `~/.config/keramos/repositories.yaml` and
prints every entry's name and URL. Nothing is fetched — this reads only local
configuration, with no network or cluster access. If you have not registered
any repositories, keramos prints `No repositories configured.`

## Usage

```
keramos repo list [flags]
```

## Flags

| Flag | Effect |
|---|---|
| `-o, --output` | Choose the output format: `table` (default), `json`, or `yaml`. |

## Worked example

```
$ keramos repo add my-charts https://charts.example.com
"my-charts" has been added to your repositories

$ keramos repo list
NAME                 URL
my-charts            https://charts.example.com
```

Get one repository's URL as JSON for scripting:

```
$ keramos repo list -o json | jq -r '.[] | select(.name=="my-charts") | .url'
https://charts.example.com
```

## See also

- [`repo add`](repo-add.md) — register a repository
- [`repo update`](repo-update.md) — refresh registered repositories
- [`search`](search.md) · [`pull`](pull.md)
{% endraw %}
