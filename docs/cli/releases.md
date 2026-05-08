# keramos releases

## Synopsis

`keramos releases` orchestrates multiple separate releases declared in `keramos-releases.yaml`. Subcommands install, upgrade, uninstall, plan, and report status across the whole graph using a topological order.

## When to use it

Use when you have a fleet of releases sourced from different places (local paths, OCI, HTTPS, git) with explicit dependency ordering between them. For releases that live in one repository, prefer `keramos workspace`; for one-shot orchestration of disparate releases, this is the right tool.

## Usage

```
keramos releases [command]
```

## Subcommands

- [`keramos releases install`](releases-install.md) — install every release in topological order
- [`keramos releases upgrade`](releases-upgrade.md) — upgrade every release; install if missing
- [`keramos releases uninstall`](releases-uninstall.md) — uninstall every release in reverse topological order
- [`keramos releases plan`](releases-plan.md) — print the topological order without applying
- [`keramos releases status`](releases-status.md) — show current revision and status of every declared release

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for releases |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Plan an install of every release in `keramos-releases.yaml`:

```sh
keramos releases plan
```

Install every release in topological order:

```sh
keramos releases install
```

Use a custom-named manifest:

```sh
keramos releases install --file ./platform.releases.yaml
```

## See also

- [`keramos-releases.yaml` reference](../reference/keramos-releases-yaml.md)
- [Cross-release dependencies guide](../guides/releases.md)
- [`workspace`](workspace.md)
