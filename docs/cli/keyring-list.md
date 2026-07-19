---
title: "keramos keyring list"
parent: "CLI"
---
{% raw %}
# keramos keyring list

## Synopsis

`keramos keyring list` (alias `keramos keyring ls`) prints every public key installed
in `~/.config/keramos/keyring/`, showing each key's fingerprint and file name.
These are the signers keramos trusts for `--verify` operations.

## When to use it

- To audit which signers are trusted on this machine.
- To find the fingerprint or filename to pass to `keramos keyring remove`.
- To confirm a `keramos keyring add` succeeded.

## What happens

1. Reads `~/.config/keramos/keyring/`, creating it empty if it does not exist.
2. If the directory holds no keys, prints `No keys installed.` and stops.
3. Otherwise prints a `FINGERPRINT` / `FILE` table, one row per key. A key file
   keramos cannot parse shows its fingerprint as `(unreadable)`.
4. Reads only local files — no cluster, no network.

## Usage

```
keramos keyring list [flags]
```

## Flags

Inherits the global flags.

## Worked example

**INPUT — a keyring with one key already added:**

```sh
keramos keyring add ./jane.pub
```

**List the trusted keys:**

```sh
keramos keyring list
```

**OUTPUT — the fingerprint and file name of each key:**

```
FINGERPRINT                                  FILE
3AA5C34371567BD2                             jane.pub
```

On a fresh machine with nothing added yet, the same command prints:

```
No keys installed.
```

## See also

- [`keyring`](keyring.md) — the parent command
- [`keyring add`](keyring-add.md) — install a key
- [`keyring remove`](keyring-remove.md) — remove a key
- [`package verify`](package-verify.md) — verify a package against the keyring
{% endraw %}
