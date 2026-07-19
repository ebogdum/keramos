# keramos dependency build

`keramos dependency build` resolves every layer and required package declared in
`keramos.yaml` and downloads each one so the package is ready to render.

## When to use it

- After [`update`](dependency-update.md), to fetch the pinned dependencies onto
  this machine.
- Before rendering or installing offline, so all remote sources are already in
  keramos's local cache.

## What happens

keramos reads `keramos.yaml` and resolves each layer and required package. If a
current `keramos.lock` exists, it fetches the exact pinned versions and commits;
otherwise it resolves fresh and writes the lock. Remote sources (`git::`,
registry) are downloaded into keramos's local cache; local paths need no
download. Once this succeeds, [`template`](template.md) and [`install`](install.md)
can render the package without reaching the network again. On success keramos
prints `Dependencies resolved successfully.`

## Usage

```
keramos dependency build <package-path>
```

## Flags

| Flag | Cause → effect |
|---|---|
| `--no-cache` | clear the cached repository index before resolving, so versions are re-read from the source instead of the last-fetched index |
| `--verify` | after downloading, check each installed dependency's digest against `keramos.lock`; fail if any does not match |

## Worked example

**INPUT — `./web/keramos.yaml`** with two layers and one required package, already
pinned in `keramos.lock` by a prior `dependency update`:

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
keramos dependency build ./web
```

**OUTPUT:**

```
Dependencies resolved successfully.
```

**What that line means, traced to the input:**

| `keramos.yaml` entry | What build did |
|---|---|
| `layers[0]` `base-layer` | resolved `../base-layer` at its locked version, ready to merge |
| `layers[1]` `common` | resolved `../common-layer` at its locked version |
| `requires[0]` `redis` | resolved `../redis-req`, ready to install alongside `web` |

With `--verify`, keramos additionally compares each downloaded dependency's digest
to the one recorded in `keramos.lock` and stops with an error if they differ —
proof the fetched bytes match what was pinned.

## See also

- [`dependency update`](dependency-update.md) — pin versions into `keramos.lock` first
- [`dependency tree`](dependency-tree.md) — see what will be downloaded
- [`install`](install.md) — install the package once dependencies are built
