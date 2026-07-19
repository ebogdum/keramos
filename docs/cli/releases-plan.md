---
title: "keramos releases plan"
parent: "CLI"
---
{% raw %}
# keramos releases plan

## Synopsis

`keramos releases plan` reads your `keramos-releases.yaml` and prints the order in
which the releases would be applied — a numbered list, dependencies before
dependents. It touches nothing: no cluster, no writes. It is the dry preview
you run before `install` or `upgrade`.

## When to use it

- Before an install or upgrade, to confirm the order is what you expect —
  especially after editing `dependsOn`.
- As a CI sanity check: `plan` exits non-zero if the file has a dependency
  cycle or names an unknown release, so a broken spec fails fast.

## What happens

1. Reads the spec file (`--file`, default `keramos-releases.yaml`).
2. Builds the dependency graph from each entry's `dependsOn`.
3. Sorts it into apply order (dependencies first; independent releases by
   name). A cycle or an unknown `dependsOn` name stops here with an error.
4. Prints the order as a numbered list — `N. name (package) ns=namespace` —
   and exits. No cluster is contacted.

## Usage

```
keramos releases plan [flags]
```

## Flags

| Flag | Type | Default | Effect |
|---|---|---|---|
| `--file` | string | `keramos-releases.yaml` | read the spec from this path instead of the default |

Inherits the global flags. `plan` reads only the file, so the cluster flags
(`--kube-context`, `--kubeconfig`, `-n`) have no effect on it.

## Worked example

**INPUT** — `keramos-releases.yaml`. `api` depends on both `postgres` and
`redis`; the two datastores depend on nothing:

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
keramos releases plan
```

**OUTPUT:**

```
1. postgres (./charts/postgres) ns=data
2. redis (./charts/redis) ns=data
3. api (./charts/api) ns=apps
```

Reading it back to the input: `postgres` and `redis` have no dependency on each
other, so they come first, ordered by name; `api` lists both in `dependsOn`, so
it is placed after both. That is exactly the order `keramos releases install` and
`keramos releases upgrade` follow.

## See also

- [`releases`](releases.md) — the parent command and the spec-file format
- [`install`](releases-install.md) — apply the set in this order
- [`upgrade`](releases-upgrade.md) — upgrade the set in this order
{% endraw %}
