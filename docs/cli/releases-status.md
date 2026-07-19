# keramos releases status

## Synopsis

`keramos releases status` reports where each release in your `keramos-releases.yaml`
currently stands — its latest revision and status — one line per release. It is
the quick "is the whole set up, and at which revision?" check.

## When to use it

- After an `install` or `upgrade`, to confirm every release landed.
- As a routine health check on the set, or a CI gate: a release that was never
  installed shows plainly as not deployed.

## What happens

1. Reads the spec file (`--file`, default `keramos-releases.yaml`) for the list of
   release names.
2. Looks up each release's latest recorded revision in the cluster (using
   `-n/--namespace` and the global cluster flags).
3. Prints one line per release, in the order they appear in the file:
   `name: revision N status=...` for a recorded release, or `name: not
   deployed` for one with no record.

Releases are reported in file order, not dependency order. A reachable cluster
is required, because the release records live there.

## Usage

```
keramos releases status [flags]
```

## Flags

| Flag | Type | Default | Effect |
|---|---|---|---|
| `--file` | string | `keramos-releases.yaml` | read the spec from this path instead of the default |

Inherits the global flags (`--kube-context`, `--kubeconfig`, `-n/--namespace`,
`--debug`).

## Worked example

**INPUT** — `keramos-releases.yaml`. `postgres` and `redis` are installed; `api`
was declared but never installed:

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
keramos releases status
```

**OUTPUT:**

```
postgres: revision 1 status=deployed
redis: revision 1 status=deployed
api: not deployed
```

Each installed release shows its latest revision and status; `api` has no
record in the cluster, so it reports `not deployed` — a signal to run
[`keramos releases install`](releases-install.md) or
[`upgrade`](releases-upgrade.md).

## See also

- [`releases`](releases.md) — the parent command and the spec-file format
- [`install`](releases-install.md) — install any that are not deployed
- [`upgrade`](releases-upgrade.md) — bring them all to current
- [`list`](list.md) — list every release in the cluster
