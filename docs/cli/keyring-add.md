---
title: "keramos keyring add"
parent: "CLI"
---
{% raw %}
# keramos keyring add

## Synopsis

`keramos keyring add` installs a PGP public key into keramos's keyring so that its
signer becomes trusted for `--verify` operations. You point it at a key file;
keramos validates the file is a real public key and copies it into
`~/.config/keramos/keyring/`.

## When to use it

- To trust a new package signer — a maintainer's published key or a CI-bot
  key — before you verify or install their signed packages.
- After a key rotation, to install the replacement key (use `--force` to
  overwrite the old file).

## What happens

1. Reads the key file at `<key-file>`. If it cannot be read, the command
   errors.
2. Validates that the file is a recognisable PGP public key (ASCII-armoured or
   binary). Anything else is rejected, so junk never enters the trust store.
3. Copies it into `~/.config/keramos/keyring/` under its original filename. A
   filename already in the keyring is an error unless you pass `--force`.
4. Prints `Installed key <filename> (<fingerprint>)`.

## Usage

```
keramos keyring add <key-file> [flags]
```

## Flags

| Flag | Cause → effect |
|---|---|
| `--force` | Overwrite a key already installed under the same filename; without it, a name collision is an error. |

Also inherits the global flags.

## Worked example

**INPUT — a signer's public key** exported to `jane.pub`:

```sh
gpg --export --armor jane@example.com > jane.pub
```

**Add it to the keyring:**

```sh
keramos keyring add ./jane.pub
```

**OUTPUT — keramos confirms the install and prints the fingerprint:**

```
Installed key jane.pub (3AA5C34371567BD2)
```

**Confirm it is trusted** — the key now appears in the listing:

```sh
keramos keyring list
```

```
FINGERPRINT                                  FILE
3AA5C34371567BD2                             jane.pub
```

keramos will now accept packages signed by this key when you pass `--verify`.

## See also

- [`keyring`](keyring.md) — the parent command
- [`keyring list`](keyring-list.md) — confirm the key installed
- [`keyring remove`](keyring-remove.md) — revoke the key later
- [`package verify`](package-verify.md) — verify a package against the keyring
{% endraw %}
