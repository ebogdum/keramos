---
title: "keramos login"
parent: "CLI"
---
{% raw %}
# keramos login

Store credentials for a package registry so later pushes and pulls to that
host authenticate automatically.

## When to use it

- Before you `keramos publish`, `keramos pull`, or `keramos registry push` against a
  registry that requires authentication.
- Once per host — the credential is saved and reused until you `keramos logout`.
- To record, per host, that you accept an untrusted transport (`--insecure`).

## What happens

1. You give a `<host>` (e.g. `registry.example.com`) and one credential: a
   username/password pair, a bearer `--token`, or an `--api-key`.
2. keramos writes the credential to `~/.config/keramos/credentials.json`, keyed by
   host. The file is created mode `0600` and its parent directory `0700`, so
   other users on the machine cannot read it.
3. It prints `Login succeeded for <host>`.
4. From then on, commands that talk to that host (`publish`, `pull`,
   `registry push`/`pull`) attach the stored credential — you do not pass it
   again.

Supply exactly one credential type. If more than one is given, keramos uses the
bearer token first, then the API key, then basic auth.

## Usage

```
keramos login <host> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-u, --username` | string | — | username for basic auth; pair with `--password` |
| `-p, --password` | string | — | password for basic auth |
| `--password-stdin` | — | — | read the password from stdin instead of `--password` (the two are mutually exclusive) — keeps it out of shell history and process listings |
| `--token` | string | — | bearer token, sent as `Authorization: Bearer <token>` |
| `--api-key` | string | — | API key credential |
| `--insecure` | — | — | record that this host may be reached over an untrusted transport (skip TLS verification / allow plain HTTP) |

Relevant global flags:

| Flag | Type | Description |
|---|---|---|
| `--allow-plaintext-auth` | — | permit sending credentials over plaintext HTTP; otherwise keramos refuses |
| `--debug` | — | enable debug output |

## Worked example

Log in with basic auth, then publish — the upload authenticates with no extra
flags:

```sh
keramos login registry.example.com -u alice -p s3cret
```

```
Login succeeded for registry.example.com
```

```sh
keramos publish ./my-app-1.0.0.keramos.tgz --repo https://registry.example.com
```

```
Published my-app@1.0.0 to https://registry.example.com
```

Keep the password off the command line by piping it in:

```sh
echo "$REGISTRY_PASSWORD" | keramos login registry.example.com -u alice --password-stdin
```

## See also

- [`logout`](logout.md) — remove the stored credential
- [`publish`](publish.md) — upload an archive to a registry
- [`pull`](pull.md) — download a chart from a repository
- [`install`](install.md)
- [`registry`](registry.md)
{% endraw %}
