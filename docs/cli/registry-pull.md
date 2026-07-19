# keramos registry pull

Download a keramos package from an OCI reference, optionally verifying its cosign
signature first.

## When to use it

- To fetch a package that was pushed with [`keramos registry push`](registry-push.md).
- To require a valid cosign signature before the artifact ever touches disk —
  supply `--cosign-key` or the keyless `--cosign-identity` + `--cosign-issuer`.
- When you need the OCI-only transport flags (`--plain-http`,
  `--insecure-skip-tls-verify`) that the general [`keramos pull`](pull.md) does not
  expose.

## What happens

1. If any cosign flag is set, keramos verifies the signature on `<ref>` first.
   Verification is fail-closed: an unsigned or wrongly-signed artifact is not
   pulled, and `cosign signature verified for <ref>` prints only on success.
2. Uses the credentials you stored with [`keramos login`](login.md) for the host
   in `<ref>`.
3. Downloads the artifact and writes it as a `.keramos.tgz` archive into
   `--destination` (default the current directory).
4. Prints `Pulled <ref> to <path>` naming the file that landed on disk.

## Usage

```
keramos registry pull <ref>
```

`<ref>` is an `oci://…` reference including the `:tag` that selects the
version.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-d, --destination` | string | `.` | Directory the downloaded `.keramos.tgz` is written to. |
| `--cosign-key` | string | "" | Verify the artifact's cosign signature with this public key before pulling (key-based). |
| `--cosign-identity` | string | "" | Keyless cosign: require this certificate identity. Use with `--cosign-issuer`. |
| `--cosign-issuer` | string | "" | Keyless cosign: require this certificate OIDC issuer. Use with `--cosign-identity`. |
| `--plain-http` | bool | false | Talk to the registry over plaintext HTTP instead of HTTPS. |
| `--insecure-skip-tls-verify` | bool | false | Keep HTTPS but skip certificate validation. |

Global flags `--oci-plain-http`, `--oci-insecure-skip-tls-verify`, and
`--allow-plaintext-auth` are inherited from `keramos`.

## Worked example

You want the signed 1.0.0 package, and you refuse to accept it unless the
signature checks out.

**INPUT** — keyless verification, saving into `./pulled`:

```sh
keramos registry pull oci://ghcr.io/example/charts/my-app:1.0.0 \
  --cosign-identity 'https://github.com/example/ci/.github/workflows/release.yml@refs/heads/main' \
  --cosign-issuer   'https://token.actions.githubusercontent.com' \
  -d ./pulled
```

**OUTPUT:**

```
cosign signature verified for oci://ghcr.io/example/charts/my-app:1.0.0
Pulled oci://ghcr.io/example/charts/my-app:1.0.0 to ./pulled/my-app-1.0.0.keramos.tgz
```

**RESULT:** `./pulled/my-app-1.0.0.keramos.tgz` is on disk, and it is guaranteed
signed by the expected identity. Without the cosign flags the pull runs the
same way, minus the first line:

```sh
keramos registry pull oci://ghcr.io/example/charts/my-app:1.0.0 -d ./pulled
```
```
Pulled oci://ghcr.io/example/charts/my-app:1.0.0 to ./pulled/my-app-1.0.0.keramos.tgz
```

## See also

- [`login`](login.md) — store the credentials this command uses
- [`registry push`](registry-push.md) — upload a package
- [`pull`](pull.md) — general pull (OCI or HTTP repo) with version resolution
- [`install`](install.md) — install a package from a reference
