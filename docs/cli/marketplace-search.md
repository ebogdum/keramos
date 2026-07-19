---
title: "keramos marketplace search"
parent: "CLI"
---
{% raw %}
# keramos marketplace search

## Synopsis

`keramos marketplace search` lists the plugins in a marketplace index. With no
keyword it prints every plugin; with a keyword it prints only those whose name or
description contains that text.

## When to use it

Use it to discover plugins before installing them, and to read off a plugin's
name and who signed it so you can verify and install it.

## What happens

1. keramos fetches the JSON index from `--index` (default
   `https://plugins.keramos.dev/index.json`) over HTTP(S).
2. If you gave a keyword, keramos keeps only entries whose name or description
   contains it (case-sensitive substring match).
3. keramos prints one line per matching plugin: name, version, the signer, and the
   description.

No cluster is contacted, and nothing is downloaded or installed.

## Usage

```
keramos marketplace search [keyword] [flags]
```

## Flags

| Flag | Cause → effect |
|---|---|
| `--index <url>` | Fetch the plugin list from this index instead of the default `https://plugins.keramos.dev/index.json`. |

Also inherits the global flags.

## Worked example

Search the default marketplace for backup-related plugins:

```sh
keramos marketplace search backup
```

```
backup-restore                 1.4.0      signedBy=keramos-core — Snapshot and restore release state
s3-backup                      0.9.2      signedBy=acme-ops — Push release snapshots to an S3 bucket
```

List everything in a private marketplace:

```sh
keramos marketplace search --index https://plugins.example.internal/index.json
```

## See also

- [`marketplace verify`](marketplace-verify.md) — check an archive before installing
- [`plugin install`](plugin-install.md)
{% endraw %}
