# keramos repo index

Build an `index.yaml` for a directory of packaged archives.

## When to use it

- When you host your own HTTP repository and need the catalogue file that
  clients fetch to discover charts.
- After adding a newly packaged `.keramos.tgz` to the directory, to refresh the
  catalogue.

## What happens

Keramos scans `<dir>` for `.keramos.tgz` archives, reads each one's name and version,
computes its digest, and writes an `index.yaml` into that directory listing
every chart version. It prints the path it wrote. Serve the directory over HTTP
and clients that [`keramos repo add`](repo-add.md) its URL can then find those
charts.

Pass `--url` so each entry's download link is absolute. Use `--merge` to keep
the existing `index.yaml` entries and add only new ones instead of regenerating
from scratch. With `--sign`, keramos also writes an `index.yaml.prov` signature
next to the index and reports it.

## Usage

```
keramos repo index <dir> [flags]
```

## Flags

| Flag | Effect |
|---|---|
| `--url` | Prefix download URLs in the index with this base URL. |
| `--merge` | Merge into the existing `index.yaml` instead of regenerating it. |
| `--sign` | Sign the index with this private key, producing `index.yaml.prov`. |

## Worked example

```
$ keramos package ./my-app -d ./build
Packaged: ./build/my-app-1.4.0.keramos.tgz

$ keramos repo index ./build --url https://charts.example.com
Index generated at /home/you/build/index.yaml
```

Regenerate with a signature after adding a new version:

```
$ keramos repo index ./build --url https://charts.example.com --merge --sign ./repo-key.asc
Index generated at /home/you/build/index.yaml
Index signed: /home/you/build/index.yaml.prov
```

## See also

- [`package`](package.md) — build the `.keramos.tgz` archives to index
- [`publish`](publish.md) — push an archive to a registry
- [`repo add`](repo-add.md) — register the served repository on a client
