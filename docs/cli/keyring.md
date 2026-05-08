# keramos keyring

## Synopsis

`keramos keyring` manages the PGP keyring used to verify package provenance signatures. Subcommands add, list, and remove armoured public keys.

## When to use it

Use to maintain the set of trusted signers for `--verify` operations.

## Usage

```
keramos keyring [command]
```

## Subcommands

- [`keramos keyring list`](keyring-list.md) — List keys in the keramos keyring
- [`keramos keyring remove`](keyring-remove.md) — Remove a key from the keramos keyring

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for keyring |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Add a signer's public key:

```sh
keramos keyring add /path/to/signer.pub
```

List trusted signers:

```sh
keramos keyring list
```

Remove a signer by fingerprint:

```sh
keramos keyring remove ABCDEF1234567890
```

## See also

- [Signing guide](../guides/signing.md)
- [`pull`](pull.md)
