# keramos pull

## Synopsis

`keramos pull` downloads a package archive from a source — an OCI registry (`oci://...`) or a chart name plus `--repo <url>` for an HTTP repository. By default the archive is saved as-is; with `--untar`, it's extracted into a directory. With `--prov`, the detached provenance signature is fetched alongside; with `--verify`, keramos validates the signature before saving.

## When to use it

Use to vendor a copy of an upstream package into your monorepo, to inspect a package's contents offline before installing, or to feed an air-gapped install workflow. For installing directly from a registry without a separate pull step, use `keramos install <release> oci://...:<tag>`.

## What happens when you run it

1. Resolves the source: OCI reference parsed from `<chart>` if it starts with `oci://`; otherwise treats `<chart>` as a name to look up in `<repo>/index.yaml`.
2. With `--version` set, picks the version satisfying the SemVer constraint; without it, picks the latest.
3. Downloads the archive (and `.prov`/sidecar if `--prov`) into `--destination`.
4. With `--verify`, validates the `.prov` against the local keyring before keeping the archive.
5. With `--untar`, extracts into `--untardir` (default `<dest>/<chart>`).

## Usage

```
keramos pull <chart> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--ca-file` | string | "" | CA bundle for HTTPS |
| `--cert-file` | string | "" | client certificate for HTTPS |
| `--destination` | string | . | directory to save the archive in |
| `-h, --help` | bool | false | help for pull |
| `--key-file` | string | "" | client key for HTTPS |
| `--prov` | bool | false | also download the `.prov` provenance sidecar |
| `--repo` | string | "" | HTTP repository URL containing `index.yaml` |
| `--untar` | bool | false | extract the archive after downloading |
| `--untardir` | string | "" | extraction directory (default: `<dest>/<chart>`) |
| `--verify` | bool | false | verify provenance signature before saving |
| `--version` | string | "" | specific version to pull (default: latest) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Pull from an HTTP repo, picking the highest 1.2.x version, and untar it:

```sh
keramos pull my-app --repo https://charts.example.com --version "^1.2.0" -d ./pulled --untar
```

Pull a specific OCI tag, verifying the provenance signature first:

```sh
keramos pull oci://ghcr.io/example/charts/my-app --version 1.2.3 --prov --verify -d ./pulled --untar
```

Pull with mutual TLS (private repo with cert auth):

```sh
keramos pull my-app --repo https://charts.example.internal --version 1.2.3 \
  --ca-file /etc/keramos/ca.pem \
  --cert-file /etc/keramos/client.crt \
  --key-file /etc/keramos/client.key
```

Pull and immediately install:

```sh
keramos pull    my-app --repo https://charts.example.com --version 1.2.3 -d ./pulled --untar
keramos install hello ./pulled/my-app -n staging --create-namespace
```

## See also

- [`install`](install.md)
- [`registry pull`](registry-pull.md) — OCI-only with extra flags
- [`repo add`](repo-add.md)
- [Signing guide](../guides/signing.md)
- [Repositories guide](../guides/repositories.md)
