---
title: "keramos marketplace verify"
parent: "CLI"
---
{% raw %}
# keramos marketplace verify

## Synopsis

`keramos marketplace verify` checks a plugin archive you downloaded against a
marketplace index: it confirms the archive's SHA-256 digest matches the one in
the index and that the index's signature for that plugin was made by a key you
trust. It succeeds only when both checks pass.

## When to use it

Run it before installing a plugin you downloaded out of band, so you know the
archive hasn't been altered and comes from a signer you pinned. Verify passing is
your cue to run [`keramos plugin install`](plugin-install.md).

## What happens

1. keramos fetches the marketplace index from `--index` (default
   `https://plugins.keramos.dev/index.json`).
2. keramos reads the archive at `--archive` and computes its SHA-256, then compares
   it to the digest the index records for `--name`. A mismatch fails.
3. keramos loads your pinned trusted keys from
   `~/.config/keramos/marketplace_trusted_keys.json` (override with the
   `KERAMOS_TRUSTED_KEYS` environment variable). The index's own key list is never
   used, so a hostile index can't supply its own root of trust.
4. keramos checks the index's Ed25519 signature for `--name` against the pinned key
   that matches the plugin's signer. A missing signature, an unknown signer, or a
   bad signature fails.
5. On success keramos prints `<name>: signature OK` and exits 0. On any failure it
   prints the reason and exits non-zero.

## Usage

```
keramos marketplace verify [flags]
```

## Flags

| Flag | Cause → effect |
|---|---|
| `--archive <path>` | Verify this archive file. Required. |
| `--name <name>` | Match against this plugin's entry in the index. Required; must match an index entry. |
| `--index <url>` | Fetch the index from this URL instead of the default `https://plugins.keramos.dev/index.json`. |

Also inherits the global flags.

## Worked example

Verify a downloaded archive, then install it once it passes:

```sh
keramos marketplace verify --archive ./backup-restore.tar.gz --name backup-restore
```

```
backup-restore: signature OK
```

```sh
keramos plugin install ./backup-restore
```

Verify against a private marketplace:

```sh
keramos marketplace verify \
  --archive ./internal-tool.tar.gz \
  --name internal-tool \
  --index https://plugins.example.internal/index.json
```

## See also

- [`marketplace search`](marketplace-search.md) — find a plugin and its signer
- [`plugin install`](plugin-install.md)
{% endraw %}
