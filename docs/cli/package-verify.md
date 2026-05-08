# keramos package verify

## Synopsis

`keramos package verify` checks that a `.keramos.tgz` archive's detached `.prov` signature is valid against either a specified public-key file or the local keramos keyring. The signature binds the archive's SHA-256 digest to a signer's PGP key; a successful verification means the archive has not been modified since signing and the signer is one whose key you trust.

## When to use it

Use to verify provenance before installing a package — particularly an archive received over a non-trusted channel (HTTP without TLS, an attached email, a USB drive). For OCI-stored or HTTP-repo packages, `keramos pull --verify` and `keramos install --verify` perform the same check inline; this command is for archives already on disk.

## What happens when you run it

1. Reads the archive at `<archive.keramos.tgz>` and computes its SHA-256.
2. Reads the sibling `<archive.keramos.tgz>.prov` file.
3. Parses the cleartext-signed provenance and validates the embedded digest matches what was just computed.
4. Validates the PGP signature against the keyring (`--keyring` if specified, else `~/.config/keramos/keyring/`).
5. Exits 0 on success; non-zero with a precise reason on any mismatch.

## Usage

```
keramos package verify <archive.keramos.tgz> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for verify |
| `--keyring` | string | "" | public-key file or PGP keyring used for verification (defaults to ~/.config/keramos/keyring/) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Verify against the default keramos keyring:

```sh
keramos package verify ./build/my-app-1.0.0.keramos.tgz
```

Verify against a specific public-key file (bypasses the keyring):

```sh
keramos package verify ./build/my-app-1.0.0.keramos.tgz --keyring /path/to/signer.pub
```

Use as a CI gate:

```sh
keramos package verify ./build/my-app-1.0.0.keramos.tgz && \
  keramos install hello ./build/my-app-1.0.0.keramos.tgz -n staging
```

## See also

- [`package`](package.md)
- [`package sign`](package-sign.md)
- [`keyring`](keyring.md)
- [Signing guide](../guides/signing.md)
