# keramos marketplace

## Synopsis

`keramos marketplace` browses and installs signed plugins from a keramos marketplace index. Subcommands search the index and verify a plugin's signature against the marketplace's trusted keys.

## When to use it

Use to discover community-published plugins and install them with provenance verification.

## Usage

```
keramos marketplace [command]
```

## Subcommands

- [`keramos marketplace verify`](marketplace-verify.md) — Verify a downloaded plugin archive against a marketplace index

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for marketplace |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Search the marketplace:

```sh
keramos marketplace search backup
```

Verify a plugin before installing:

```sh
keramos marketplace verify backup-plugin-1.0.tgz
```

## See also

- [`plugin`](plugin.md)
