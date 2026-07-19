---
title: "keramos releases uninstall"
parent: "CLI"
---
{% raw %}
# keramos releases uninstall

## Synopsis

`keramos releases uninstall` removes every release in your `keramos-releases.yaml`, in
**reverse** dependency order — dependents first, their dependencies last. That
order means nothing is torn out from under a release that still depends on it.

## When to use it

- To tear down a whole set cleanly — for example when recreating a test
  environment or retiring an instance.
- Reverse order matters: removing dependents before dependencies avoids leaving
  a release running with its backing services already gone.

## What happens

1. Reads the spec file (`--file`, default `keramos-releases.yaml`), sorts the
   releases into dependency order, then reverses it.
2. For each release in that reversed order, uninstalls it from the entry's
   `namespace` (falling back to `-n/--namespace`). A release that is already
   gone is treated as success, not an error.
3. If one release fails to uninstall, the failure is reported and the command
   keeps going with the rest — one bad release does not strand the others.
4. Prints one line per release as it is removed.

## Usage

```
keramos releases uninstall [flags]
```

## Flags

| Flag | Type | Default | Effect |
|---|---|---|---|
| `--file` | string | `keramos-releases.yaml` | read the spec from this path instead of the default |

Inherits the global flags (`--kube-context`, `--kubeconfig`, `-n/--namespace`,
`--debug`). `-n` sets the namespace for any release that does not name its own.

## Worked example

**INPUT** — `keramos-releases.yaml`, the same set you installed:

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
keramos releases uninstall
```

**OUTPUT:**

```
[api] uninstalled
[redis] uninstalled
[postgres] uninstalled
```

`api` goes first because both datastores depend on it being gone before they
are removed; `postgres` and `redis` follow. This is the install order from
[`plan`](releases-plan.md), reversed.

## See also

- [`releases`](releases.md) — the parent command and the spec-file format
- [`plan`](releases-plan.md) — see the install order this reverses
- [`install`](releases-install.md) — bring the set back up
- [`uninstall`](uninstall.md) — uninstall a single release
{% endraw %}
