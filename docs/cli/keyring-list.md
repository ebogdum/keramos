# keramos keyring list

## Synopsis

`keramos keyring list` (alias `keramos keyring ls`) prints every public key currently installed in the keramos keyring directory: file name, fingerprint, user-id, and creation date. The keyring is the trust store consulted by `--verify` operations on pull and install.

## When to use it

Use to audit which signers are trusted on this machine, to find a fingerprint for a `keramos keyring remove` call, or to confirm a `keramos keyring add` succeeded.

## What happens when you run it

1. Reads `~/.config/keramos/keyring/` (or `${KERAMOS_CONFIG_HOME}/keyring/`).
2. Parses each `.asc` file as a PGP public key block.
3. Prints one row per key.
4. Read-only; no cluster contact, no network.

## Usage

```
keramos keyring list [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for list |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

List trusted keys:

```sh
keramos keyring list
```

Use the short alias:

```sh
keramos keyring ls
```

Find a specific signer:

```sh
keramos keyring list | grep jane@example.com
```

## See also

- [`keyring`](keyring.md)
- [`keyring add`](keyring-add.md)
- [`keyring remove`](keyring-remove.md)
- [Signing guide](../guides/signing.md)
