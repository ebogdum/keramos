# keramos plugin remove

## Synopsis

`keramos plugin remove` (aliases `rm`, `uninstall`) deletes an installed plugin.
Its directory under `~/.config/keramos/plugins/` is removed, and `keramos <name>` no
longer runs it.

## When to use it

Use it to retire a plugin you no longer need, clean up after developing one, or
clear a broken install before reinstalling.

## What happens

1. keramos finds the plugin's directory under `~/.config/keramos/plugins/`, matching
   `<name>` against either the directory name or the `name` in its
   `plugin.yaml`.
2. keramos runs the plugin's delete hook if it declares one.
3. keramos removes the directory.
4. keramos prints a confirmation line.

## Usage

```
keramos plugin remove <name> [flags]
```

## Flags

Inherits the global flags.

## Worked example

Remove a plugin by name:

```sh
keramos plugin remove hello
```

```
Removed plugin: hello
```

You can use an alias:

```sh
keramos plugin rm hello
```

## See also

- [`plugin install`](plugin-install.md)
- [`plugin list`](plugin-list.md)
- [`plugin update`](plugin-update.md)
