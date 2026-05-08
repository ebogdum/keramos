# keramos repo add

## Synopsis

`keramos repo add` registers an HTTP package repository with keramos. The local registration is just an entry in `~/.config/keramos/repositories.yaml` (name, URL, credentials, TLS material); subsequent `keramos repo update` calls fetch the repo's `index.yaml` into the local cache so `keramos search`, `keramos pull`, and dependency-resolution paths can find packages by name.

## When to use it

Use the first time you want to consume packages from a given repo, or to update the credentials / TLS settings of an already-registered repo (`--force-update`). For OCI registries, use `keramos registry login` instead — repos are HTTP-backed; OCI is its own auth model.

## What happens when you run it

1. Validates `<url>` reachable: a `GET` against `<url>/index.yaml` must succeed (or return a recognised auth challenge).
2. Adds (or replaces, with `--force-update`) the entry in `~/.config/keramos/repositories.yaml`.
3. Stores credentials in `~/.config/keramos/credentials.json` keyed by URL.
4. Fetches the repo's `index.yaml` into `~/.cache/keramos/indexes/<name>.yaml` for immediate use.

## Usage

```
keramos repo add <name> <url> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--ca-file` | string | "" | CA bundle to verify the repo's server certificate |
| `--cert-file` | string | "" | client certificate for mutual TLS |
| `--force-update` | bool | false | replace the existing repository entry |
| `-h, --help` | bool | false | help for add |
| `--insecure-skip-tls-verify` | bool | false | skip TLS certificate verification |
| `--key-file` | string | "" | client key for mutual TLS |
| `--no-update` | bool | false | do nothing if the repository already exists |
| `--pass-credentials` | bool | false | forward credentials on HTTP redirects |
| `--pass-credentials-all` | bool | false | send credentials on every HTTP redirect |
| `--password` | string | "" | HTTP basic-auth password |
| `--username` | string | "" | HTTP basic-auth username |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Register a public repo:

```sh
keramos repo add my-charts https://charts.example.com
```

Register a private repo with HTTP basic auth:

```sh
keramos repo add private https://charts.example.internal \
  --username u --password p
```

Register a repo with mutual TLS:

```sh
keramos repo add secure https://charts.example.com \
  --cert-file /etc/keramos/client.crt \
  --key-file  /etc/keramos/client.key \
  --ca-file   /etc/keramos/server-ca.pem
```

Replace credentials on an existing repo:

```sh
keramos repo add my-charts https://charts.example.com --username new-user --password new-pass --force-update
```

## See also

- [`repo`](repo.md)
- [`repo list`](repo-list.md)
- [`repo update`](repo-update.md)
- [`repo remove`](repo-remove.md)
- [`login`](login.md) — refresh credentials without re-adding
- [Repositories guide](../guides/repositories.md)
