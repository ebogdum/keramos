# keramos releases install

## Synopsis

`keramos releases install` installs every release in your `keramos-releases.yaml`, in
dependency order — dependencies first, dependents after. One command brings up
a whole set of related releases in the right sequence.

## When to use it

- To stand up a fresh set of related releases in one shot — for example a
  datastore tier plus the services that depend on it.
- When the ordering matters: keramos applies each release only after everything it
  lists in `dependsOn` is already installed.

## What happens

1. Reads the spec file (`--file`, default `keramos-releases.yaml`) and sorts the
   releases into dependency order. A cycle or unknown `dependsOn` name stops
   here, before anything is applied.
2. For each release in order, installs its package under the release `name`,
   in the entry's `namespace` (falling back to `-n/--namespace`), applying its
   `profile`, `values`, and `set`.
3. Each release is installed atomically: if one release fails to install, that
   release rolls itself back, the command stops, and the releases already
   installed before it are left in place.
4. Prints one confirmation line per release as it completes.

`install` does not wait for pods to become ready before moving to the next
release. If a dependent must not start until its dependency is actually
serving, use [`keramos workspace`](workspace.md) with `--health-gate`.

## Usage

```
keramos releases install [flags]
```

## Flags

| Flag | Type | Default | Effect |
|---|---|---|---|
| `--file` | string | `keramos-releases.yaml` | read the spec from this path instead of the default |

Inherits the global flags (`--kube-context`, `--kubeconfig`, `-n/--namespace`,
`--debug`). `-n` sets the namespace for any release that does not name its own.

## Worked example

**INPUT** — `keramos-releases.yaml`:

```yaml
releases:
  - name: postgres
    package: ./charts/postgres
    namespace: data

  - name: redis
    package: ./charts/redis
    namespace: data

  - name: api
    package: ./charts/api
    namespace: apps
    dependsOn:
      - postgres
      - redis
```

**COMMAND:**

```sh
keramos releases install
```

**OUTPUT:**

```
[postgres] installed (revision 1, ns data)
[redis] installed (revision 1, ns data)
[api] installed (revision 1, ns apps)
```

`postgres` and `redis` install first (they depend on nothing), each in the
`data` namespace; `api` installs last, in `apps`, because it lists both as
dependencies. Every release is new, so each is at revision 1.

## See also

- [`releases`](releases.md) — the parent command and the spec-file format
- [`plan`](releases-plan.md) — preview the order first
- [`upgrade`](releases-upgrade.md) — re-run to pick up changes later
- [`uninstall`](releases-uninstall.md) — tear the set back down
