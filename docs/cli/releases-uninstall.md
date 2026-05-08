# keramos releases uninstall

## Synopsis

`keramos releases uninstall` tears down every release declared in `keramos-releases.yaml` in **reverse** topological order — releases at the highest level go first, then progressively lower levels. This ensures that a release whose dependents have already been removed gets uninstalled cleanly without dangling references. Each release goes through the standard `keramos uninstall` flow (pre-delete hooks, resource removal, post-delete hooks, release-record removal).

## When to use it

Use when tearing down a managed platform cleanly — for example, in CI when re-creating a fresh test environment, or when retiring a customer instance. Reverse-order ensures dependents are gone before their dependencies, which avoids orphaned-resource errors.

## What happens when you run it

1. Reads `--file` (default `keramos-releases.yaml`) from the current directory.
2. Computes the topological order, then reverses it.
3. For each release, calls the same code path as `keramos uninstall` (pre-delete hook, resource delete, post-delete hook, release record removal).
4. Reports per-release outcome.
5. Exits 0 if every uninstall succeeded; non-zero if any failed.

## Usage

```
keramos releases uninstall [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--file` | string | keramos-releases.yaml | spec file path |
| `-h, --help` | bool | false | help for uninstall |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Tear down the platform in reverse order:

```sh
keramos releases uninstall
```

Tear down using a custom-named manifest:

```sh
keramos releases uninstall --file ./platform.releases.yaml
```

Plan first, then uninstall:

```sh
keramos releases plan
keramos releases uninstall
```

## See also

- [`releases`](releases.md)
- [`releases install`](releases-install.md)
- [`releases upgrade`](releases-upgrade.md)
- [`uninstall`](uninstall.md) — single-release uninstall
- [`keramos-releases.yaml` reference](../reference/keramos-releases-yaml.md)
