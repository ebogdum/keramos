---
title: "keramos repo add"
parent: "CLI"
---
{% raw %}
# keramos repo add

Register an HTTP package repository under a name and URL.

## When to use it

- The first time you want to search or pull charts from a given repository.
- To attach basic-auth credentials or TLS material to a private repository.
- To replace an existing entry's URL or credentials with `--force-update`.

## What happens

Keramos adds a `<name> → <url>` entry to your repository list at
`~/.config/keramos/repositories.yaml` and prints a confirmation line. Any
`--username`/`--password` you pass are stored separately in the credential
store, keyed by the URL's host; TLS flags are recorded on the entry.

Adding a repository does not fetch its index yet — run
[`keramos repo update`](repo-update.md) to pull the index into the cache, after
which [`keramos search`](search.md) and [`keramos pull`](pull.md) can find its
charts. If the name already exists, keramos leaves it untouched and tells you so
unless you pass `--force-update`.

## Usage

```
keramos repo add <name> <url> [flags]
```

## Flags

| Flag | Effect |
|---|---|
| `--username` | Send this basic-auth username to the repository. |
| `--password` | Send this basic-auth password to the repository. |
| `--pass-credentials` | Keep sending credentials when the repo redirects. |
| `--pass-credentials-all` | Send credentials on every redirect hop. |
| `--ca-file` | Verify the repo's certificate against this CA bundle. |
| `--cert-file` | Present this client certificate for mutual TLS. |
| `--key-file` | Use this client key for mutual TLS. |
| `--insecure-skip-tls-verify` | Skip TLS certificate verification for this repo. |
| `--no-update` | Do nothing if the repository name already exists. |
| `--force-update` | Replace an existing entry of the same name. |

## Worked example

```
$ keramos repo add my-charts https://charts.example.com
"my-charts" has been added to your repositories

$ keramos repo list
NAME                 URL
my-charts            https://charts.example.com
```

Register a private repository with basic auth, then refresh its index:

```
$ keramos repo add private https://charts.internal --username u --password p
"private" has been added to your repositories

$ keramos repo update
...successfully got an update from "my-charts"
...successfully got an update from "private"
Update complete.
```

## See also

- [`repo update`](repo-update.md) — fetch the index after adding
- [`repo list`](repo-list.md) — confirm the entry was added
- [`login`](login.md) — store credentials for a registry host
- [`search`](search.md) · [`pull`](pull.md)
{% endraw %}
