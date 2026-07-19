# keramos package verify

Verify a `.keramos.tgz` archive's `.prov` signature against a public key or
keyring.

## When to use it

- Before installing an archive you received out of band, to confirm it was
  signed by a key you trust and has not been altered.
- As a gate in CI: verify, then install only if the signature checks out.

## What happens

1. keramos reads `<archive.keramos.tgz>` and its sibling `<archive.keramos.tgz>.prov`
   provenance file.
2. It computes the archive's digest and checks it against the digest recorded
   in the signed provenance.
3. It validates the PGP signature using the key material in `--keyring`
   (required) — either a single public-key file or a keyring.
4. On success the command exits 0 and prints nothing. On any mismatch —
   missing `.prov`, altered archive, or untrusted signer — it exits non-zero
   with a precise reason.

## Usage

```
keramos package verify <archive.keramos.tgz> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--keyring` | string | — | public-key file or PGP keyring to verify against (required) |

## Worked example

Verify a downloaded archive against the signer's public key:

```sh
keramos package verify ./my-app-1.0.0.keramos.tgz --keyring ./cosign.pub
```

No output and exit status 0 mean the signature is valid. Chain it as an
install gate — the install runs only if verification passes:

```sh
keramos package verify ./my-app-1.0.0.keramos.tgz --keyring ./cosign.pub && \
  keramos install my-app ./my-app-1.0.0.keramos.tgz -n staging
```

If the archive was tampered with, verify fails and the install never runs:

```
Error: signature verification failed
```

## See also

- [`package sign`](package-sign.md) — produce the signature
- [`package`](package.md)
- [`keyring`](keyring.md) — manage trusted public keys
- [`pull`](pull.md) · [`install`](install.md) — verify inline with `--verify`
