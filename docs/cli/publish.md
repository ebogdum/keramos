# keramos publish

## Synopsis

`keramos publish` uploads a packaged `.keramos.tgz` archive to a registry. The target is selected by flag — `--repo <url>` for an HTTP API registry, `--oci <ref>` for an OCI distribution-spec registry. For OCI-specific workflows with TLS and plain-HTTP options, prefer `keramos registry push`; `keramos publish` is the multi-target convenience wrapper.

## When to use it

Use as the publication step in CD pipelines. After `keramos package`, run `keramos publish` to push the archive to the destination.

## What happens when you run it

1. Reads the archive at `<archive.keramos.tgz>` and extracts its `keramos.yaml` for name and version.
2. Resolves credentials for the chosen target (HTTP repo creds via `keramos repo add` / `keramos login`; OCI creds via `keramos registry login` or `~/.docker/config.json`).
3. Uploads the archive — to `<repo>/api/charts` for HTTP repos, or as an OCI artifact tagged with the package version for OCI.
4. Reports success.

## Usage

```
keramos publish <archive.keramos.tgz> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for publish |
| `--oci` | string | "" | OCI registry reference |
| `--repo` | string | "" | HTTP API registry URL |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Publish to an HTTP API registry:

```sh
keramos publish ./build/my-app-1.0.0.keramos.tgz --repo https://charts.example.com
```

Publish to an OCI registry:

```sh
keramos publish ./build/my-app-1.0.0.keramos.tgz --oci oci://ghcr.io/example/charts/my-app
```

Publish to both targets in a CD job:

```sh
keramos publish ./build/my-app-1.0.0.keramos.tgz --repo https://charts.example.com
keramos publish ./build/my-app-1.0.0.keramos.tgz --oci  oci://ghcr.io/example/charts/my-app
```

## See also

- [`package`](package.md)
- [`registry push`](registry-push.md) — OCI with extra flags
- [`repo`](repo.md)
- [Repositories guide](../guides/repositories.md)
- [OCI guide](../guides/oci.md)
