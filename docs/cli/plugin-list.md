---
title: "keramos plugin list"
parent: "CLI"
---
{% raw %}
# keramos plugin list

## Synopsis

`keramos plugin list` (alias `keramos plugin ls`) shows the plugins installed on this
machine — name, version, and description, one row per plugin.

## When to use it

Use it to see what you have installed, to find the exact name to pass to
[`update`](plugin-update.md) or [`remove`](plugin-remove.md), or to confirm an
[`install`](plugin-install.md) took.

## What happens

1. keramos reads `~/.config/keramos/plugins/` and looks at each subdirectory.
2. For each one, it reads `plugin.yaml` to get the name, version, and
   description.
3. It prints those as a table. Directories without a readable `plugin.yaml` are
   skipped.
4. If nothing is installed, keramos prints `No plugins installed.`

No cluster and no network are involved.

## Usage

```
keramos plugin list [flags]
```

## Flags

Inherits the global flags.

## Worked example

With two plugins installed:

```sh
keramos plugin list
```

```
NAME    VERSION  DESCRIPTION
backup  0.4.0    Snapshot and restore release state
hello   1.2.0    Print a friendly greeting
```

With nothing installed:

```sh
keramos plugin list
```

```
No plugins installed.
```

## See also

- [`plugin install`](plugin-install.md)
- [`plugin update`](plugin-update.md)
- [`plugin remove`](plugin-remove.md)
{% endraw %}
