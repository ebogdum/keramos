# keramos dependency list

`keramos dependency list` shows every layer and required package your package
declares, with its source type and whether it is pinned in `keramos.lock`.

## When to use it

- To confirm what a package composes with before you install or render it.
- To check whether your dependencies are pinned (`locked`) or still floating
  (`unlocked`) after editing `keramos.yaml`.

## What happens

keramos reads `keramos.yaml` and prints a `LAYERS:` table and a `REQUIRES:` table.
For each entry you see:

- **NAME** — the layer or package name.
- **TYPE** — `local`, `git`, or `registry`, derived from the `source`.
- **SOURCE** — the declared source (truncated if long).
- **STATUS** — `locked` if `keramos.lock` pins this entry, otherwise `unlocked`.

Nothing is downloaded or resolved; this is a read-only view of what you
declared and what the lock file currently records. If a package declares no
layers or requires, keramos prints `No layers or requires declared.`

## Usage

```
keramos dependency list <package-path>
```

## Flags

Inherits the global flags.

## Worked example

**INPUT — `./web/keramos.yaml`** declaring two layers and one required package,
with no `keramos.lock` yet:

```yaml
apiVersion: keramos/v1
name: web
version: 0.3.0
layers:
  - name: base-layer
    source: ../base-layer
  - name: common
    source: ../common-layer
requires:
  - name: redis
    source: ../redis-req
```

**Command:**

```sh
keramos dependency list ./web
```

**OUTPUT:**

```
LAYERS:
  NAME                 TYPE         SOURCE                                   STATUS
  base-layer           local        ../base-layer                            unlocked
  common               local        ../common-layer                          unlocked

REQUIRES:
  NAME                 TYPE         SOURCE                                   STATUS
  redis                local        ../redis-req                             unlocked
```

**Tracing each input to its output line:**

| `keramos.yaml` entry | Output row | Why |
|---|---|---|
| `layers[0]` `base-layer`, `../base-layer` | `base-layer  local … unlocked` | a `layers:` entry → LAYERS table; a path → `local` type; no lock yet → `unlocked` |
| `layers[1]` `common`, `../common-layer` | `common  local … unlocked` | second layer, same reasoning |
| `requires[0]` `redis`, `../redis-req` | `redis  local … unlocked` | a `requires:` entry → REQUIRES table |

After you run [`update`](dependency-update.md), a `keramos.lock` is written and the
same command reports every `STATUS` as `locked`.

## See also

- [`dependency update`](dependency-update.md) — pin these entries into `keramos.lock`
- [`dependency tree`](dependency-tree.md) — see nested layers, not just the top level
- [`dependency build`](dependency-build.md) — download the resolved dependencies
