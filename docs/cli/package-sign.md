# keramos package sign

## Synopsis

`keramos package sign` produces a detached PGP `.prov` (provenance) signature alongside an existing `.keramos.tgz` archive. The `.prov` file is a cleartext-signed envelope that includes the archive's SHA-256 digest, package name, and version. Consumers verify it with `keramos package verify` or with `--verify` on `keramos pull` / `keramos install`. The `--key` flag points at the **private key file** — typically an exported PGP secret key in armoured form.

## When to use it

Use when packaging happened without `--sign` (e.g. an archive built upstream that you want to re-sign before redistribution) or when re-signing after a key rotation. For new archives, prefer `keramos package <pkg-dir> --sign --key <path>` which packages and signs in one shot.

## What happens when you run it

1. Reads the archive at `<archive.keramos.tgz>` and computes its SHA-256.
2. Loads the PGP private key at `--key`.
3. Constructs the provenance manifest (name, version, digest, signer fingerprint).
4. PGP-signs the manifest and writes `<archive.keramos.tgz>.prov` next to the archive.
5. No cluster contact, no network.

## Usage

```
keramos package sign <archive.keramos.tgz> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for sign |
| `--key` | string | "" | path to PGP private key file (required) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Sign an archive with an exported secret key file:

```sh
keramos package sign ./build/my-app-1.0.0.keramos.tgz --key /path/to/secret-key.asc
```

Result is `./build/my-app-1.0.0.keramos.tgz.prov` next to the archive.

Re-sign an archive after key rotation (overwrites the existing `.prov`):

```sh
keramos package sign ./build/my-app-1.0.0.keramos.tgz --key /path/to/new-key.asc
```

Sign and immediately verify in one shell pipeline:

```sh
keramos package sign   ./build/my-app-1.0.0.keramos.tgz --key /path/to/key.asc
keramos package verify ./build/my-app-1.0.0.keramos.tgz --keyring /path/to/pubkey.asc
```

## See also

- [`package`](package.md) — package and sign in one command
- [`package verify`](package-verify.md)
- [`keyring add`](keyring-add.md)
- [Signing guide](../guides/signing.md)
