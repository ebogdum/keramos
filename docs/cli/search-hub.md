# keramos search hub

## Synopsis

`keramos search hub` queries Artifact Hub (or another Artifact-Hub-compatible endpoint) for package archives matching a keyword. Results include package name, repository, version, and a short description; clicking through to the listed repository lets you `keramos repo add` and `keramos pull` to actually fetch the package. With `--regexp`, the keyword is treated as a regular expression rather than a substring.

## When to use it

Use to discover community-published packages without first adding any repository to keramos. The default endpoint is `https://artifacthub.io`; point at an internal hub mirror with `--endpoint`.

## What happens when you run it

1. Constructs a search request to `<endpoint>/api/v1/packages/search?ts_query_web=<keyword>`.
2. Filters by `--kind` if set.
3. Truncates to `--max-results`.
4. Renders a tabular view with columns capped at `--max-col-width`.
5. No cluster contact, no repo additions.

## Usage

```
keramos search hub <keyword> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--endpoint` | string | https://artifacthub.io | Artifact Hub-compatible endpoint |
| `-h, --help` | bool | false | help for hub |
| `--kind` | int | 0 | package kind code per the index endpoint (e.g. 1=Falco, 14=OCI) |
| `--max-col-width` | int | 50 | maximum column width for output |
| `--max-results` | int | 20 | maximum results to display |
| `--regexp` | bool | false | treat the keyword as a regular expression |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Search the default Artifact Hub for an MQTT broker:

```sh
keramos search hub mqtt
```

Use a regular expression to find any database-related package:

```sh
keramos search hub '^(postgres|mysql|mariadb)' --regexp
```

Cap output to a top-5 view:

```sh
keramos search hub redis --max-results 5
```

Search an internal Artifact Hub mirror:

```sh
keramos search hub redis --endpoint https://hub.example.internal
```

## See also

- [`search`](search.md)
- [`search repo`](search-repo.md) — search added repositories instead
- [`repo add`](repo-add.md)
- [`pull`](pull.md)
