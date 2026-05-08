# keramos plugin

## Synopsis

`keramos plugin` manages the local plugin set. Plugins are external commands keramos invokes when an unknown top-level command is given (`keramos foo` becomes `keramos-plugin-foo args...` if `foo` is an installed plugin). Subcommands install, list, remove, and update plugins.

## When to use it

Use to extend keramos with site-specific or organisation-wide commands without modifying keramos itself.

## Usage

```
keramos plugin [command]
```

## Subcommands

- [`keramos plugin list`](plugin-list.md) — List installed plugins
- [`keramos plugin remove`](plugin-remove.md) — Remove an installed plugin
- [`keramos plugin update`](plugin-update.md) — Update an installed plugin

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for plugin |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Install a plugin from a local archive:

```sh
keramos plugin install ./my-plugin-1.0.tgz
```

List installed plugins:

```sh
keramos plugin list
```

Update every installed plugin:

```sh
keramos plugin update
```

## See also

- [`marketplace`](marketplace.md)
