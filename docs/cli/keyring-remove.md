# keramos keyring remove

## Synopsis

`keramos keyring remove` (alias `keramos keyring rm`) deletes a key from
`~/.config/keramos/keyring/`, revoking trust in its signer. After removal,
packages signed by that key fail `--verify`. You identify the key by its full
fingerprint or by its file name.

## When to use it

- To revoke a signer after a key compromise, an offboarding, or a rotation
  where you have already added the replacement key.

The removal is local to this machine. It does not invalidate the key globally
or notify any keyserver.

## What happens

1. Reads `~/.config/keramos/keyring/`.
2. Matches your argument against each installed key — its file name, or its
   full fingerprint (both case-insensitive). Partial fingerprints do not match.
3. Deletes the matching file and prints `Removed key <filename>` (matched by
   name) or `Removed key <filename> (<fingerprint>)` (matched by fingerprint).
4. If nothing matches, errors with `no key matching "<arg>" in keyring`.

## Usage

```
keramos keyring remove <fingerprint-or-filename> [flags]
```

## Flags

Inherits the global flags.

## Worked example

**INPUT — a keyring holding one key** (from an earlier `keyring add`):

```sh
keramos keyring list
```

```
FINGERPRINT                                  FILE
3AA5C34371567BD2                             jane.pub
```

**Remove it by file name:**

```sh
keramos keyring remove jane.pub
```

**OUTPUT — keramos confirms the deletion:**

```
Removed key jane.pub
```

The full fingerprint works too, and reports it back:

```sh
keramos keyring remove 3AA5C34371567BD2
```

```
Removed key jane.pub (3AA5C34371567BD2)
```

Either way the key is gone — `keramos keyring list` now prints `No keys
installed.`, and packages signed by it no longer pass `--verify`.

## See also

- [`keyring`](keyring.md) — the parent command
- [`keyring add`](keyring-add.md) — install a key
- [`keyring list`](keyring-list.md) — find the fingerprint or file name
- [`package verify`](package-verify.md) — verify a package against the keyring
