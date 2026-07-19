---
title: "keramos publish"
parent: "CLI"
---
{% raw %}
# keramos publish

Upload a packaged `.keramos.tgz` archive to a registry.

## When to use it

- As the last step of a release, once `keramos package` has produced an archive.
- To push to an HTTP API registry (`--repo`) or an OCI registry (`--oci`).
- In CD: package once, then publish that same file to one or both targets.

## What happens

1. keramos checks that `<archive.keramos.tgz>` exists and ends in `.keramos.tgz`, then
   reads the `keramos.yaml` inside it to recover the package name and version.
2. You choose exactly one target: `--repo <url>` or `--oci <ref>`. If you give
   neither (or both), keramos stops with an error.
3. For `--repo`, keramos POSTs the archive as a multipart upload to
   `<url>/v1/packages`, attaching any credential you stored with
   [`keramos login`](login.md) for that host.
4. For `--oci`, keramos pushes the archive as an OCI artifact to the reference.
5. On success it prints `Published <name>@<version> to <target>`.

The upload has a 5-minute timeout. A non-2xx HTTP response fails the command
and prints the registry's status code and message.

## Usage

```
keramos publish <archive.keramos.tgz> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--repo` | string | — | HTTP API registry URL; the archive is uploaded to `<url>/v1/packages` |
| `--oci` | string | — | OCI registry reference to push the archive to |

Relevant global flags:

| Flag | Type | Description |
|---|---|---|
| `--allow-plaintext-auth` | — | permit sending credentials over plaintext HTTP |
| `--oci-plain-http` | — | use plain HTTP for OCI registries |
| `--oci-insecure-skip-tls-verify` | — | skip TLS verification for OCI registries |
| `--debug` | — | enable debug output |

## Worked example

Build an archive, then publish it to an HTTP registry:

```sh
keramos package ./my-app -d ./build
```

```
Successfully packaged to: ./build/my-app-1.0.0.keramos.tgz
```

```sh
keramos publish ./build/my-app-1.0.0.keramos.tgz --repo https://registry.example.com
```

```
Published my-app@1.0.0 to https://registry.example.com
```

Push the same archive to an OCI registry instead:

```sh
keramos publish ./build/my-app-1.0.0.keramos.tgz --oci oci://ghcr.io/example/charts/my-app
```

```
Published my-app@1.0.0 to OCI registry oci://ghcr.io/example/charts/my-app
```

## See also

- [`package`](package.md) — build the archive to publish
- [`login`](login.md) — store the credential the upload uses
- [`pull`](pull.md) — download a published package
- [`registry`](registry.md)
{% endraw %}
