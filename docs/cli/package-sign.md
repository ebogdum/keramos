# keramos package sign

Sign an existing `.keramos.tgz` archive with a PGP private key, producing a
detached `.prov` provenance file.

## When to use it

- To sign an archive that was built without `--sign`, or to re-sign one after
  a key rotation.
- When the archive and the signing key live on different machines, so signing
  is a separate step from packaging.

## What happens

1. keramos reads `<archive.keramos.tgz>` and computes its digest.
2. It loads the PGP private key from `--key` (required) and produces a
   cleartext-signed provenance document over the archive.
3. It writes `<archive.keramos.tgz>.prov` next to the archive and prints
   `Successfully signed: <prov-path>`.

Anyone can later confirm the archive is untampered with
[`keramos package verify`](package-verify.md) using your public key.

## Usage

```
keramos package sign <archive.keramos.tgz> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--key` | string | — | path to the PGP private key file (required) |

## Worked example

Sign an archive, then verify it validates against the matching public key:

```sh
keramos package sign ./build/my-app-1.0.0.keramos.tgz --key ./cosign.key
```

```
Successfully signed: ./build/my-app-1.0.0.keramos.tgz.prov
```

```sh
keramos package verify ./build/my-app-1.0.0.keramos.tgz --keyring ./cosign.pub
```

The verify command exits 0 with no output, confirming the signature is valid.

## See also

- [`package`](package.md) — package and sign in one step (`--sign`)
- [`package verify`](package-verify.md) — check the signature
- [`keyring`](keyring.md) — manage trusted public keys
- [`publish`](publish.md)
