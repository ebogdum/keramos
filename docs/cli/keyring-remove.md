# keramos keyring remove

## Synopsis

`keramos keyring remove` (alias `keramos keyring rm`) deletes a key from keramos's keyring directory. After removal, packages signed by that key fail verification with `--verify`. The argument is either a key fingerprint (use `keyring list` to find it) or the filename of the key as it sits in `~/.config/keramos/keyring/`.

## When to use it

Use to revoke trust in a signer — typically after a key compromise, an offboarding, or a key rotation where you've already added the new key. The action is local to this machine; it does not invalidate the key globally or notify any keyserver.

## What happens when you run it

1. Resolves the argument to a file under `~/.config/keramos/keyring/` — either matching a fingerprint within an installed key, or the filename directly.
2. Deletes the file.
3. Prints the removed key's fingerprint and user-id.
4. No cluster contact, no network.

## Usage

```
keramos keyring remove <fingerprint-or-filename> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for remove |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Remove by fingerprint (long-form):

```sh
keramos keyring remove 0xABCDEF1234567890ABCDEF1234567890ABCDEF12
```

Remove by short fingerprint suffix:

```sh
keramos keyring remove ABCDEF12
```

Remove by filename:

```sh
keramos keyring remove jane@example.com.asc
```

Use the short alias:

```sh
keramos keyring rm jane@example.com.asc
```

## See also

- [`keyring`](keyring.md)
- [`keyring add`](keyring-add.md)
- [`keyring list`](keyring-list.md)
- [Signing guide](../guides/signing.md)
