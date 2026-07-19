---
title: "keramos search hub"
parent: "CLI"
---
{% raw %}
# keramos search hub

`keramos search hub` queries Artifact Hub (or another compatible endpoint) for
packages matching a keyword, without you having to add any repository first.

## When to use it

Use it to discover community-published packages. Once you find one, add its
repository with [`repo add`](repo-add.md) and fetch it with [`pull`](pull.md).
To search only repositories you already trust, use
[`search repo`](search-repo.md).

## What happens

1. keramos sends your keyword to `<endpoint>/api/v1/packages/search`. The
   endpoint must be `https://` — a plain-HTTP endpoint is refused.
2. The hub returns up to `--max-results` packages of the `--kind` you asked
   for.
3. With `--regexp`, keramos additionally keeps only the packages whose name or
   description matches your keyword as a regular expression.
4. It prints one row per package — repository-qualified name, version, app
   version, repository URL, and description — under a header, with each column
   truncated to `--max-col-width`. With no matches it prints `No results found
   for "<keyword>"`.

No cluster is contacted and nothing is added to your repository list.

## Usage

```
keramos search hub <keyword> [flags]
```

## Flags

| Flag | Type | Default | Effect |
|---|---|---|---|
| `--endpoint` | string | `https://artifacthub.io` | query this Artifact Hub-compatible host instead of the default; must be `https://` |
| `--kind` | int | `0` | restrict to a package kind code from the index (e.g. `1`=Falco, `14`=OCI); `0` is the index default |
| `--max-results` | int | `20` | cap how many packages the hub returns |
| `--max-col-width` | int | `50` | truncate each output column to this many characters |
| `--regexp` | — | off | treat the keyword as a regular expression and drop rows whose name and description both fail to match |

Also inherits the global flags.

## Worked example

Search the default hub for an MQTT broker:

```sh
keramos search hub mqtt
```

Output:

```
NAME                         VERSION  APP VERSION  REPO URL                          DESCRIPTION
mosquitto/mosquitto          0.3.1    2.0.18       https://charts.example.io/mqtt    Eclipse Mosquitto MQTT broker
t3n/mosquitto                2.4.1    2.0.15       https://storage.googleapis.com/…  Mosquitto is a message broker
```

Each row is a hub hit for `mqtt`. The `REPO URL` column is the repository to
add next:

```sh
keramos repo add mosquitto https://charts.example.io/mqtt
keramos pull mosquitto/mosquitto
```

Narrow a noisy search to database charts with a regular expression:

```sh
keramos search hub '^(postgres|mysql|mariadb)' --regexp --max-results 5
```

## See also

- [`search`](search.md)
- [`search repo`](search-repo.md) — search added repositories instead
- [`repo add`](repo-add.md) — add a repository you found
- [`pull`](pull.md) — download a matched chart
{% endraw %}
