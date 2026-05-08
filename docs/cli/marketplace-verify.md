# keramos marketplace verify

## Synopsis

`keramos marketplace verify` checks that a downloaded plugin archive matches the entry recorded in the marketplace index — same SHA-256 digest, valid signature against the index's published trusted keys. Verification is recommended before any `keramos plugin install`, especially for plugins downloaded out-of-band (e.g. from a release page rather than via `keramos plugin install` directly, which performs verification automatically).

## When to use it

Use to confirm a plugin archive on disk has not been tampered with and was signed by the marketplace's trusted keys. A non-zero exit means the archive is suspect — do not install it.

## What happens when you run it

1. Fetches the marketplace index at `--index`.
2. Locates the entry for `--name`.
3. Hashes the archive at `--archive` and compares against the index's recorded digest.
4. Validates the index's signature against the marketplace's trusted keys.
5. Exits 0 on success, non-zero with a precise reason on failure (digest mismatch, missing signature, untrusted key).

## Usage

```
keramos marketplace verify [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--archive` | string | "" | path to the plugin archive to verify |
| `-h, --help` | bool | false | help for verify |
| `--index` | string | https://plugins.keramos.dev/index.json | marketplace index URL |
| `--name` | string | "" | plugin name (must match an index entry) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Verify a downloaded archive against the default marketplace:

```sh
keramos marketplace verify --archive ./backup-plugin-1.0.tgz --name backup
```

Verify against a self-hosted marketplace index:

```sh
keramos marketplace verify \
  --archive ./internal-plugin-2.1.tgz \
  --name internal-plugin \
  --index https://plugins.example.internal/index.json
```

Use as a CI gate — exit 0 means safe to proceed:

```sh
keramos marketplace verify --archive ./plugin.tgz --name backup && \
  keramos plugin install ./plugin.tgz
```

## See also

- [`marketplace`](marketplace.md)
- [`marketplace search`](marketplace-search.md)
- [`plugin install`](plugin-install.md)
- [Signing guide](../guides/signing.md)
