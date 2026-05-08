# keramos package

## Synopsis

`keramos package` creates a `.keramos.tgz` archive from a package directory. The archive is self-contained: every layer is materialised, `keramos.lock` is included, and (with `--sign`) a detached PGP `.prov` file is produced alongside.

## When to use it

Use as the final step before publication. The output archive can be uploaded to an HTTP repository (`keramos publish --repo`), pushed to OCI (`keramos registry push` / `keramos publish --oci`), or distributed by hand.

## What happens when you run it

1. Reads `<path>` and resolves every layer (uses cached materials from `keramos dependency build` if present).
2. Validates `keramos.yaml` and `values.yaml`.
3. Composes a tarball containing the package directory, layer cache, and `keramos.lock`.
4. Names the output `<destination>/<name>-<version>.keramos.tgz`.
5. With `--sign --key <path>`, also emits a `.prov` file (detached PGP signature) next to the archive.
6. With `--reproducible`, writes the archive deterministically (zero timestamps, canonical modes) for reproducible builds.

## Usage

```
keramos package <path> [flags]
keramos package [command]
```

## Subcommands

- [`keramos package sign`](package-sign.md) — sign an existing archive with a PGP private key
- [`keramos package verify`](package-verify.md) — verify a `.prov` signature against the local keyring

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--app-version` | string | — | override the appVersion in keramos.yaml |
| `-d, --destination` | string | "." | directory to write the archive to |
| `-h, --help` | — | — | help for package |
| `--key` | string | — | PGP private key file or signer name (used with --sign) |
| `--keyring` | string | — | PGP keyring file containing the signer (alternative to --key) |
| `--passphrase-file` | string | — | file containing the key's passphrase (- for stdin) |
| `--reproducible` | — | — | produce byte-identical output across machines (zero timestamps, canonical modes) |
| `--sign` | — | — | produce a .prov provenance file alongside the archive (requires --key or --keyring) |
| `--version` | string | — | override the version in keramos.yaml |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Package a directory:

```sh
keramos package ./my-app -d ./build
```

Package and sign in one shot:

```sh
keramos package ./my-app -d ./build --sign --key /path/to/secret-key.asc
```

Reproducible build (deterministic output across machines):

```sh
keramos package ./my-app -d ./build --reproducible
```

Override the version captured in the archive (e.g. for a CI release-candidate tag):

```sh
keramos package ./my-app -d ./build --version 1.2.3-rc.4 --app-version 1.5.0
```

Verify a previously-signed archive:

```sh
keramos package verify ./build/my-app-1.0.0.keramos.tgz
```

## See also

- [`publish`](publish.md)
- [`registry push`](registry-push.md)
- [Repositories guide](../guides/repositories.md)
- [Signing guide](../guides/signing.md)
