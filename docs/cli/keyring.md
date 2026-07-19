---
title: "keramos keyring"
parent: "CLI"
---
{% raw %}
# keramos keyring

## Synopsis

`keramos keyring` manages the set of trusted PGP public keys keramos uses to verify
package provenance. When you run a signed operation with `--verify` (for
example `keramos package verify` or `keramos install --verify`), keramos checks the
package's `.prov` signature against the keys in this keyring.

The keyring is a per-user, per-machine directory, `~/.config/keramos/keyring/`.
There is no shared cluster-wide keyring.

## Subcommands

| Command | What it does |
|---|---|
| [`add`](keyring-add.md) | Install a public key so its signer becomes trusted. |
| [`list`](keyring-list.md) | Show the installed keys and their fingerprints. |
| [`remove`](keyring-remove.md) | Delete a key, revoking trust in its signer. |

## Usage

```
keramos keyring [command]
```

Add a signer, confirm it landed, and later remove it:

```sh
keramos keyring add ./jane.pub
keramos keyring list
keramos keyring remove jane.pub
```

## See also

- [`keyring add`](keyring-add.md) — trust a signer
- [`keyring list`](keyring-list.md) — show trusted signers
- [`keyring remove`](keyring-remove.md) — revoke a signer
- [`package verify`](package-verify.md) — verify a package against the keyring
- [`login`](login.md) — store registry credentials
{% endraw %}
