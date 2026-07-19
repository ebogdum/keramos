# Distribute packages via an OCI registry

Keramos packages push to and pull from any OCI distribution-spec registry — GHCR,
Docker Hub, Quay, Harbor, Artifactory, ECR, GAR, ACR, self-hosted Zot or
Distribution. Each package version becomes an OCI artifact: keramos stores the
`*.keramos.tgz` archive as the artifact blob under a thin manifest.

Choose OCI when you already run a container registry, want cloud IAM
integration, or want immutable, content-addressable artifacts. For static HTTP
hosting instead, see [Repositories](repositories.md).

## Authenticate

Registry credentials are stored by `keramos login`, keyed by host, in
`~/.config/keramos/credentials.json`. (There is no `keramos registry login` — the
`keramos registry` command has only `push` and `pull`.)

```sh
echo "$GITHUB_TOKEN" | keramos login ghcr.io -u myuser --password-stdin
keramos login registry.example.com -u u -p p
```

```
Login succeeded for ghcr.io
```

`keramos login` is non-interactive and needs one credential: `-u/--username` with
`-p/--password` (or `--password-stdin`), or `--token`, or `--api-key`.
Subsequent push and pull authenticate automatically. Remove a credential with
`keramos logout ghcr.io`.

## Push a package

Build the archive, then push it. Two commands can push:

```sh
keramos package ./my-app -d ./build

# Dedicated OCI push:
keramos registry push ./build/my-app-1.2.3.keramos.tgz oci://ghcr.io/example/charts/my-app:1.2.3
```

```
Pushed ./build/my-app-1.2.3.keramos.tgz to oci://ghcr.io/example/charts/my-app:1.2.3
```

```sh
# Or the multi-target publish command:
keramos publish ./build/my-app-1.2.3.keramos.tgz --oci oci://ghcr.io/example/charts/my-app
```

```
Published my-app@1.2.3 to OCI registry oci://ghcr.io/example/charts/my-app
```

An untagged reference is tagged with the package's own version from `keramos.yaml`.

## Pull a package

There are two pull paths. `keramos registry pull` is OCI-only, takes a full
reference **including the tag**, and can verify a cosign signature first. It
writes the archive to `-d/--destination`; it does not resolve version
constraints or untar.

```sh
keramos registry pull oci://ghcr.io/example/charts/my-app:1.2.3 -d ./pulled
```

```
Pulled oci://ghcr.io/example/charts/my-app:1.2.3 to ./pulled/my-app-1.2.3.keramos.tgz
```

`keramos pull` accepts an `oci://` reference too, resolves a SemVer `--version`
against the registry's tags, and can unpack with `--untar`. Its destination
flag is `--destination` (no `-d` shorthand):

```sh
keramos pull oci://ghcr.io/example/charts/my-app --version "^1.2.0" \
  --destination ./pulled --untar
```

```
Pulled and extracted: ./pulled/my-app
```

## Install from OCI

`keramos install` takes a local package **directory** — it does not fetch remote
references. Pull and unpack first, then install from the directory:

```sh
keramos pull oci://ghcr.io/example/charts/my-app --version 1.2.3 \
  --destination ./pulled --untar
keramos install my-app ./pulled/my-app -n prod --create-namespace
```

```
release my-app installed (revision 1)
```

## Verify a cosign signature on pull

`keramos registry pull` verifies a cosign signature on the artifact **before** it
touches disk — fail-closed, so an unsigned or wrongly-signed artifact is never
pulled. Supply a public key, or a keyless identity + issuer:

```sh
# Key-based:
keramos registry pull oci://ghcr.io/example/charts/my-app:1.2.3 \
  --cosign-key cosign.pub -d ./pulled

# Keyless (Sigstore):
keramos registry pull oci://ghcr.io/example/charts/my-app:1.2.3 \
  --cosign-identity 'https://github.com/example/ci/.github/workflows/release.yml@refs/heads/main' \
  --cosign-issuer   'https://token.actions.githubusercontent.com' \
  -d ./pulled
```

```
cosign signature verified for oci://ghcr.io/example/charts/my-app:1.2.3
Pulled oci://ghcr.io/example/charts/my-app:1.2.3 to ./pulled/my-app-1.2.3.keramos.tgz
```

Cosign signing itself is external — sign with the cosign CLI after pushing:

```sh
keramos registry push ./build/my-app-1.2.3.keramos.tgz oci://ghcr.io/example/charts/my-app:1.2.3
cosign sign --key cosign.key ghcr.io/example/charts/my-app:1.2.3
```

For PGP `.prov` provenance instead of cosign, see [Signing](signing.md).

## Consume a layer from OCI

A package can source a composition layer from an OCI registry. In `keramos.yaml`:

```yaml
layers:
  - name: my-layer
    source: oci://ghcr.io/example/charts/my-layer
    version: ^2.0.0
```

`keramos dependency update ./my-app` resolves the constraint against the registry
and locks the result in `keramos.lock`:

```yaml
# keramos.lock
layers:
  - name: my-layer
    source: oci://ghcr.io/example/charts/my-layer
    resolvedVersion: 2.4.1
    digest: sha256:7e2a90d8c5...
```

Later `keramos dependency build ./my-app` fetches the locked version. See
[`keramos dependency`](../cli/dependency.md).

## Use an insecure or plaintext registry

For a local registry without TLS, add `--plain-http` to `keramos registry push`
and `keramos registry pull`; for a self-signed certificate, use
`--insecure-skip-tls-verify`:

```sh
keramos registry push ./build/my-app-1.2.3.keramos.tgz \
  oci://localhost:5000/charts/my-app:1.2.3 --plain-http
keramos registry pull oci://localhost:5000/charts/my-app:1.2.3 --plain-http
```

The same behaviour can be set globally — as flags on any command
(`--oci-plain-http`, `--oci-insecure-skip-tls-verify`) or as environment
variables, which is convenient in CI:

| Env var | Effect |
|---|---|
| `KERAMOS_OCI_PLAIN_HTTP=1` | use HTTP instead of HTTPS for OCI |
| `KERAMOS_OCI_INSECURE_SKIP_TLS=1` | skip TLS verification for OCI |

Sending basic-auth credentials over plaintext HTTP additionally requires
`--allow-plaintext-auth` (or `KERAMOS_ALLOW_PLAINTEXT_AUTH=1`).

## Troubleshooting

- **`unauthorized: authentication required`** — log in: `keramos login <host>`.
- **`x509: certificate signed by unknown authority`** — the registry uses an
  internal CA; for a quick local test use `--insecure-skip-tls-verify` (never
  in production).
- **`no matching version for constraint`** — the constraint matched no tag;
  list the registry's tags to see what is published.

## See also

- [`keramos registry push`](../cli/registry-push.md) ·
  [`keramos registry pull`](../cli/registry-pull.md)
- [`keramos pull`](../cli/pull.md) · [`keramos publish`](../cli/publish.md)
- [`keramos login`](../cli/login.md) — store registry credentials
- [Repositories](repositories.md) — static HTTP distribution
- [Signing](signing.md) — PGP and cosign verification
