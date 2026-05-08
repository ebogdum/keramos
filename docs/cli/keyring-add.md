# keramos keyring add

## Synopsis

`keramos keyring add` installs an armoured PGP public key into keramos's local keyring directory (`~/.config/keramos/keyring/`, or `${KERAMOS_CONFIG_HOME}/keyring/`). Once installed, the key's signer is trusted for all subsequent `--verify` operations on `keramos pull`, `keramos install`, and dependency resolution.

## When to use it

Use to trust a new package signer — typically a CI-bot key or a maintainer's published public key. Existing keys are not overwritten unless you pass `--force`. The keyring is per-user and per-machine; there is no shared cluster-wide keyring.

## What happens when you run it

1. Reads the public key file at `<key-file>` (must be ASCII-armoured).
2. Validates that it is a recognisable PGP public key block.
3. Copies the file into `~/.config/keramos/keyring/<basename>.asc` (basename collisions error unless `--force`).
4. Prints the imported key's fingerprint and user-id on success.

## Usage

```
keramos keyring add <key-file> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--force` | bool | false | overwrite an existing key file with the same basename |
| `-h, --help` | bool | false | help for add |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Trust a new signer's key:

```sh
keramos keyring add /path/to/jane@example.com.pub
```

Replace an existing key (e.g. after a key rotation):

```sh
keramos keyring add /path/to/jane@example.com.pub --force
```

Install a key fetched from a keyserver:

```sh
gpg --keyserver hkps://keys.openpgp.org --recv-keys 0xABCD1234
gpg --export --armor 0xABCD1234 > /tmp/jane.pub
keramos keyring add /tmp/jane.pub
```

## See also

- [`keyring`](keyring.md)
- [`keyring list`](keyring-list.md)
- [`keyring remove`](keyring-remove.md)
- [Signing guide](../guides/signing.md)
