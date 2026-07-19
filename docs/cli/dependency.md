# keramos dependency

`keramos dependency` manages the **layers** and **required packages** your
package composes with. You declare them in `keramos.yaml`; these subcommands
inspect, resolve, download, and pin them.

- **Layers** (`layers:`) are packages merged *into* yours at render time — a
  shared base, common labels, org defaults.
- **Requires** (`requires:`) are separate packages installed *alongside* yours
  — a database, a cache, a sidecar release.

Each entry names a `source`: a local path, a `git::` URL, an `https://`
registry URL, or an `oci://` reference. Resolved versions and git commits are
pinned in a `keramos.lock` file, so every machine builds the same thing.

## Subcommands

| Command | What it does |
|---|---|
| [`list`](dependency-list.md) | Show every layer and required package with its type and lock status |
| [`tree`](dependency-tree.md) | Print the composition chain, including nested layers |
| [`update`](dependency-update.md) | Re-resolve versions and rewrite `keramos.lock` |
| [`build`](dependency-build.md) | Resolve and download everything declared |

`dep` is an alias for `dependency`.

## Usage

```
keramos dependency <command> <package-path> [flags]
```

Every subcommand takes the package directory as its argument (there is no
default). Run `list` or `tree` to see what is declared, `update` to pin
versions, and `build` to fetch them.

## See also

- [`install`](install.md) — install the package once its dependencies resolve
- [`template`](template.md) — render the package with its layers merged in
- [`repo`](repo.md) — manage the registries that registry sources pull from
