# keramos login

## Synopsis

`keramos login` stores credentials for a package registry. The credentials are saved in `~/.config/keramos/credentials.json` keyed by host; subsequent `keramos pull`, `keramos install`, `keramos push` commands use them transparently.

## When to use it

Use when first interacting with a private repository or to refresh credentials on an existing one. For OCI registries specifically, prefer `keramos registry login` which is symmetric with `keramos registry logout`.

## What happens when you run it

1. Prompts for username/password if not supplied (interactive mode).
2. Validates credentials by attempting authentication against the host.
3. Stores the credential bundle (basic auth, bearer token, or API key) in `~/.config/keramos/credentials.json`, keyed by host.
4. Subsequent commands targeting that host pick up the credentials transparently.

## Usage

```
keramos login <host> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--api-key` | string | — | API key |
| `-h, --help` | — | — | help for login |
| `--insecure` | — | — | allow plaintext connections (credential storage is unchanged) |
| `-p, --password` | string | — | registry password (basic auth) |
| `--password-stdin` | — | — | read password from stdin (mutually exclusive with --password) |
| `--token` | string | — | bearer token |
| `-u, --username` | string | — | registry username (basic auth) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Log in to an HTTP repo:

```sh
keramos login charts.example.com
```

Log in non-interactively for CI (basic auth):

```sh
keramos login charts.example.com --username ci --password "$REPO_TOKEN"
```

Log in with a bearer token:

```sh
keramos login charts.example.com --token "$BEARER_TOKEN"
```

Log in with an API key:

```sh
keramos login charts.example.com --api-key "$API_KEY"
```

Read password from stdin (avoids exposing it in process listings):

```sh
echo "$REPO_TOKEN" | keramos login charts.example.com -u ci --password-stdin
```

## See also

- [`logout`](logout.md)
- [`registry`](registry.md)
- [Repositories guide](../guides/repositories.md)
