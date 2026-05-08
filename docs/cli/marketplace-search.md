# keramos marketplace search

## Synopsis

`keramos marketplace search` lists plugins published to the keramos plugin marketplace. With no keyword, it prints every plugin in the index; with a keyword, it filters by substring match against name, description, and tags. The default index is `https://plugins.keramos.dev/index.json`; use `--index` to point at a self-hosted or alternate marketplace.

## When to use it

Use to discover plugins before installing them. The marketplace is a curated, signed index — the entries here have been verified by the marketplace operator. For ad-hoc plugins, install directly with `keramos plugin install` against a URL or path.

## What happens when you run it

1. Fetches the JSON index at `--index` over HTTPS.
2. Filters entries by the optional keyword.
3. Prints a table of name, version, description, and signing metadata to stdout.
4. No cluster contact.

## Usage

```
keramos marketplace search [keyword] [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for search |
| `--index` | string | https://plugins.keramos.dev/index.json | marketplace index URL |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Browse the entire default marketplace:

```sh
keramos marketplace search
```

Find backup-related plugins:

```sh
keramos marketplace search backup
```

Use a self-hosted marketplace index:

```sh
keramos marketplace search --index https://plugins.example.internal/index.json
```

## See also

- [`marketplace`](marketplace.md)
- [`marketplace verify`](marketplace-verify.md)
- [`plugin`](plugin.md) — install plugins from arbitrary sources
- [`plugin install`](plugin-install.md)
