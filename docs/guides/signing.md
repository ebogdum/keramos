# Signing & verification

Keramos packages can be signed with PGP (detached `.prov` file) or cosign (OCI signature attached to the registry artifact). Verification can be required at install time, so the cluster never receives a package whose provenance hasn't been checked.

## Why sign

- **Tamper detection.** A modified archive fails verification.
- **Authorship attestation.** Signatures bind the archive to a key, the key to an identity.
- **Supply-chain audit.** `keramos audit <release>` records the signing key fingerprint or cosign certificate that authorised each install.

## PGP (detached `.prov`)

### Producing

```sh
keramos package ./my-app -d ./build --sign --key author@example.com --keyring ~/.gnupg/secring.gpg
```

Output:

```
build/
├── my-app-1.2.3.keramos.tgz
├── my-app-1.2.3.keramos.tgz.prov         # detached PGP signature + envelope
└── my-app-1.2.3.keramos.tgz.sha256       # plain hash, optional convenience
```

The `.prov` file embeds a manifest (package name, version, archive SHA-256) signed by the named key. Verification re-derives the archive's SHA-256 and compares it to the signed manifest.

The default keyring is `~/.gnupg/secring.gpg`. Override with `--keyring` (PGP secret keyring) and `--key` (key ID, fingerprint, or user-id email).

To sign an already-built archive:

```sh
keramos package ./build/my-app-1.2.3.keramos.tgz --sign --key author@example.com --keyring ~/.gnupg/secring.gpg
```

### Distributing the public key

Consumers add the signer's public key to their keramos keyring:

```sh
keramos keyring add /path/to/author@example.com.pub
keramos keyring list
keramos keyring remove <fingerprint>
```

The keramos keyring lives at `~/.config/keramos/keyring/` (or `${KERAMOS_CONFIG_HOME}/keyring/`). It's a directory of armoured PGP public keys, one file per signer.

### Verifying

Keramos's `--verify` flag (on `keramos pull` and `keramos install`) fetches the `.prov` file alongside the archive and validates it against the local keyring. A failed verification aborts the operation with a clear error naming the missing key fingerprint or the digest mismatch.

```sh
# pull from an HTTP repo, verify before saving
keramos pull my-app --repo https://charts.example.com --version 1.2.3 --prov --verify -d ./pulled

# install from a pulled, verified directory
keramos install my-app ./pulled/my-app -n prod

# OR install directly from OCI with .prov verification:
keramos install my-app oci://ghcr.io/example/charts/my-app:1.2.3 -n prod --verify
```

## Cosign (OCI artifacts)

Cosign signatures live in the OCI registry as a sibling artifact (a separate manifest tagged `sha256-<digest>.sig`). Keramos does not bundle cosign signing or verification; treat cosign as a separate gate that runs before `keramos install`.

### Producing

```sh
keramos registry push ./build/my-app-1.2.3.keramos.tgz oci://ghcr.io/example/charts/my-app
cosign sign --key cosign.key ghcr.io/example/charts/my-app:1.2.3
```

For keyless signing with OIDC (Sigstore Fulcio):

```sh
cosign sign ghcr.io/example/charts/my-app:1.2.3
```

### Verifying

Run cosign before invoking keramos:

```sh
cosign verify --key cosign.pub ghcr.io/example/charts/my-app:1.2.3
keramos install my-app oci://ghcr.io/example/charts/my-app:1.2.3 -n prod
```

Or, with keyless OIDC verification:

```sh
cosign verify ghcr.io/example/charts/my-app:1.2.3 \
  --certificate-identity user@example.com \
  --certificate-oidc-issuer https://accounts.google.com
keramos install my-app oci://ghcr.io/example/charts/my-app:1.2.3 -n prod
```

In CI, gate the install on cosign's exit code: a non-zero exit aborts the pipeline before keramos even runs.

### Mixing PGP and cosign

A package can be signed with PGP `.prov`, with cosign, both, or neither. Keramos's `--verify` flag covers PGP; cosign is external. Use both for layered assurance:

```sh
cosign verify --key cosign.pub ghcr.io/example/charts/my-app:1.2.3
keramos install my-app oci://ghcr.io/example/charts/my-app:1.2.3 -n prod --verify
```

## Audit trail

Every successful install/upgrade records its verification metadata in the release record:

```sh
keramos audit my-app
```

```
revision 3 — installed by user@example.com at 2026-05-08T14:32:01Z
  package digest:   sha256:9c1b8e3f4a...
  pgp signature:    valid (key 0xABCD1234, signed-by Jane Doe <jane@example.com>)
  cosign signature: valid (cert subject: CN=jane@example.com, Issuer: ...)
  flags:            install --verify --verify-cosign
```

The audit data is part of the release Secret and persists for the life of the release.

## Patterns

### CI builds with key-per-environment

```sh
# in CI
keramos package ./my-app -d ./build --sign --key ci-${ENV}@example.com --keyring ${KEYRING}

# operator side, per-env keyring
keramos keyring add /etc/keramos/dev-pubkeys.asc       # only knows dev signer
keramos install my-app my-charts/my-app --verify   # rejects archives signed by staging or prod CIs
```

### Verifying a layer

`layers:` and `requires:` entries also support `--verify` semantics during dependency resolution. Set `verify: true` on the layer entry:

```yaml
layers:
  - name: shared-base
    source: oci://ghcr.io/example/shared-base
    version: ^1.0.0
    verify: cosign
    cosignKey: /etc/keramos/shared-base.pub
```

`verify` accepts `pgp`, `cosign`, or `both`. `keramos dependency update` and `keramos dependency build` enforce the policy.

### Build provenance attestations

For SLSA-style provenance, see `keramos sbom` (CycloneDX 1.5 SBOM emission) — the SBOM and the cosign signature together produce a verifiable supply-chain trace.

## Common errors

- **`provenance verification failed: unknown signer (key 0xABCD)`** — add the signer's public key to your keramos keyring: `keramos keyring add /path/to/key.pub`.
- **`provenance verification failed: archive digest sha256:def... does not match signed sha256:abc...`** — the archive on disk doesn't match the one that was signed. Common cause: re-packaging without re-signing; re-fetch the original archive.
- **`cosign verify failed: no matching signatures`** — no cosign signature is attached to that artifact. Verify with `cosign tree <ref>` to see what's there.
- **`provenance file (.prov) not found`** — repo doesn't ship a `.prov`; either remove `--verify` or use a repo that signs.
