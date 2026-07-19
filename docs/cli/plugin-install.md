# keramos plugin install

## Synopsis

`keramos plugin install` adds a plugin so its command becomes available as
`keramos <plugin-name>`. The source is either a git URL (cloned for you) or a path
to a local directory (copied in). keramos reads the plugin's `plugin.yaml`, checks
it, and stores it under `~/.config/keramos/plugins/`.

## When to use it

Use it when you want a keramos command the core binary doesn't ship — an
organisation helper, a custom auth flow, a workflow shortcut. For curated,
signed plugins, browse [`keramos marketplace search`](marketplace-search.md) and
[`verify`](marketplace-verify.md) the archive first.

## What happens

1. keramos decides whether `<source>` is a git source. A source ending in `.git`,
   or starting with `git@`, `git://`, `ssh://`, or `file://`, is treated as git.
2. For a git source, keramos runs `git clone --depth 1` into
   `~/.config/keramos/plugins/<name>`, where `<name>` is the last path segment of
   the URL without `.git`. For a local directory, keramos copies it to
   `~/.config/keramos/plugins/<name>`, where `<name>` is the directory's name.
3. keramos reads `plugin.yaml` for the plugin's name, version, description, and the
   `command` to run. The `command` must be a bare filename in the plugin
   directory (no slashes).
4. keramos runs the plugin's install hook if it declares one.
5. From now on, `keramos <name>` runs the plugin.

If a plugin with that name is already installed, keramos stops and leaves the
existing one untouched — remove it first to reinstall.

## Usage

```
keramos plugin install <source> [flags]
```

## Flags

Inherits the global flags.

## Worked example

Install a plugin from a local directory, then confirm it registered:

```sh
keramos plugin install ./hello
```

```
Installed plugin: hello v1.2.0
```

```sh
keramos plugin list
```

```
NAME   VERSION  DESCRIPTION
hello  1.2.0    Print a friendly greeting
```

```sh
keramos hello world
```

```
hello from the plugin: world
```

Install from a git URL instead:

```sh
keramos plugin install https://github.com/acme/keramos-backup.git
```

```
Installed plugin: backup v0.4.0
```

## See also

- [Plugins guide](../guides/plugins.md) — build a plugin from scratch
- [`plugin list`](plugin-list.md)
- [`plugin update`](plugin-update.md)
- [`plugin remove`](plugin-remove.md)
- [`marketplace search`](marketplace-search.md) — discover signed plugins
