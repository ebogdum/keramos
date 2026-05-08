# keramos plugin install

## Synopsis

`keramos plugin install` adds a plugin to the local keramos installation. The source can be a git URL (cloned and built in place) or a local directory path. Plugins extend keramos with extra subcommands; once installed, they appear under `keramos <plugin-name>` and `keramos --help` lists them alongside built-in commands.

## When to use it

Use when you need a keramos subcommand that the core binary does not provide — typically organisation-specific helpers like backup, custom auth flows, or workflow shortcuts. For curated, signed plugins, prefer the `keramos marketplace` flow which pre-verifies provenance before install.

## What happens when you run it

1. Resolves the source: clones a git URL into the plugin directory, or symlinks/copies a local path.
2. Reads the plugin's `plugin.yaml` to discover its name, command path, and any post-install hook.
3. Materialises the plugin under `~/.config/keramos/plugins/<name>/`.
4. Runs the plugin's install hook if declared.
5. Subsequent invocations of `keramos <name>` resolve to the plugin.

## Usage

```
keramos plugin install <source> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for install |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Install from a git URL:

```sh
keramos plugin install https://github.com/example/keramos-backup-plugin.git
```

Install from a local directory (great while developing the plugin):

```sh
keramos plugin install ./my-plugin
```

Install a marketplace plugin after verifying it:

```sh
keramos marketplace verify --archive ./backup-plugin.tgz --name backup
keramos plugin install ./backup-plugin
```

## See also

- [`plugin`](plugin.md)
- [`plugin list`](plugin-list.md)
- [`plugin update`](plugin-update.md)
- [`plugin remove`](plugin-remove.md)
- [`marketplace`](marketplace.md) — discover and verify signed plugins
