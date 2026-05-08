# keramos registry pull

## Synopsis

`keramos registry pull` downloads a keramos package from an OCI distribution-spec registry and writes it as a `.keramos.tgz` archive on disk. The OCI reference includes the tag (`oci://host/name:tag`); there is no separate `--version` flag — the tag in the URI is the version selector. For HTTP-repository pulls or for SemVer-constraint resolution, use `keramos pull` instead.

## When to use it

Use when scripting OCI-specific workflows or when working against a registry that requires `--plain-http` / `--insecure-skip-tls-verify` (which keramos's unified `pull` does not expose). For everyday pulls, the more general `keramos pull` is preferable.

## What happens when you run it

1. Resolves the OCI reference (`oci://host/path:tag`) using stored credentials.
2. Pulls the artifact's manifest and blob.
3. Writes the blob to `<destination>/<name>-<tag>.keramos.tgz`.
4. Verifies the blob's digest against the manifest before considering the pull complete.

## Usage

```
keramos registry pull <ref> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-d, --destination` | string | . | directory to save the pulled package |
| `-h, --help` | bool | false | help for pull |
| `--insecure-skip-tls-verify` | bool | false | skip TLS certificate verification |
| `--plain-http` | bool | false | use plaintext HTTP (no TLS) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Pull a tagged version into the current directory:

```sh
keramos registry pull oci://ghcr.io/example/charts/my-app:1.2.3
```

Pull the `latest` tag into a specific directory:

```sh
keramos registry pull oci://ghcr.io/example/charts/my-app:latest -d ./pulled
```

Pull from an internal registry over plain HTTP (development only):

```sh
keramos registry pull oci://localhost:5000/charts/my-app:1.2.3 --plain-http
```

Pull from a registry with a self-signed TLS certificate (skip cert validation):

```sh
keramos registry pull oci://registry.local/charts/my-app:1.2.3 --insecure-skip-tls-verify
```

## See also

- [`registry`](registry.md)
- [`registry push`](registry-push.md)
- [`pull`](pull.md) — unified pull with SemVer-constraint resolution
- [OCI guide](../guides/oci.md)
